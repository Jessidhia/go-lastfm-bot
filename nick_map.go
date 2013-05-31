package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Kovensky/go-lastfm"
	"github.com/fluffle/goirc/client"
	"github.com/howeyc/fsnotify"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	requireAuth = flag.Bool("require-auth", true,
		`Requires that nicknames be authenticated for using the user-nick mapping. `+
			`Disable on networks that don't implement a NickServ, such as EFNet.`)

	saveNicks = flag.Bool("save-nicks", true, `Whether to persist the user-nick mappings.`)
	nickFile  = flag.String("nick-file", "", `JSON file where user-nick map is stored. If blank, {{server}}.nicks.json is used.`)
)

type NickMap struct {
	nickMap    map[string]string
	reverseMap map[string][]string
	beingSaved bool
	sync.Mutex
}

func NewNickMap() *NickMap {
	return &NickMap{
		nickMap:    make(map[string]string),
		reverseMap: make(map[string][]string)}
}

func loadNickMap() {
	if *saveNicks {
		path := *nickFile
		if path == "" {
			path = *server + ".nicks.json"
		}
		log.Println("Watching", path)
		decodeNickMap(path)
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatalln(err)
		}
		err = watcher.Watch(path)
		if err != nil {
			log.Println(err)
		}
		go func() {
			var timer <-chan time.Time
			for {
				select {
				case <-watcher.Event:
					timer = time.After(time.Second)
				case err := <-watcher.Error:
					log.Println(err)
				case <-timer:
					if !nickMap.beingSaved {
						log.Println("Nick persistence file changed, reloading...")
						decodeNickMap(path)
					}
					nickMap.beingSaved = false
					timer = nil
				}
			}
		}()
	}
}

func decodeNickMap(path string) {
	fh, err := os.Open(path)
	if err != nil {
		log.Println("Error opening nick persistence file:", err)
	} else {
		nickMap.Lock()
		j := json.NewDecoder(fh)
		err = j.Decode(nickMap)
		nickMap.Unlock()
		if err != nil {
			log.Println("Error reading nick-user map:", err)
		}
		fh.Close()
	}
}

func saveNickMap() {
	if *saveNicks {
		nickMap.beingSaved = true
		path := *nickFile
		if path == "" {
			path = *server + ".nicks.json"
		}
		fh, err := os.Create(path)
		if err != nil {
			log.Println("Error creating nick persistence file:", err)
		} else {
			b, err := json.MarshalIndent(nickMap, "", "\t")
			if err != nil {
				log.Println("Error marshaling nick-user map:", err)
			}
			_, err = fh.Write(b)
			if err != nil {
				log.Println("Error writing persistence file:", err)
			}

			fh.Close()
		}
	}
}

func addNickHandlers(irc *client.Conn) {
	if !flag.Parsed() {
		flag.Parse()
	}

	irc.AddHandler("QUIT", dropIdentifiedCache)
	irc.AddHandler("307", isIdentified)
	irc.AddHandler("330", isIdentified)
	irc.AddHandler("318", isIdentified)
}

func (m *NickMap) MarshalJSON() (j []byte, err error) {
	data := make(map[string][]string)
	for _, rm := range m.reverseMap {
		if len(rm) > 0 {
			user := m.nickMap[strings.ToLower(rm[0])]
			data[user] = rm
		}
	}
	return json.Marshal(data)
}

func (m *NickMap) UnmarshalJSON(j []byte) (err error) {
	data := make(map[string][]string)
	err = json.Unmarshal(j, &data)
	if err != nil {
		return
	}
	nickMap.nickMap = make(map[string]string)
	nickMap.reverseMap = make(map[string][]string)
	for user, nicks := range data {
		for _, nick := range nicks {
			m.setUser(nick, user)
		}
	}
	return
}

type NickMapError string

func (err *NickMapError) Error() string {
	return string(*err)
}

