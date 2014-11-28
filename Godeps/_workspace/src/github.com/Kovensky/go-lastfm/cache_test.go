package lastfm_test

import (
	"github.com/Kovensky/go-lastfm"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCache_Simple(T *testing.T) {
	T.Parallel()
	c := lastfm.Cache{}

	utc := time.FixedZone("UTC", 0)
	empty := map[string]string{}

	c.Set("test", empty, "test", http.Header{})
	v, _ := c.Get("test", empty)
	Expect(T, "empty headers to cache", nil, v)

	c.Set("test", empty, "test",
		http.Header{"Expires": []string{time.Now().In(utc).Add(time.Hour).Format(time.RFC1123)}})
	v, _ = c.Get("test", empty)
	Expect(T, "Expires header to cache", "test", v)

	c.Set("test", empty, "test2",
		http.Header{"Cache-Control": []string{"max-age=60"}})
	v, _ = c.Get("test", empty)
	Expect(T, "Cache-Control header to cache", "test2", v)
}

func TestCache_Persist(T *testing.T) {
	T.Parallel()

	file := strings.Join([]string{os.TempDir(), "TestCache_Persist"}, string(os.PathSeparator))
	defer os.Remove(file)
	c := lastfm.Cache{}

	empty := map[string]string{}

	c.Set("test", empty, "test",
		http.Header{"Cache-Control": []string{"max-age=60"}})

	fh, err := os.Create(file)
	if err == nil {
		err = c.Store(fh)
		fh.Close()
	}
	if Expect(T, "save error", nil, err) {
		c.Purge()
		v, _ := c.Get("test", empty)
		Expect(T, "nothing to be cached", nil, v)

		fh, err = os.Open(file)
		if err == nil {
			err = c.Load(fh)
			fh.Close()
		}
		if Expect(T, "reload error", nil, err) {
			v, _ := c.Get("test", empty)
			Expect(T, "cached value", "test", v)
		}
	}
}
