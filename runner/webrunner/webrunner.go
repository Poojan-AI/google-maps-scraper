package webrunner

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gosom/google-maps-scraper/deduper"
	"github.com/gosom/google-maps-scraper/exiter"
	"github.com/gosom/google-maps-scraper/runner"
	"github.com/gosom/google-maps-scraper/tlmt"
	"github.com/gosom/google-maps-scraper/web"
	"github.com/gosom/google-maps-scraper/web/sqlite"
	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/adapters/writers/csvwriter"
	"github.com/gosom/scrapemate/adapters/writers/jsonwriter"
	"github.com/gosom/scrapemate/scrapemateapp"
	"golang.org/x/sync/errgroup"
)

const (
	// MaxConcurrentJobs defines the maximum number of jobs that can be processed concurrently
	// 
	// Constraints considered:
	// 1. Proxy pool: 23 usable proxies (25 total - 2 excluded bad ones)
	// 2. Detection avoidance: Don't use all proxies simultaneously (looks suspicious)
	// 3. Memory: Each job with Playwright uses ~200-300MB RAM
	// 4. CPU: Each job spawns browser instances (CPU intensive)
	// 
	// Recommended values:
	// - Conservative (2GB RAM): 5 jobs  (~1.5GB peak usage)
	// - Moderate (4GB RAM):     10 jobs (~3GB peak usage)
	// - Aggressive (8GB+ RAM):  15 jobs (~4.5GB peak usage)
	// 
	// Current setting optimized for: Moderate workload with good detection avoidance
	MaxConcurrentJobs = 10
)

type webrunner struct {
	srv        *web.Server
	svc        *web.Service
	cfg        *runner.Config
	jobSemaphore chan struct{} // Semaphore to limit concurrent jobs
}

func New(cfg *runner.Config) (runner.Runner, error) {
	if cfg.DataFolder == "" {
		return nil, fmt.Errorf("data folder is required")
	}

	if err := os.MkdirAll(cfg.DataFolder, os.ModePerm); err != nil {
		return nil, err
	}

	const dbfname = "jobs.db"

	dbpath := filepath.Join(cfg.DataFolder, dbfname)

	repo, err := sqlite.New(dbpath)
	if err != nil {
		return nil, err
	}

	svc := web.NewService(repo, cfg.DataFolder)

	srv, err := web.New(svc, cfg.Addr)
	if err != nil {
		return nil, err
	}

	// Determine the number of concurrent jobs to allow
	// Use MaxConcurrentJobs by default, but allow cfg.Concurrency to override if explicitly set higher
	maxJobs := MaxConcurrentJobs
	if cfg.Concurrency > MaxConcurrentJobs {
		// If user explicitly sets a higher concurrency, respect it
		maxJobs = cfg.Concurrency
	}
	// Note: We always use MaxConcurrentJobs (5) for multi-job concurrency in web mode
	// The cfg.Concurrency controls per-job place scraping concurrency, not multi-job concurrency
	
	log.Printf("Webrunner initialized with max %d concurrent jobs (cfg.Concurrency=%d)", maxJobs, cfg.Concurrency)

	ans := webrunner{
		srv:          srv,
		svc:          svc,
		cfg:          cfg,
		jobSemaphore: make(chan struct{}, maxJobs),
	}

	return &ans, nil
}

func (w *webrunner) Run(ctx context.Context) error {
	egroup, ctx := errgroup.WithContext(ctx)

	egroup.Go(func() error {
		return w.work(ctx)
	})

	egroup.Go(func() error {
		return w.srv.Start(ctx)
	})

	return egroup.Wait()
}

func (w *webrunner) Close(context.Context) error {
	return nil
}

