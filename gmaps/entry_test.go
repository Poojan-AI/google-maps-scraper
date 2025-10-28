package gmaps_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"

	"github.com/gosom/google-maps-scraper/gmaps"
)

func createGoQueryFromFile(t *testing.T, path string) *goquery.Document {
	t.Helper()

	fd, err := os.Open(path)
	require.NoError(t, err)

	defer fd.Close()

	doc, err := goquery.NewDocumentFromReader(fd)
	require.NoError(t, err)

	return doc
}

func Test_EntryFromJSON(t *testing.T) {
	expected := gmaps.Entry{
		Link:       "https://www.google.com/maps/place/Kipriakon/data=!4m2!3m1!1s0x14e732fd76f0d90d:0xe5415928d6702b47!10m1!1e1",
		Title:      "Kipriakon",
		Category:   "Restaurant",
		Categories: []string{"Restaurant"},
		Address:    "Old port, Limassol 3042",
		OpenHours: map[string][]string{
			"Monday":    {"12:30–10 pm"},
			"Tuesday":   {"12:30–10 pm"},
			"Wednesday": {"12:30–10 pm"},
			"Thursday":  {"12:30–10 pm"},
			"Friday":    {"12:30–10 pm"},
			"Saturday":  {"12:30–10 pm"},
			"Sunday":    {"12:30–10 pm"},
		},
		WebSite:      "",
		Phone:        "25 101555",
		PlusCode:     "M2CR+6X Limassol",
		ReviewCount:  396,
		ReviewRating: 4.2,
		Latitude:     34.670595399999996,
		Longtitude:   33.042456699999995,
		Cid:          "16519582940102929223",
		Status:       "Closed ⋅ Opens 12:30\u202fpm Tue",
		ReviewsLink:  "https://search.google.com/local/reviews?placeid=ChIJDdnwdv0y5xQRRytw1ihZQeU&q=Kipriakon&authuser=0&hl=en&gl=CY",
		Thumbnail:    "https://lh5.googleusercontent.com/p/AF1QipP4Y7A8nYL3KKXznSl69pXSq9p2IXCYUjVvOh0F=w408-h408-k-no",
		Timezone:     "Asia/Nicosia",
		PriceRange:   "€€",
		DataID:       "0x14e732fd76f0d90d:0xe5415928d6702b47",
		Images: []gmaps.Image{
			{
				Title: "All",
				Image: "https://lh5.googleusercontent.com/p/AF1QipP4Y7A8nYL3KKXznSl69pXSq9p2IXCYUjVvOh0F=w298-h298-k-no",
			},
			{
				Title: "Latest",
				Image: "https://lh5.googleusercontent.com/p/AF1QipNgMqyaQs2MqH1oiGC44eDcvudurxQfNb2RuDsd=w224-h298-k-no",
			},
			{
				Title: "Videos",
				Image: "https://lh5.googleusercontent.com/p/AF1QipPZbq8v8K8RZfvL6gZ_4Dw6qwNJ_MUxxOOfBo7h=w224-h398-k-no",
			},
			{
				Title: "Menu",
				Image: "https://lh5.googleusercontent.com/p/AF1QipNhoFtPcaLCIhdN3GhlJ6sQIvdhaESnRG8nyeC8=w397-h298-k-no",
			},
			{
				Title: "Food & drink",
				Image: "https://lh5.googleusercontent.com/p/AF1QipMbu-iiWkE4DsXx3aI7nGaqyXJKbBYCrBXvzOnu=w298-h298-k-no",
			},
			{
				Title: "Vibe",
				Image: "https://lh5.googleusercontent.com/p/AF1QipOGg_vrD4bzkOre5Ly6CFXuO3YCOGfFxQ-EiEkW=w224-h398-k-no",
			},
			{
				Title: "Fried green tomatoes",
				Image: "https://lh5.googleusercontent.com/p/AF1QipOziHd2hqM1jnK9KfCGf1zVhcOrx8Bj7VdJXj0=w397-h298-k-no",
			},
			{
				Title: "French fries",
				Image: "https://lh5.googleusercontent.com/p/AF1QipNJyq7nAlKtsxxbNy4PHUZOhJ0k7HPP8tTAlwcV=w397-h298-k-no",
			},
			{
				Title: "By owner",
				Image: "https://lh5.googleusercontent.com/p/AF1QipNRE2R5k13zT-0WG4b6XOD_BES9-nMK04hlCMVV=w298-h298-k-no",
			},
			{
				Title: "Street View & 360°",
				Image: "https://lh5.googleusercontent.com/p/AF1QipMwkHP8GmDCSuwnWS7pYVQvtDWdsdk-CUwxtsXL=w224-h298-k-no-pi-23.425545-ya289.20517-ro-8.658787-fo100",
			},
		},
		OrderOnline: []gmaps.LinkSource{
			{
				Link:   "https://foody.com.cy/delivery/lemesos/to-kypriakon?utm_source=google&utm_medium=organic&utm_campaign=google_reserve_place_order_action",
				Source: "foody.com.cy",
			},
			{
				Link:   "https://wolt.com/en/cyp/limassol/restaurant/kypriakon?utm_source=googlemapreserved&utm_campaign=kypriakon",
				Source: "wolt.com",
			},
		},
		Owner: gmaps.Owner{
			ID:   "102769814432182832009",
			Name: "Kipriakon (Owner)",
			Link: "https://www.google.com/maps/contrib/102769814432182832009",
		},
		CompleteAddress: gmaps.Address{
			Borough:    "",
			Street:     "Old port",
			City:       "Limassol",
			PostalCode: "3042",
			State:      "",
			Country:    "CY",
		},
		ReviewsPerRating: map[int]int{
			1: 37,
			2: 16,
			3: 27,
			4: 60,
			5: 256,
		},
	}

	raw, err := os.ReadFile("../testdata/raw.json")
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	entry, err := gmaps.EntryFromJSON(raw)
	require.NoError(t, err)

	require.Len(t, entry.About, 10)

	for _, about := range entry.About {
		require.NotEmpty(t, about.ID)
		require.NotEmpty(t, about.Name)
		require.NotEmpty(t, about.Options)
	}

	entry.About = nil

	require.Len(t, entry.PopularTimes, 7)

	for k, v := range entry.PopularTimes {
		require.Contains(t,
			[]string{
				"Monday",
				"Tuesday",
				"Wednesday",
				"Thursday",
				"Friday",
				"Saturday",
				"Sunday",
			}, k)

		for _, traffic := range v {
			require.GreaterOrEqual(t, traffic, 0)
			require.LessOrEqual(t, traffic, 100)
		}
	}

	monday := entry.PopularTimes["Monday"]
	require.Equal(t, 100, monday[20])

	entry.PopularTimes = nil
	entry.UserReviews = nil

	require.Equal(t, expected, entry)
}

