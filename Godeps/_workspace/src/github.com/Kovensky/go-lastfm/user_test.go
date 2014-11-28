package lastfm_test

import (
	"github.com/Kovensky/go-lastfm"
	"testing"
)

// TODO: more coverage?

func TestGetRecentTracks(T *testing.T) {
	T.Parallel()
	lfm := lastfm.Mock(lastfm.New("4c563adf68bc357a4570d3e7986f6481"))
	tracks, err := lfm.GetRecentTracks("Kovensky", 1)

	if Expect(T, "error", nil, err) {
		Expect(T, "scrobble count", 39679, tracks.Total)
		Expect(T, "now playing track", &tracks.Tracks[0], tracks.NowPlaying)
		Expect(T, "first track's loved status to be", true, tracks.Tracks[0].Loved)
	}
}

func TestCompareTaste(T *testing.T) {
	T.Parallel()
	lfm := lastfm.Mock(lastfm.New("4c563adf68bc357a4570d3e7986f6481"))
	taste, err := lfm.CompareTaste("Kovensky", "D4RK-PH0ENIX")

	if Expect(T, "error", nil, err) {
		Expect(T, "artist count", 5, len(taste.Artists))
		Expect(T, "top similar artist", "DIR EN GREY", taste.Artists[0])
	}
}

func TestGetUserNeighbours(T *testing.T) {
	T.Parallel()
	lfm := lastfm.Mock(lastfm.New("4c563adf68bc357a4570d3e7986f6481"))
	n, err := lfm.GetUserNeighbours("Kovensky", 1)

	if Expect(T, "error", nil, err) {
		Expect(T, "neighbour count", 1, len(n))
		Expect(T, "neighbour", "AT_Field", n[0].Name)
	}
}

func TestGetUserTopArtists(T *testing.T) {
	T.Parallel()
	lfm := lastfm.Mock(lastfm.New("4c563adf68bc357a4570d3e7986f6481"))
	t, err := lfm.GetUserTopArtists("Kovensky", lastfm.Overall, 1)

	if Expect(T, "error", nil, err) {
		Expect(T, "user", "Kovensky", t.User)
		Expect(T, "period", lastfm.Overall, t.Period)
		if Expect(T, "artist count", 1, len(t.Artists)) {
			Expect(T, "top artist", "CROW'SCLAW", t.Artists[0].Name)
		}
	}
}