func (w *webrunner) work(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// WaitGroup to track running jobs
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			// Wait for all running jobs to complete before exiting
			wg.Wait()
			return nil
		case <-ticker.C:
			jobs, err := w.svc.SelectPending(ctx)
			if err != nil {
				return err
			}

			// Launch jobs concurrently, respecting the semaphore limit
			for i := range jobs {
				select {
				case <-ctx.Done():
					// Wait for all running jobs to complete before exiting
					wg.Wait()
					return nil
				default:
					// Acquire semaphore slot (blocks if at capacity)
					w.jobSemaphore <- struct{}{}
					
					wg.Add(1)
					
					// Launch goroutine to process job
					go func(job *web.Job) {
						defer wg.Done()
						defer func() {
							// Release semaphore slot
							<-w.jobSemaphore
						}()

						t0 := time.Now().UTC()
						if err := w.scrapeJob(ctx, job); err != nil {
							params := map[string]any{
								"job_count": len(job.Data.Keywords),
								"duration":  time.Now().UTC().Sub(t0).String(),
								"error":     err.Error(),
							}

							evt := tlmt.NewEvent("web_runner", params)

							_ = runner.Telemetry().Send(ctx, evt)

							log.Printf("error scraping job %s: %v", job.ID, err)
						} else {
							params := map[string]any{
								"job_count": len(job.Data.Keywords),
								"duration":  time.Now().UTC().Sub(t0).String(),
							}

							_ = runner.Telemetry().Send(ctx, tlmt.NewEvent("web_runner", params))

							log.Printf("job %s scraped successfully", job.ID)
						}
					}(&jobs[i])
				}
			}
		}
	}
}

func (w *webrunner) scrapeJob(ctx context.Context, job *web.Job) error {
	job.Status = web.StatusWorking

	err := w.svc.Update(ctx, job)
	if err != nil {
		return err
	}

	if len(job.Data.Keywords) == 0 {
		job.Status = web.StatusFailed

		return w.svc.Update(ctx, job)
	}

	// Use JSON format when extra reviews are enabled to handle large datasets better
	var outpath string
	if job.Data.ExtraReviews {
		outpath = filepath.Join(w.cfg.DataFolder, job.ID+".json")
	} else {
		outpath = filepath.Join(w.cfg.DataFolder, job.ID+".csv")
	}

	outfile, err := os.Create(outpath)
	if err != nil {
		return err
	}

	defer func() {
		_ = outfile.Close()
	}()

	// Proxy rotation: try each proxy in sequence if job fails
	var lastErr error
	maxProxyRetries := 1 // Default: no proxy rotation
	
	if len(job.Data.Proxies) > 1 {
		maxProxyRetries = len(job.Data.Proxies)
		log.Printf("Job %s has %d proxies available for rotation on failure", job.ID, maxProxyRetries)
	}
	
	for proxyIdx := 0; proxyIdx < maxProxyRetries; proxyIdx++ {
		if proxyIdx > 0 {
			log.Printf("Job %s: Retrying with proxy #%d after previous failure", job.ID, proxyIdx+1)
			// Truncate and reopen the output file for retry
			outfile.Close()
			outfile, err = os.Create(outpath)
			if err != nil {
				return err
			}
		}
		
		mate, err := w.setupMateWithProxyIndex(ctx, outfile, job, proxyIdx)
		if err != nil {
			lastErr = err
			log.Printf("Job %s: Failed to setup scraper with proxy #%d: %v", job.ID, proxyIdx+1, err)
			if mate != nil {
				mate.Close()
			}
			continue
		}
		
		// Try to run the job with this proxy
		lastErr = w.executeJobWithMate(ctx, job, mate, proxyIdx, maxProxyRetries)
		mate.Close()
		
		if lastErr == nil {
			// Success! No need to try more proxies
			log.Printf("Job %s: Completed successfully with proxy #%d", job.ID, proxyIdx+1)
			job.Status = web.StatusOK
			return w.svc.Update(ctx, job)
		}
		
		log.Printf("Job %s: Failed with proxy #%d: %v", job.ID, proxyIdx+1, lastErr)
	}
	
	// All proxies failed
	job.Status = web.StatusFailed
	err2 := w.svc.Update(ctx, job)
	if err2 != nil {
		log.Printf("failed to update job status: %v", err2)
	}
	return fmt.Errorf("job failed with all %d proxies, last error: %w", maxProxyRetries, lastErr)
}

