package lastfm

import "encoding/xml"

type Tag struct {
	Name  string `xml:"name"`
	Count int    `xml:"count"`
	URL   string `xml:"url"`
}

type TopTags struct {
	Artist string `xml:"artist,attr"`
	Track  string `xml:"track,attr"`
	Tags   []Tag  `xml:"tag"`
}

// Gets the top tags for a Track. The second argument tells last.fm whether
// it is to apply autocorrections to the name/artist.
//
// The Track struct must specify either the MBID or both Artist.Name and Name.
// Example literals that can be given as the first argument:
//   lastfm.Track{MBID: "mbid"}
//   lastfm.Track{Artist: lastfm.Artist{Name: "Artist"}, Name: "Track"}
//
// See http://www.last.fm/api/show/track.getTopTags.
func (lfm *LastFM) GetTrackTopTags(track Track, autocorrect bool) (toptags *TopTags, err error) {
	method := "track.getTopTags"
	query := map[string]string{}
	if autocorrect {
		query["autocorrect"] = "1"
	} else {
		query["autocorrect"] = "0"
	}

	if track.MBID != "" {
		query["mbid"] = track.MBID
	} else {
		query["artist"] = track.Artist.Name
		query["track"] = track.Name
	}

	if data, err := lfm.cacheGet(method, query); data != nil {
		switch v := data.(type) {
		case TopTags:
			return &v, err
		case *TopTags:
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
		go lfm.cacheSet(method, query, err, hdr)
		return
	}

	toptags = &status.TopTags
	go lfm.cacheSet(method, query, toptags, hdr)
	return
}

// Gets the top tags for an Artist. The second argument tells last.fm whether
// it is to apply autocorrections to the artist name.
//
// The Artist struct must specify either the MBID or the Name.
// Example literals that can be given as the first argument:
//   lastfm.Artist{MBID: "mbid"}
//   lastfm.Artist{Name: "Artist"}
//
// See http://www.last.fm/api/show/artist.getTopTags.
func (lfm *LastFM) GetArtistTopTags(artist Artist, autocorrect bool) (toptags *TopTags, err error) {
	method := "artist.getTopTags"
	query := map[string]string{}
	if autocorrect {
		query["autocorrect"] = "1"
	} else {
		query["autocorrect"] = "0"
	}

	if artist.MBID != "" {
		query["mbid"] = artist.MBID
	} else {
		query["artist"] = artist.Name
	}

	if data, err := lfm.cacheGet(method, query); data != nil {
		switch v := data.(type) {
		case TopTags:
			return &v, err
		case *TopTags:
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
		go lfm.cacheSet(method, query, err, hdr)
		return
	}

	toptags = &status.TopTags
	go lfm.cacheSet(method, query, toptags, hdr)
	return
}