func Test_EntryFromJSON2(t *testing.T) {
	fnames := []string{
		"../testdata/panic.json",
		"../testdata/panic2.json",
	}
	for _, fname := range fnames {
		raw, err := os.ReadFile(fname)
		require.NoError(t, err)
		require.NotEmpty(t, raw)

		_, err = gmaps.EntryFromJSON(raw)
		require.NoError(t, err)
	}
}

func Test_EntryFromJSONRaw2(t *testing.T) {
	raw, err := os.ReadFile("../testdata/raw2.json")

	require.NoError(t, err)
	require.NotEmpty(t, raw)

	entry, err := gmaps.EntryFromJSON(raw)

	require.NoError(t, err)
	require.Greater(t, len(entry.About), 0)
}

func Test_EntryFromJsonC(t *testing.T) {
	raw, err := os.ReadFile("../testdata/output.json")

	require.NoError(t, err)
	require.NotEmpty(t, raw)

	entries, err := gmaps.ParseSearchResults(raw)

	require.NoError(t, err)

	for _, entry := range entries {
		fmt.Printf("%+v\n", entry)
	}
}

func Test_ConvertRelativeDateToAbsolute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantYear int
	}{
		{
			name:     "5 months ago",
			input:    "5 months ago",
			wantYear: -1, // Will be determined at runtime
		},
		{
			name:     "a month ago",
			input:    "a month ago",
			wantYear: -1,
		},
		{
			name:     "2 years ago",
			input:    "2 years ago",
			wantYear: -1,
		},
		{
			name:     "a year ago",
			input:    "a year ago",
			wantYear: -1,
		},
		{
			name:     "3 weeks ago",
			input:    "3 weeks ago",
			wantYear: -1,
		},
		{
			name:     "a week ago",
			input:    "a week ago",
			wantYear: -1,
		},
		{
			name:     "10 days ago",
			input:    "10 days ago",
			wantYear: -1,
		},
		{
			name:     "a day ago",
			input:    "a day ago",
			wantYear: -1,
		},
		{
			name:     "5 hours ago",
			input:    "5 hours ago",
			wantYear: -1,
		},
		{
			name:     "an hour ago",
			input:    "an hour ago",
			wantYear: -1,
		},
		{
			name:     "1 hour ago",
			input:    "1 hour ago",
			wantYear: -1,
		},
		{
			name:     "empty string",
			input:    "",
			wantYear: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gmaps.ConvertRelativeDateToAbsolute(tt.input)

			if tt.input == "" {
				require.Empty(t, result)
				return
			}

			// Verify result is not empty for valid input
			require.NotEmpty(t, result, "Expected non-empty result for input: %s", tt.input)

			// Verify result is in the format YYYY-M-D or YYYY-MM-DD
			require.Regexp(t, `^\d{4}-\d{1,2}-\d{1,2}$`, result)
		})
	}
}