func (w *webrunner) executeJobWithMate(ctx context.Context, job *web.Job, mate *scrapemateapp.ScrapemateApp, proxyIdx int, totalProxies int) error {
	var coords string
	if job.Data.Lat != "" && job.Data.Lon != "" {
		coords = job.Data.Lat + "," + job.Data.Lon
	}

	dedup := deduper.New()
	exitMonitor := exiter.New()

	log.Printf("Starting scrape for job %s. Global ExtraReviews: %t, Job ExtraReviews: %t", job.ID, w.cfg.ExtraReviews, job.Data.ExtraReviews)

	seedJobs, err := runner.CreateSeedJobs(
		job.Data.FastMode,
		job.Data.Lang,
		strings.NewReader(strings.Join(job.Data.Keywords, "\n")),
		job.Data.Depth,
		job.Data.Email,
		coords,
		job.Data.Zoom,
		func() float64 {
			if job.Data.Radius <= 0 {
				return 10000 // 10 km
			}

			return float64(job.Data.Radius)
		}(),
		dedup,
		exitMonitor,
		job.Data.ExtraReviews,
	)
	if err != nil {
		return err
	}

	if len(seedJobs) > 0 {
		exitMonitor.SetSeedCount(len(seedJobs))

		allowedSeconds := max(60, len(seedJobs)*10*job.Data.Depth/50+120)

		if job.Data.MaxTime > 0 {
			if job.Data.MaxTime.Seconds() < 180 {
				allowedSeconds = 180
			} else {
				allowedSeconds = int(job.Data.MaxTime.Seconds())
			}
		}
		
		// For proxy rotation: use full timeout for each proxy attempt
		// Progress monitoring will detect failures quickly (30 seconds with no progress)
		// So we don't need to artificially reduce timeout
		if totalProxies > 1 {
			log.Printf("Proxy rotation enabled (attempt %d/%d): using full %d seconds timeout with progress monitoring", 
				proxyIdx+1, totalProxies, allowedSeconds)
		}

		log.Printf("running job %s with %d seed jobs and %d allowed seconds", job.ID, len(seedJobs), allowedSeconds)

		mateCtx, cancel := context.WithTimeout(ctx, time.Duration(allowedSeconds)*time.Second)
		defer cancel()

		exitMonitor.SetCancelFunc(cancel)

		go exitMonitor.Run(mateCtx)

		// Start scraping with early failure detection
		errChan := make(chan error, 1)
		go func() {
			errChan <- mate.Start(mateCtx, seedJobs...)
		}()
		
		// Monitor progress for early failure detection (especially for proxy issues)
		progressTicker := time.NewTicker(30 * time.Second) // Check every 30 seconds
		defer progressTicker.Stop()
		
		var lastPlacesCompleted int
		var lastPlacesFound int
		var lastSeedCompleted int
		progressCheckCount := 0
		
	monitorLoop:
		for {
			select {
			case err = <-errChan:
				// Scraping finished (success or error)
				break monitorLoop
				
			case <-progressTicker.C:
				// Check if we're making ANY progress (found, completed, or seed processing)
				currentPlacesCompleted := exitMonitor.GetPlacesCompleted()
				currentPlacesFound := exitMonitor.GetPlacesFound()
				currentSeedCompleted := exitMonitor.GetSeedCompleted()
				progressCheckCount++
				
				// Check if there's ANY activity at all
				hasAnyActivity := currentPlacesFound > 0 || currentPlacesCompleted > 0 || currentSeedCompleted > 0
				
				if !hasAnyActivity && progressCheckCount >= 1 {
					// No activity after 30 seconds (1 check) - likely proxy/connection issue
					cancel()
					log.Printf("Job %s: No activity after %d seconds (found=%d, completed=%d, seeds=%d), likely proxy failure", 
						job.ID, progressCheckCount*30, currentPlacesFound, currentPlacesCompleted, currentSeedCompleted)
					return fmt.Errorf("no scraping activity after %d seconds - proxy may be blocked or connection failed", progressCheckCount*30)
				}
				
				// Log progress if we see any changes
				if currentPlacesCompleted > lastPlacesCompleted || currentPlacesFound > lastPlacesFound || currentSeedCompleted > lastSeedCompleted {
					log.Printf("Job %s: Progress check - seeds: %d, found: %d, completed: %d", 
						job.ID, currentSeedCompleted, currentPlacesFound, currentPlacesCompleted)
					lastPlacesCompleted = currentPlacesCompleted
					lastPlacesFound = currentPlacesFound
					lastSeedCompleted = currentSeedCompleted
				}
				
			case <-mateCtx.Done():
				// Timeout reached
				break monitorLoop
			}
		}
		
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			cancel()
			return err
		}

		cancel()
		
		// Check if we actually got any results
		placesCompleted := exitMonitor.GetPlacesCompleted()
		log.Printf("Job %s completed with %d places scraped", job.ID, placesCompleted)
		
		if placesCompleted == 0 {
			return fmt.Errorf("no places were successfully scraped (0 results)")
		}
	}

	return nil
}

