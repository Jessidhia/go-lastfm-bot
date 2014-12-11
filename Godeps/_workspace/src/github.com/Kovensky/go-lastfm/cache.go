package lastfm

import (
	"encoding/gob"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func init() {
	// github.com/pmylund/go-cache's caching is gob-based
	gob.Register(LastFMError{})
	gob.Register(Neighbours{})
	gob.Register(RecentTracks{})
	gob.Register(Tasteometer{})
	gob.Register(TopArtists{})
	gob.Register(TopTags{})
	gob.Register(TrackInfo{})
}

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
