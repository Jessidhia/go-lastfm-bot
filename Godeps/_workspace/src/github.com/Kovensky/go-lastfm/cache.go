package lastfm

import (
	"encoding/gob"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CacheData struct {
	Expires time.Time
	Value   interface{}
}

type CacheMap map[string]CacheData

type CacheStats struct {
	Hit   uint64
	Miss  uint64
	Stale uint64
}

type Cache struct {
	data  CacheMap
	mutex sync.Mutex
	Stats CacheStats
}

func init() {
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

func (c *Cache) Len() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return len(c.data)
}

func (c *Cache) HitRate(withStale bool) float64 {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	sum := float64(c.Stats.Miss) + float64(c.Stats.Hit)
	if withStale {
		sum += float64(c.Stats.Stale)
	}
	return float64(c.Stats.Hit) / sum
}

func (c *Cache) ResetStats() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Stats = CacheStats{}
}

func (c *Cache) Get(method string, query map[string]string) (v interface{}, err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.data == nil {
		c.Stats.Miss++
		return nil, nil
	}

	key := makeCacheKey(method, query)
	if data, ok := c.data[key]; !ok {
		c.Stats.Miss++
		return nil, nil
	} else if data.Expires.Before(time.Now()) {
		c.Stats.Stale++
		delete(c.data, key)
		return nil, nil
	} else {
		c.Stats.Hit++
		switch v := data.Value.(type) {
		case error:
			return nil, v
		default:
			return v, nil
		}
	}
}

func (c *Cache) Set(method string, query map[string]string, v interface{}, hdr http.Header) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	data := CacheData{Value: v}
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
						data.Expires = now.Add(time.Duration(age) * time.Second)
					}
				}
			}
		} else if expires, ok := hdr["Expires"]; ok && len(expires) > 0 {
			data.Expires, _ = time.Parse(time.RFC1123, expires[0])
		}
	}
	if data.Expires.After(now) {
		key := makeCacheKey(method, query)
		if c.data == nil {
			c.data = make(CacheMap)
		}
		c.data[key] = data
	}
	return
}

func (c *Cache) Purge() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data = make(CacheMap)
}

func (c *Cache) Clean() (n int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.data == nil {
		return
	}

	now := time.Now()
	old := []string{}
	for key, v := range c.data {
		if v.Expires.Before(now) {
			old = append(old, key)
		}
	}
	for _, key := range old {
		delete(c.data, key)
		n++
	}
	return
}

func (c *Cache) Load(r io.Reader) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return gob.NewDecoder(r).Decode(&c.data)
}

func (c *Cache) Store(w io.Writer) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return gob.NewEncoder(w).Encode(c.data)
}