func (m *NickMap) IgnoreNick(irc *client.Conn, target, nick string) (err error) {
	log.Println("Adding", nick, "to ignored list")
	identified := checkIdentified(irc, nick)
	if !identified {
		irc.Privmsg(target, fmt.Sprintf("%s: you must be identified with NickServ to use this command", nick))
		log.Println("Nick", nick, "is not identified, and identity verification is enabled")
		e := NickMapError("nick is not identified")
		return &e
	}
	m.Lock()
	m.setUser(nick, "")
	saveNickMap()
	m.Unlock()

	r := fmt.Sprintf("[%s] is now ignored by last.fm commands; use %sdeluser to be unignored", nick, *cmdPrefix)
	log.Println(r)
	irc.Privmsg(target, r)
	return nil
}

func (m *NickMap) AddNick(irc *client.Conn, target, nick, user string) (err error) {
	log.Println("Associating", nick, "with user", user)
	identified := checkIdentified(irc, nick)
	if !identified {
		irc.Privmsg(target, fmt.Sprintf("%s: you must be identified with NickServ to use this command", nick))
		log.Println("Nick", nick, "is not identified, and identity verification is enabled")
		e := NickMapError("nick is not identified")
		return &e
	}
	log.Println("Checking whether", user, "is a valid Last.fm user for associating with", nick)
	// Smallest query we can do (we're only interested in errors)
	_, err = lfm.GetUserTopArtists(user, lastfm.OneWeek, 1)
	if err != nil {
		extra := ""
		lfmerr, ok := err.(*lastfm.LastFMError)
		// lfmerr.Code is unreliable; a lot of things may be code 6...
		if ok && lfmerr.Error() == "No user with that name" {
			// plus their error messages are misleading...
			extra = ", or user never scrobbled anything"
		}

		r := fmt.Sprintf("[%s] %v%s", nick, err, extra)
		log.Println(r)
		irc.Privmsg(target, r)
		return err
	}
	m.Lock()
	m.setUser(nick, user)
	saveNickMap()
	m.Unlock()

	r := fmt.Sprintf("[%s] is now associated with last.fm user %s", nick, user)
	log.Println(r)
	irc.Privmsg(target, r)
	return nil
}

func (m *NickMap) DelNick(irc *client.Conn, target, nick string) (err error) {
	if user, ok := m.GetUser(nick); !ok {
		log.Println("Nick", nick, "asked for dissociation but isn't associated")
		irc.Privmsg(target, fmt.Sprintf("%s: you're not associated with an username", nick))
		e := NickMapError("nick isn't associated")
		return &e
	} else {
		log.Println("Dissociating", nick, "with user", user)
		identified := checkIdentified(irc, nick)
		if !identified {
			irc.Privmsg(target, fmt.Sprintf("%s: you must be identified with NickServ to use this command", nick))
			log.Println("Nick", nick, "is not identified, and identity verification is enabled")
			e := NickMapError("nick is not identified")
			return &e
		}
		m.Lock()
		m.delUser(nick)
		m.Unlock()
		r := fmt.Sprintf("[%s] is no longer associated with last.fm user %s", nick, user)
		log.Println(r)
		irc.Privmsg(target, r)
		return nil
	}
	panic("unreachable")
}

func (m *NickMap) QueryNick(irc *client.Conn, target, asker, nick string) {
	r := ""
	if user, ok := m.GetUser(nick); ok {
		if asker == nick {
			if user == "" {
				r = fmt.Sprintf("%s: you asked to be ignored by last.fm commands", asker)
			} else {
				r = fmt.Sprintf("%s: your ", asker)
			}
		} else {
			if user == "" {
				r = fmt.Sprintf("%s: %s has asked to be ignored by last.fm commands", asker, nick)
			} else {
				r = fmt.Sprintf("%s: %s's ", asker, nick)
			}
		}
		if user != "" {
			r += fmt.Sprintf("last.fm username is %s (http://last.fm/user/%s)", user, user)
		}
	} else {
		if asker == nick {
			r = fmt.Sprintf("%s: you ", asker)
		} else {
			r = fmt.Sprintf("%s: %s ", asker, nick)
		}
		r += fmt.Sprintf("didn't associate an username")
	}
	irc.Privmsg(target, r)
	return
}

