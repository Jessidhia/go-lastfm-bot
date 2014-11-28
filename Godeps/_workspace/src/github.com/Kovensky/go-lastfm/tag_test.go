package lastfm_test

import (
	"github.com/Kovensky/go-lastfm"
	"testing"
)

func TestGetTrackTopTags(T *testing.T) {
	T.Parallel()
	lfm := lastfm.Mock(lastfm.New("4c563adf68bc357a4570d3e7986f6481"))
	topTags, err := lfm.GetTrackTopTags(
		lastfm.Track{MBID: "48fa1cab-5250-4767-bbdf-14e0ef563d11"}, false)

	if Expect(T, "error", nil, err) {
		Expect(T, "track name", "One More Time", topTags.Track)
		Expect(T, "top tag", "electronic", topTags.Tags[0].Name)
		Expect(T, "top tag count", 100, topTags.Tags[0].Count)
	}
}

func TestGetArtistTopTags(T *testing.T) {
	T.Parallel()
	lfm := lastfm.Mock(lastfm.New("4c563adf68bc357a4570d3e7986f6481"))
	topTags, err := lfm.GetArtistTopTags(
		lastfm.Artist{Name: "Daft Punk"}, false)

	if Expect(T, "error", nil, err) {
		Expect(T, "track name", "", topTags.Track)
		Expect(T, "top tag", "electronic", topTags.Tags[0].Name)
		Expect(T, "top tag count", 100, topTags.Tags[0].Count)
	}
}
