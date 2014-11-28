package lastfm

import (
	"strings"
	"time"
)

// Some structs need extra processing after XML unmarshalling.
type unmarshalHelper interface {
	unmarshalHelper() error
}

type LastFMError struct {
	error
	Code    int    `xml:"code,attr"`
	Message string `xml:",chardata"`
}

func (e *LastFMError) Error() string {
	return strings.Trim(e.Message, "\n ")
}

type Artist struct {
	Name      string `xml:"name"`
	PlayCount int    `xml:"playcount"` // Currently is always 0, except when part of the result of GetUserTopArtists.
	MBID      string `xml:"mbid"`
	URL       string `xml:"url"`
}

// Less detailed struct returned in GetRecentTracks.
type Album struct {
	Name string `xml:",chardata"`
	MBID string `xml:"mbid,attr"`
}

// More detailed struct returned in GetTrackInfo.
type AlbumInfo struct {
	TrackNo int    `xml:"position,attr"`
	Name    string `xml:"title"`
	Artist  string `xml:"artist"`
	MBID    string `xml:"mbid"`
	URL     string `xml:"url"`
}

type Track struct {
	NowPlaying bool      `xml:"nowplaying,attr"`
	Artist     Artist    `xml:"artist"`
	Album      Album     `xml:"album"`
	Loved      bool      `xml:"loved"`
	Name       string    `xml:"name"`
	MBID       string    `xml:"mbid"`
	URL        string    `xml:"url"`
	Date       time.Time `xml:"-"`

	// For internal use
	RawDate lfmDate `xml:"date"`
}

func (track *Track) unmarshalHelper() (err error) {
	if track.RawDate.Date != "" {
		track.Date = time.Unix(track.RawDate.UTS, 0)
	}
	return
}

type Wiki struct {
	Published time.Time `xml:"-"`
	Summary   string    `xml:"summary"`
	Content   string    `xml:"content"`

	// For internal use
	RawPublished string `xml:"published"`
}

func (wiki *Wiki) unmarshalHelper() (err error) {
	if wiki.RawPublished != "" {
		wiki.Published, err = time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", wiki.RawPublished)
	}
	return
}
