package lastfm

import (
	"encoding/xml"
	"strconv"
)

type RecentTracks struct {
	User       string  `xml:"user,attr"`
	Total      int     `xml:"total,attr"`
	Tracks     []Track `xml:"track"`
	NowPlaying *Track  `xml:"-"` // Points to the currently playing track, if any
}

func (tracks *RecentTracks) unmarshalHelper() (err error) {
	for i, track := range tracks.Tracks {
		if track.NowPlaying {
			tracks.NowPlaying = &tracks.Tracks[i]
		}
		err = tracks.Tracks[i].unmarshalHelper()
		if err != nil {
			return
		}
	}
	return
}

// Gets a list of recent tracks from the user. The .Tracks field includes the currently playing track,
// if any, and up to the count most recent scrobbles.
// The .NowPlaying field points to any currently playing track.
//
// See http://www.last.fm/api/show/user.getRecentTracks.
func (lfm *LastFM) GetRecentTracks(user string, count int) (tracks *RecentTracks, err error) {
	method := "user.getRecentTracks"
	query := map[string]string{
		"user":     user,
		"extended": "1",
		"limit":    strconv.Itoa(count)}

	if data, err := lfm.Cache.Get(method, query); data != nil {
		switch v := data.(type) {
		case RecentTracks:
			return &v, err
		case *RecentTracks:
			return v, err
		}
	} else if err != nil {
		return nil, err
	}

	body, hdr, err := lfm.doQuery(method, query)
	if err != nil {
		return
	}
	defer body.Close()

	status := lfmStatus{}
	err = xml.NewDecoder(body).Decode(&status)
	if err != nil {
		return
	}
	if status.Error.Code != 0 {
		err = &status.Error
		go lfm.Cache.Set(method, query, err, hdr)
		return
	}

	tracks = &status.RecentTracks
	err = tracks.unmarshalHelper()
	if err == nil {
		go lfm.Cache.Set(method, query, tracks, hdr)
	}
	return
}

type Tasteometer struct {
	Users   []string `xml:"input>user>name"`            // The compared users
	Score   float32  `xml:"result>score"`               // Varies from 0.0 to 1.0
	Artists []string `xml:"result>artists>artist>name"` // Short list of up to 5 common artists with the most affinity
}

// Compares the taste of 2 users.
//
// See http://www.last.fm/api/show/tasteometer.compare.
func (lfm *LastFM) CompareTaste(user1 string, user2 string) (taste *Tasteometer, err error) {
	method := "tasteometer.compare"
	query := map[string]string{
		"type1":  "user",
		"type2":  "user",
		"value1": user1,
		"value2": user2}

	if data, err := lfm.Cache.Get(method, query); data != nil {
		switch v := data.(type) {
		case Tasteometer:
			return &v, err
		case *Tasteometer:
			return v, err
		}
	} else if err != nil {
		return nil, err
	}

	body, hdr, err := lfm.doQuery(method, query)
	if err != nil {
		return
	}
	defer body.Close()

	status := lfmStatus{}
	err = xml.NewDecoder(body).Decode(&status)
	if err != nil {
		return
	}
	if status.Error.Code != 0 {
		err = &status.Error
		go lfm.Cache.Set(method, query, err, hdr)
		return
	}

	taste = &status.Tasteometer
	go lfm.Cache.Set(method, query, taste, hdr)
	return
}

type Neighbour struct {
	Name  string  `xml:"name"`
	Match float32 `xml:"match"`
}
type Neighbours []Neighbour

// Gets a list of up to limit closest neighbours of a user. A neighbour is another user
// that has high tasteometer comparison scores.
//
// See http://www.last.fm/api/show/user.getNeighbours
func (lfm *LastFM) GetUserNeighbours(user string, limit int) (neighbours Neighbours, err error) {
	method := "user.getNeighbours"
	query := map[string]string{
		"user":  user,
		"limit": strconv.Itoa(limit)}

	if data, err := lfm.Cache.Get(method, query); data != nil {
		return data.(Neighbours), err
	} else if err != nil {
		return nil, err
	}

	body, hdr, err := lfm.doQuery(method, query)
	if err != nil {
		return
	}
	defer body.Close()

	status := lfmStatus{}
	err = xml.NewDecoder(body).Decode(&status)
	if err != nil {
		return
	}
	if status.Error.Code != 0 {
		err = &status.Error
		go lfm.Cache.Set(method, query, err, hdr)
		return
	}

	neighbours = status.Neighbours
	go lfm.Cache.Set(method, query, neighbours, hdr)
	return
}

type Period int

const (
	Overall Period = 1 + iota
	OneWeek
	OneMonth
	ThreeMonths
	SixMonths
	OneYear
)

var periodStringMap = map[Period]string{
	Overall:     "overall",
	OneWeek:     "7day",
	OneMonth:    "1month",
	ThreeMonths: "3month",
	SixMonths:   "6month",
	OneYear:     "12month"}

func (p Period) String() string {
	return periodStringMap[p]
}

type TopArtists struct {
	User   string `xml:"user,attr"`
	Period Period `xml:"-"`
	Total  int    `xml:"total,attr"`

	Artists []Artist `xml:"artist"`

	// For internal use
	RawPeriod string `xml:"type,attr"`
}

func (top *TopArtists) unmarshalHelper() (err error) {
	for k, v := range periodStringMap {
		if top.RawPeriod == v {
			top.Period = k
			break
		}
	}
	return
}

// Gets a list of the (up to limit) most played artists of a user within a Period.
//
// See http://www.last.fm/api/show/user.getTopArtists.
func (lfm *LastFM) GetUserTopArtists(user string, period Period, limit int) (top *TopArtists, err error) {
	method := "user.getTopArtists"
	query := map[string]string{
		"user":   user,
		"period": periodStringMap[period],
		"limit":  strconv.Itoa(limit)}

	if data, err := lfm.Cache.Get(method, query); data != nil {
		switch v := data.(type) {
		case TopArtists:
			return &v, err
		case *TopArtists:
			return v, err
		}
	} else if err != nil {
		return nil, err
	}

	body, hdr, err := lfm.doQuery(method, query)
	if err != nil {
		return
	}
	defer body.Close()

	status := lfmStatus{}
	err = xml.NewDecoder(body).Decode(&status)
	if err != nil {
		return
	}
	if status.Error.Code != 0 {
		err = &status.Error
		go lfm.Cache.Set(method, query, err, hdr)
		return
	}

	top = &status.TopArtists
	err = top.unmarshalHelper()
	if err == nil {
		go lfm.Cache.Set(method, query, top, hdr)
	}
	return
}