func Test_ConvertRelativeDateToAbsolute_MonthsAgo(t *testing.T) {
	result := gmaps.ConvertRelativeDateToAbsolute("5 months ago")
	require.NotEmpty(t, result)

	// The result should be 5 months ago from now, keeping the same day
	// We can't test exact date, but we can verify format
	require.Regexp(t, `^\d{4}-\d{1,2}-\d{1,2}$`, result)
}

func Test_ConvertRelativeDateToAbsolute_YearsAgo(t *testing.T) {
	result := gmaps.ConvertRelativeDateToAbsolute("2 years ago")
	require.NotEmpty(t, result)

	// The result should be 2 years ago from now, keeping the same month and day
	// We can't test exact date, but we can verify format
	require.Regexp(t, `^\d{4}-\d{1,2}-\d{1,2}$`, result)
}

func Test_ConvertRelativeDateToAbsolute_HoursAgo(t *testing.T) {
	// Test with current day - hours ago should return today's date
	result := gmaps.ConvertRelativeDateToAbsolute("5 hours ago")
	require.NotEmpty(t, result)

	// Should return a valid date format
	require.Regexp(t, `^\d{4}-\d{1,2}-\d{1,2}$`, result)

	// Test "an hour ago"
	result = gmaps.ConvertRelativeDateToAbsolute("an hour ago")
	require.NotEmpty(t, result)
	require.Regexp(t, `^\d{4}-\d{1,2}-\d{1,2}$`, result)

	// Test "1 hour ago"
	result = gmaps.ConvertRelativeDateToAbsolute("1 hour ago")
	require.NotEmpty(t, result)
	require.Regexp(t, `^\d{4}-\d{1,2}-\d{1,2}$`, result)
}

func Test_FindRelativeDateString(t *testing.T) {
	// Test with simple array
	simpleArray := []any{
		"some string",
		"5 months ago",
		"another string",
	}
	result := gmaps.FindRelativeDateString(simpleArray)
	require.Equal(t, "5 months ago", result)

	// Test with nested array
	nestedArray := []any{
		"some string",
		123,
		[]any{
			"nested string",
			[]any{
				"deeply nested",
				"2 years ago",
			},
		},
		"after",
	}
	result = gmaps.FindRelativeDateString(nestedArray)
	require.Equal(t, "2 years ago", result)

	// Test with no relative date
	noDateArray := []any{
		"some string",
		"no date here",
		[]any{"nested", "values"},
	}
	result = gmaps.FindRelativeDateString(noDateArray)
	require.Empty(t, result)

	// Test with "AGO" in uppercase (case insensitive check)
	uppercaseArray := []any{
		"3 WEEKS AGO",
	}
	result = gmaps.FindRelativeDateString(uppercaseArray)
	require.Equal(t, "3 WEEKS AGO", result)
}