func (w *webrunner) setupMate(_ context.Context, writer io.Writer, job *web.Job) (*scrapemateapp.ScrapemateApp, error) {
	return w.setupMateWithProxyIndex(nil, writer, job, 0)
}

func (w *webrunner) setupMateWithProxyIndex(_ context.Context, writer io.Writer, job *web.Job, proxyIndex int) (*scrapemateapp.ScrapemateApp, error) {
	opts := []func(*scrapemateapp.Config) error{
		scrapemateapp.WithConcurrency(w.cfg.Concurrency),
		scrapemateapp.WithExitOnInactivity(time.Minute * 3),
	}

	if !job.Data.FastMode {
		opts = append(opts,
			scrapemateapp.WithJS(scrapemateapp.DisableImages()),
		)
	} else {
		opts = append(opts,
			scrapemateapp.WithStealth("firefox"),
		)
	}

	hasProxy := false

	if len(w.cfg.Proxies) > 0 {
		opts = append(opts, scrapemateapp.WithProxies(w.cfg.Proxies))
		hasProxy = true
		log.Printf("job %s using global proxy config: %v", job.ID, w.cfg.Proxies)
	} else if len(job.Data.Proxies) > 0 {
		// Use specific proxy from the list based on proxyIndex
		var proxyToUse []string
		if proxyIndex < len(job.Data.Proxies) {
			proxyToUse = []string{job.Data.Proxies[proxyIndex]}
		} else {
			// Fallback to first proxy if index is out of bounds
			proxyToUse = []string{job.Data.Proxies[0]}
		}
		
		opts = append(opts,
			scrapemateapp.WithProxies(proxyToUse),
		)
		hasProxy = true
		log.Printf("job %s using proxy #%d: %s", job.ID, proxyIndex+1, proxyToUse[0])
	}

	if !w.cfg.DisablePageReuse {
		opts = append(opts,
			scrapemateapp.WithPageReuseLimit(2),
			scrapemateapp.WithPageReuseLimit(200),
		)
	}

	if !hasProxy {
		log.Printf("job %s running WITHOUT proxy", job.ID)
	}

	var writers []scrapemate.ResultWriter
	if job.Data.ExtraReviews {
		// Use JSON format for jobs with extra reviews to handle large datasets
		jsonWriter := jsonwriter.NewJSONWriter(writer)
		writers = []scrapemate.ResultWriter{jsonWriter}
		log.Printf("job %s using JSON writer for extra reviews", job.ID)
	} else {
		csvWriter := csvwriter.NewCsvWriter(csv.NewWriter(writer))
		writers = []scrapemate.ResultWriter{csvWriter}
		log.Printf("job %s using CSV writer", job.ID)
	}

	matecfg, err := scrapemateapp.NewConfig(
		writers,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	return scrapemateapp.NewScrapeMateApp(matecfg)
}
