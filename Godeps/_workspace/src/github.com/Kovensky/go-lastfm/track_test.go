package lastfm_test

import (
	"github.com/Kovensky/go-lastfm"
	"testing"
	"time"
)

func TestGetTrackInfo_ByMBID(T *testing.T) {
	T.Parallel()
	lfm := lastfm.Mock(lastfm.New("4c563adf68bc357a4570d3e7986f6481"))
	trackInfo, err := lfm.GetTrackInfo(
		lastfm.Track{MBID: "29b45fae-fc32-43c0-ab74-052842458315"}, "", false)

	if Expect(T, "error", nil, err) {
		Expect(T, "track ID", 4313, trackInfo.ID)
		Expect(T, "artist", "Daft Punk", trackInfo.Artist.Name)
		Expect(T, "album", "Discovery", trackInfo.Album.Name)
		Expect(T, "top tag", "electronic", trackInfo.TopTags[0])
		dur, _ := time.ParseDuration("212s")
		Expect(T, "duration", dur, trackInfo.Duration)
		Expect(T, "user playcount", 0, trackInfo.UserPlaycount)
	}
}

func TestGetTrackInfo_ByTrackArtist(T *testing.T) {
	T.Parallel()
	lfm := lastfm.Mock(lastfm.New("4c563adf68bc357a4570d3e7986f6481"))
	trackInfo, err := lfm.GetTrackInfo(
		lastfm.Track{Artist: lastfm.Artist{Name: "Daft Punk"}, Name: "Motherboard"},
		"Kovensky", false)

	if Expect(T, "error", nil, err) {
		Expect(T, "track ID", 651481384, trackInfo.ID)
		Expect(T, "album", "Random Access Memories", trackInfo.Album.Name)
		Expect(T, "tag count", 0, len(trackInfo.TopTags))
		dur, _ := time.ParseDuration("326s")
		Expect(T, "duration", dur, trackInfo.Duration)
		Expect(T, "loved", true, trackInfo.UserLoved)
		Expect(T, "user playcount", 64, trackInfo.UserPlaycount)
	}
}