func (m *NickMap) ListAllNicks(irc *client.Conn, target, asker, user string) {
	if user == "" {
		return
	}
	log.Println("Listing all nicks for user", user)
	r := ""
	if nicks, ok := m.reverseMap[strings.ToLower(user)]; !ok || len(nicks) == 0 {
		r = fmt.Sprintf("%s: %s has no associated IRC nick", asker, user)
	} else {
		plural := "s are"
		if len(nicks) == 1 {
			plural = " is"
		}
		r = fmt.Sprintf("%s: %s's known IRC nick%s %s",
			asker, user, plural, strings.Join(sort.StringSlice(nicks), ", "))
	}
	log.Println(r)
	irc.Privmsg(target, r)
	return
}

func (m *NickMap) GetUser(nick string) (user string, ok bool) {
	m.Lock()
	defer m.Unlock()
	user, ok = m.nickMap[strings.ToLower(nick)]
	if ok {
		if user == "" {
			log.Println("Nick", nick, "requested to be ignored")
			return user, ok
		}
		log.Println("Nick", nick, "is associated with", user)
		return user, ok
	}
	return nick, ok
}

func (m *NickMap) setUser(nick, user string) {
	if _, ok := m.nickMap[strings.ToLower(nick)]; ok {
		m.delUser(nick)
	}
	m.nickMap[strings.ToLower(nick)] = user
	lcuser := strings.ToLower(user)
	if rm, ok := m.reverseMap[lcuser]; !ok {
		m.reverseMap[lcuser] = []string{nick}
	} else {
		m.reverseMap[lcuser] = append(rm, nick)
	}
	return
}

func (m *NickMap) delUser(nick string) {
	lcuser := ""
	if user, ok := m.nickMap[strings.ToLower(nick)]; ok {
		lcuser = strings.ToLower(user)
	}
	delete(m.nickMap, strings.ToLower(nick))

	if rm, ok := m.reverseMap[lcuser]; ok {
		newRm := []string{}
		for _, n := range rm {
			if n != nick {
				newRm = append(newRm, n)
			}
		}
		m.reverseMap[lcuser] = newRm
	}
}

var (
	isIdentifiedChan  = make(map[string]chan bool)
	isIdentifiedCache = make(map[string]bool)
)

func resetIdentifiedCache() {
	isIdentifiedCache = make(map[string]bool)
	// We also reset the channels, in case there's any verification pending
	for _, c := range isIdentifiedChan {
		close(c)
	}
	isIdentifiedChan = make(map[string]chan bool)
	return
}

func dropIdentifiedCache(irc *client.Conn, line *client.Line) {
	delete(isIdentifiedCache, line.Nick)
	return
}

func checkIdentified(irc *client.Conn, nick string) bool {
	if !*requireAuth {
		return true
	}
	// We don't cache identification failures since the user can always identify later
	if isIdentifiedCache[nick] {
		return true
	}

	log.Println("Checking whether", nick, "is identified")

	isIdentifiedChan[nick] = make(chan bool, 1)
	timeout := time.After(10 * time.Second)
	go irc.Whois(nick)

	r := false
	for is, ok := false, true; ok; {
		select {
		case is, ok = <-isIdentifiedChan[nick]:
			if is {
				r = true
			}
		case <-timeout:
			log.Println("Timeout checking for whether", nick, "is identified")
			return false
		}
	}
	delete(isIdentifiedChan, nick)

	log.Println("Is nick", nick, "identified:", r)
	if r {
		isIdentifiedCache[nick] = true
	}

	return r
}

var isIdentifiedMutex sync.Mutex

func isIdentified(irc *client.Conn, line *client.Line) {
	nick := line.Args[1]
	isIdentifiedMutex.Lock()
	switch line.Cmd {
	case "307", "330": // identified; 330 is the freenode version
		isIdentifiedChan[nick] <- true
	case "318": // end of response
		close(isIdentifiedChan[nick])
	}
	isIdentifiedMutex.Unlock()
	return
}
