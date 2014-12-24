package lastfm

import (
	"encoding/gob"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pmylund/go-cache"
)

func init() {
	gob.Register(cache.Item{})

	gob.Register(LastFMError{})
	gob.Register(Neighbours{})
	gob.Register(RecentTracks{})
	gob.Register(Tasteometer{})
	gob.Register(TopArtists{})
	gob.Register(TopTags{})
	gob.Register(TrackInfo{})
}

const (
	DefaultDuration        = 5 * time.Minute
	DefaultCleanupInterval = 1 * time.Minute
)

func makeCacheKey(method string, query map[string]string) string {
	keys := make([]string, 0, len(query))
	for key, _ := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(query)+1)
	parts = append(parts, method)
	for _, key := range keys {
		parts = append(parts, strings.Join([]string{key, query[key]}, "="))
	}

	return strings.Join(parts, "&")
}

func (lfm *LastFM) cacheGet(method string, query map[string]string) (v interface{}, err error) {
	key := makeCacheKey(method, query)
	if data, ok := lfm.Cache.Get(key); !ok {
		return nil, nil
	} else {
		switch v := data.(type) {
		case error:
			return nil, v
		default:
			return v, nil
		}
	}
}

func (lfm *LastFM) cacheSet(method string, query map[string]string, v interface{}, hdr http.Header) {
	now := time.Now()

	end := now

	if hdr != nil {
		if _, ok := hdr["Cache-Control"]; ok {
			for _, control := range hdr["Cache-Control"] {
				ctrl := strings.Split(control, "=")
				if len(ctrl) > 0 {
					switch ctrl[0] {
					case "no-cache":
						return
					case "max-age":
						age, _ := strconv.ParseInt(ctrl[1], 10, 64)
						end = now.Add(time.Duration(age) * time.Second)
					}
				}
			}
		} else if expires, ok := hdr["Expires"]; ok && len(expires) > 0 {
			end, _ = time.Parse(time.RFC1123, expires[0])
		}
	}

	if dur := end.Sub(now); dur > 0 {
		key := makeCacheKey(method, query)
		lfm.Cache.Set(key, v, dur)
	}
}

// Writes a gob-encoded representation of the Cache to the
// given io.Writer.
func (lfm *LastFM) SaveCache(w io.Writer) error {
	lfm.Cache.RLock()
	defer lfm.Cache.RUnlock()

	enc := gob.NewEncoder(w)
	return enc.Encode(lfm.Cache.Items())
}

// Reads a gob-encoded representation of the Cache from the
// given io.Reader. Does not change the current Cache if
// there is a read or decoding error.
//
// The Cache will have its parameters set to this package's
// DefaultDuration and DefaultCleanupInterval. To change
// the cache's duration/interval, a new cache is needed
// like so:
//
//     lfm.Cache = cache.NewFrom(duration, interval, lfm.Cache.Items())
//
// This method is not safe for concurrent access, use before
// starting any requests.
func (lfm *LastFM) LoadCache(r io.Reader) error {
	dec := gob.NewDecoder(r)
	var items map[string]*cache.Item
	if err := dec.Decode(&items); err != nil {
		return err
	}
	lfm.Cache = cache.NewFrom(DefaultDuration, DefaultCleanupInterval, items)
	return nil
}
