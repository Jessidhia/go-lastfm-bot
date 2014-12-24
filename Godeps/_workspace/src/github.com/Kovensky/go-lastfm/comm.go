package lastfm

import (
	"io"
	"net/http"
	"net/url"

	"github.com/pmylund/go-cache"
)

var (
	apiBaseURL = url.URL{Scheme: "http", Host: "ws.audioscrobbler.com", Path: "/2.0/"}
)

func buildQueryURL(query map[string]string) string {
	v := url.Values{}
	for key, value := range query {
		v.Add(key, value)
	}
	u := apiBaseURL
	u.RawQuery = v.Encode()
	return u.String()
}

type getter interface {
	Get(url string) (resp *http.Response, err error)
}

type mockServer interface {
	doQuery(params map[string]string) ([]byte, error)
}

// Struct used to access the API servers.
type LastFM struct {
	apiKey string
	getter getter
	Cache  *cache.Cache
}

// Create a new LastFM struct.
// The apiKey parameter must be an API key registered with Last.fm.
func New(apiKey string) LastFM {
	return LastFM{
		apiKey: apiKey,
		getter: http.DefaultClient,
		Cache:  cache.New(DefaultDuration, DefaultCleanupInterval),
	}
}

func (lfm *LastFM) doQuery(method string, params map[string]string) (body io.ReadCloser, hdr http.Header, err error) {
	queryParams := make(map[string]string, len(params)+2)
	queryParams["api_key"] = lfm.apiKey
	queryParams["method"] = method
	for key, value := range params {
		queryParams[key] = value
	}

	resp, err := lfm.getter.Get(buildQueryURL(queryParams))
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return
	}
	return resp.Body, resp.Header, err
}

// Used to unwrap XML from inside the <lfm> parent
type lfmStatus struct {
	Status       string       `xml:"status,attr"`
	RecentTracks RecentTracks `xml:"recenttracks"`
	Tasteometer  Tasteometer  `xml:"comparison"`
	TrackInfo    TrackInfo    `xml:"track"`
	TopTags      TopTags      `xml:"toptags"`
	Neighbours   Neighbours   `xml:"neighbours>user"`
	TopArtists   TopArtists   `xml:"topartists"`
	Error        LastFMError  `xml:"error"`
}

type lfmDate struct {
	Date string `xml:",chardata"`
	UTS  int64  `xml:"uts,attr"`
}
