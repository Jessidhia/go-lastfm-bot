package main

import (
	"compress/zlib"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/Kovensky/go-lastfm"
	"github.com/fluffle/goirc/client"
)

var (
	botNick     = flag.String("nick", "Lastfm_bot", `The nickname the bot should use.`)
	server      = flag.String("server", "", `The IRC server to connect to. Required. If a ":" is present, uses the right side as the port, otherwise connects to port 6667.`)
	useSSL      = flag.Bool("ssl", false, `Whether to use explicit SSL. Changes the default port to 6697.`)
	password    = flag.String("password", "", `The password needed to connect to the server, if any.`)
	nickPass    = flag.String("nickserv-password", "", `A NickServ password to authenticate the bot, if any. Tested on Freenode and SynIRC.`)
	channelList = flag.String("channels", "", `Comma-separated list of channels to join on the server. Required`)
	apiKey      = flag.String("api-key", "", `The Last.fm API key. Required.`)
	cmdPrefix   = flag.String("cmd-prefix", ".", `The prefix to user commands.`)
	cacheFile   = flag.String("cache-file", "", `File used to persist the last.fm API cache. If blank, the cache is only kept in memory. Not multiprocess safe.`)
	lfm         lastfm.LastFM
	nickMap     = NewNickMap()
	cacheTimer  *time.Timer
)

var whitespace = regexp.MustCompile(`\s+`)

func onPrivmsg(irc *client.Conn, line *client.Line) {
	words := whitespace.Split(line.Args[1], -1)
	switch words[0] {
	case *cmdPrefix + "wp":
		go reportAllNowPlaying(irc, line.Nick, line.Args[0])
	case *cmdPrefix + "np":
		who := line.Nick
		if len(words) > 1 && words[1] != "" {
			who = words[1]
		}
		go reportNowPlaying(irc, line.Args[0], line.Nick, who, false)
	case *cmdPrefix + "compare":
		who := line.Nick
		target := ""
		if len(words) > 2 && words[2] != "" {
			who = words[1]
			target = words[2]
		} else if len(words) > 1 {
			target = words[1]
		} else {
			irc.Privmsg(line.Args[0], fmt.Sprintf("%s: tell me who to compare to!", line.Nick))
			return
		}
		go doCompare(irc, line.Args[0], line.Nick, who, target)
	case *cmdPrefix + "top5":
		who := line.Nick
		period := lastfm.Overall
		if len(words) > 2 && words[2] != "" {
			who = words[2]
		}
		if len(words) > 1 {
			switch words[1] {
			case "overall":
				period = lastfm.Overall
			case "year":
				period = lastfm.OneYear
			case "month":
				period = lastfm.OneMonth
			case "week":
				period = lastfm.OneWeek
			default:
				irc.Privmsg(line.Args[0],
					fmt.Sprintf("%s: usage: %stop5 ((overall|year|month|week) ($nick)?)?", line.Nick, *cmdPrefix))
				return
			}
		}
		go doTop5(irc, line.Args[0], line.Nick, period, who)
	case *cmdPrefix + "setuser":
		if len(words) < 2 || words[1] == "" {
			irc.Privmsg(line.Args[0], fmt.Sprintf("%s: tell the username to associate with", line.Nick))
			return
		}
		go nickMap.AddNick(irc, line.Args[0], line.Nick, words[1])
	case *cmdPrefix + "ignore":
		go nickMap.IgnoreNick(irc, line.Args[0], line.Nick)
	case *cmdPrefix + "deluser":
		go nickMap.DelNick(irc, line.Args[0], line.Nick)
	case *cmdPrefix + "whois":
		who := line.Nick
		if len(words) > 1 && words[1] != "" {
			who = words[1]
		}
		go nickMap.QueryNick(irc, line.Args[0], line.Nick, who)
	case *cmdPrefix + "aka":
		if len(words) < 2 {
			irc.Privmsg(line.Args[0], fmt.Sprintf("%s: tell the username to know about", line.Nick))
			return
		}
		go nickMap.ListAllNicks(irc, line.Args[0], line.Nick, words[1])
	case *cmdPrefix + "help":
		go sendHelp(irc, line.Nick)
	}
}

var helpSplit = regexp.MustCompile(`\s{2,}`)

func sendHelp(irc *client.Conn, nick string) {
	helpStr := `
	Last.fm commands:
	` + *cmdPrefix + `np ($user)?: Shows your now playing song. If you give $user, queries for that $user.
	` + *cmdPrefix + `compare ($user1) ($user2)?: Runs a tasteometer compare between you and $user1, or between $user1 and $user2 if present.
	` + *cmdPrefix + `top5 ((overall|year|month|week) ($user)?)?: Shows the top5 artists in the chosen period for you or the $user.
	` + *cmdPrefix + `whois ($nick)?: Shows your associated last.fm username, or the username associated with $nick.
	` + *cmdPrefix + `aka ($username): Shows the nicks that have been associated with $username.
	A nick can be used in place of a username if it's associated with a last.fm account.
	`
	if *requireAuth {
		helpStr += `Commands that require that you be authenticated with NickServ:`
	}
	// There's also *cmdPrefix + "wp", but we don't document it to not encourage abuse.
	helpStr += `
	` + *cmdPrefix + `ignore: Makes the bot ignore you for most commands. Use ` +
		*cmdPrefix + `setuser or ` + *cmdPrefix + `deluser to be unignored.
	` + *cmdPrefix + `setuser ($username): Associates your nick with the given last.fm $username.
	` + *cmdPrefix + `deluser: Removes your nick's association, if any.
	` // + *cmdPrefix + `wp: Shows what's playing for everyone in the channel.` // uncomment this at your peril :)
	for _, line := range helpSplit.Split(helpStr, -1) {
		if line != "" {
			irc.Notice(nick, line)
		}
	}
}

func reportIgnored(irc *client.Conn, asker, who string) {
	if asker == who {
		irc.Notice(asker, "You asked to be ignored by last.fm commands")
	} else {
		irc.Notice(asker, fmt.Sprintf("%s asked to be ignored by last.fm commands", who))
	}
}

func onInvite(irc *client.Conn, line *client.Line) {
	who, channel := line.Args[0], line.Args[1]
	log.Println(line.Nick, "invited bot to", channel)
	if who == irc.Me().Nick {
		// some IRCds only allow operators to INVITE, and on registered channels normally only identified users are operators
		// check anyway, since there are some corner cases where that doesn't happen
		if checkIdentified(irc, line.Nick) {
			log.Println("Accepting invite to", channel)
			irc.Join(channel)
		} else {
			irc.Notice(line.Nick, "you must be identified to invite")
			log.Println("Ignoring invite, user is not identified")
		}
	}
}

func doTop5(irc *client.Conn, target, asker string, period lastfm.Period, user string) {
	log.Println("Listing top", period, "5 artists for", user)
	lfmUser, _ := nickMap.GetUser(user)
	if lfmUser == "" {
		reportIgnored(irc, asker, user)
		return
	}
	top5, err := lfm.GetUserTopArtists(lfmUser, period, 5)
	if err != nil {
		irc.Privmsg(target, fmt.Sprintf("[%s] %v", user, err))
		return
	}
	artists := []string{}
	for _, a := range top5.Artists {
		artists = append(artists, a.Name)
	}
	r := fmt.Sprintf("[%s] %v top5: %s",
		user, period, strings.Join(artists, ", "))
	log.Println("Reply:", r)
	irc.Privmsg(target, r)
	saveCache()
}

func doCompare(irc *client.Conn, target, asker, user1, user2 string) {
	log.Println("Comparing", user1, "with", user2)
	lfmUser1, _ := nickMap.GetUser(user1)
	lfmUser2, _ := nickMap.GetUser(user2)
	if lfmUser1 == "" || lfmUser2 == "" {
		if lfmUser1 == "" {
			reportIgnored(irc, asker, user1)
		} else {
			reportIgnored(irc, asker, user2)
		}
		return
	}
	taste, err := lfm.CompareTaste(lfmUser1, lfmUser2)
	if err != nil {
		irc.Privmsg(target, fmt.Sprintf("[%s vs %s] %v", user1, user2, err))
		return
	}
	r := fmt.Sprintf("[%s vs %s] %.2f%% -- %s",
		user1, user2, taste.Score*100, strings.Join(taste.Artists, ", "))
	log.Println("Reply:", r)
	irc.Privmsg(target, r)
	saveCache()
}

func reportNowPlaying(irc *client.Conn, target, asker, who string, onlyReportSuccess bool) bool {
	log.Println("Reporting Now Playing for", who, "on channel", target)
	user, _ := nickMap.GetUser(who)
	if user == "" {
		if !onlyReportSuccess {
			reportIgnored(irc, asker, who)
		}
		return false
	}
	recent, err := lfm.GetRecentTracks(user, 1)
	if err != nil {
		extra := ""
		lfmerr, ok := err.(*lastfm.LastFMError)
		// lfmerr.Code is unreliable; a lot of things may be code 6...
		if ok && lfmerr.Error() == "No user with that name was found" {
			// plus their error messages are misleading...
			extra = ", or user never scrobbled anything"
		}
		r := fmt.Sprintf("[%s] %v%s", who, err, extra)
		if !onlyReportSuccess {
			log.Println("Reply:", r)
			irc.Privmsg(target, r)
		} else {
			log.Println(r)
		}
		saveCache()
		return false
	}
	np := recent.NowPlaying
	if np != nil {
		c := make(chan interface{})
		go func() {
			r, err := lfm.GetTrackInfo(*np, user, true)
			if err != nil {
				c <- err
			} else {
				c <- r
			}
		}()
		go func() {
			r, err := lfm.GetTrackTopTags(*np, true)
			if err != nil {
				c <- err
			} else {
				c <- r
			}
		}()
		go func() {
			r, err := lfm.GetArtistTopTags(np.Artist, true)
			if err != nil {
				c <- err
			} else {
				c <- r
			}
		}()

		var ti *lastfm.TrackInfo
		var topTags *lastfm.TopTags

		for reads := 0; reads < 3; reads++ {
			select {
			case rep := <-c:
				switch r := rep.(type) {
				case *lastfm.TrackInfo:
					ti = r
				case *lastfm.TopTags:
					if r.Track != "" { // track tags
						// only use if present
						if len(r.Tags) > 0 {
							topTags = r
						}
					} else { // artist tags
						// only use if there are no track tags
						if topTags == nil || len(topTags.Tags) == 0 {
							topTags = r
						}
					}
				case error:
					log.Println(err)
				default:
					panic(r)
				}
			}
		}
		close(c)

		if ti == nil {
			ti = &lastfm.TrackInfo{
				Artist:        np.Artist,
				Name:          np.Name,
				Duration:      -1,
				UserPlaycount: -1,
			}
		}

		reply := []string{
			fmt.Sprintf("[%s] np: %s - %s", who, ti.Artist.Name, ti.Name)}
		info := []string{}
		if ti.UserLoved {
			info = append(info, "<3")
		}
		if ti.UserPlaycount > 0 {
			info = append(info, fmt.Sprintf("playcount %dx", ti.UserPlaycount))
		} else {
			info = append(info, "first listen")
		}
		reply = append(reply, fmt.Sprintf("[%s]", strings.Join(info, " - ")))

		tags := []string{}
		for i := 0; topTags != nil && i < 5 && i < len(topTags.Tags); i++ {
			tags = append(tags, topTags.Tags[i].Name)
		}
		reply = append(reply, fmt.Sprintf("(%s)", strings.Join(tags, ", ")))

		if ti.Duration != 0 {
			reply = append(reply, fmt.Sprintf("[%v]", ti.Duration))
		}

		r := strings.Join(reply, " ")
		log.Println("Reply:", r)
		irc.Privmsg(target, r)
		saveCache()
		return true
	} else if len(recent.Tracks) > 0 && !onlyReportSuccess {
		tr := recent.Tracks[0]
		reply := []string{
			fmt.Sprintf("[%s] last listened to %s - %s,", who, tr.Artist.Name, tr.Name)}
		if (tr.Date != time.Time{}) {
			diff := time.Now().Sub(tr.Date)
			// round the diff to the nearest second
			diff += time.Second / 2
			diff -= diff % time.Second

			pre := ""
			years := 0
			for diff > 365*24*time.Hour {
				// I know it's inaccurate, it's just for simplification
				diff -= 365 * 24 * time.Hour
				years += 1
			}
			if years > 0 {
				pre += fmt.Sprintf("%dy", years)
			}

			days := 0
			for diff > 24*time.Hour {
				diff -= 24 * time.Hour
				days += 1
			}
			if days > 0 {
				pre += fmt.Sprintf("%dd", days)
			}

			reply = append(reply, fmt.Sprintf("%s%v ago", pre, diff))
		} else {
			reply = append(reply, "not even last.fm knows when")
		}
		r := strings.Join(reply, " ")
		log.Println("Reply:", r)
		irc.Privmsg(target, r)
	} else if !onlyReportSuccess {
		r := fmt.Sprintf("[%s] never scrobbled anything", who)
		log.Println("Reply:", r)
		irc.Privmsg(target, r)
	} else {
		log.Printf("[%s] is not listening to anything\n", who)
	}
	saveCache()
	return false
}

var sig chan os.Signal

func main() {
	flag.Parse()
	if *apiKey == "" {
		log.Fatalln("Missing API key, provide one using -api-key")
	}
	lfm = lastfm.New(*apiKey)
	loadNickMap()
	loadCache()

	if *cacheFile != "" {
		cacheTimer = time.NewTimer(120 * time.Second)
		go func() {
			for _ = range cacheTimer.C {
				saveCacheNow()
				cacheTimer.Reset(120 * time.Second)
			}
		}()
	}

	if *server == "" {
		log.Fatalln("No server to connect to")
	}
	if *channelList == "" {
		log.Fatalln("No channels to join")
	}

	config := client.NewConfig(*botNick)
	config.Version = "github.com/Kovensky/go-lastfm-bot"
	config.SSL = *useSSL
	config.Flood = false

	// TODO: CA management?
	config.SSLConfig = &tls.Config{InsecureSkipVerify: true}

	irc := client.Client(config)

	addNickHandlers(irc)
	addWhoHandlers(irc)

	irc.HandleFunc(client.CONNECTED, func(irc *client.Conn, line *client.Line) {
		if *nickPass != "" {
			if irc.Me().Nick != *botNick {
				log.Println("Nick", *botNick, "was not available; trying to retake it")
				irc.Privmsg("NickServ", fmt.Sprintf("GHOST %s %s", *botNick, *nickPass))
			} else {
				log.Println("Identifying with NickServ")
				irc.Privmsg("NickServ", fmt.Sprintf("IDENTIFY %s", *nickPass))
			}
		}
		log.Println("Connected; joining", *channelList)
		irc.Join(*channelList)
	})
	irc.HandleFunc("NOTICE", func(irc *client.Conn, line *client.Line) {
		if strings.ToLower(line.Nick) == "nickserv" {
			log.Println("NickServ:", line.Args[1])
			switch {
			case strings.Contains(strings.ToLower(line.Args[1]), "ghost"):
				log.Println("Ghost command successful")
				log.Println("Changing nick to", *botNick)
				irc.Nick(*botNick)
			case strings.Contains(line.Args[1], "identified"),
				strings.Contains(line.Args[1], "recognized"):
				log.Println("Successfully identified with NickServ; joining", *channelList)
				irc.Join(*channelList)
			}
		}
	})
	irc.HandleFunc("QUIT", func(irc *client.Conn, line *client.Line) {
		if line.Nick == *botNick {
			log.Println("Nick", *botNick, "now available, changing to it")
			irc.Nick(*botNick)
		}
	})
	irc.HandleFunc("NICK", func(irc *client.Conn, line *client.Line) {
		if line.Args[len(line.Args)-1] == irc.Me().Nick {
			log.Println("Nick successfully changed to", irc.Me().Nick)
			if *nickPass != "" {
				log.Println("Identifying with NickServ")
				irc.Privmsg("NickServ", fmt.Sprintf("IDENTIFY %s", *nickPass))
			}
		}
	})
	irc.HandleFunc("332", func(irc *client.Conn, line *client.Line) {
		log.Println("Joined", line.Args[1])
	})
	irc.HandleFunc("INVITE", onInvite)
	irc.HandleFunc("PRIVMSG", onPrivmsg)

	quitting := false
	quit := make(chan bool)
	irc.HandleFunc(client.DISCONNECTED, func(irc *client.Conn, line *client.Line) {
		if quitting {
			quit <- true
			return
		}
		resetIdentifiedCache()
		log.Println("Disconnected; waiting 10 seconds then reconnecting...")
		saveCacheNow()
		go func() {
			time.Sleep(10 * time.Second)
			errorCount := 0
			for !irc.Connected() {
				log.Println("Reconnecting...")
				err := irc.ConnectTo(*server, *password)
				if err != nil {
					log.Println("Error reconnecting:", err)
					// limited exponential backoff (10, 12, 14, 18, 26, 42, 74)
					retryDuration := 10 + time.Duration(math.Pow(2, float64(errorCount)))*time.Second
					if errorCount < 6 {
						errorCount += 1
					}
					log.Println("Retrying in", retryDuration)
					time.Sleep(retryDuration)
				}
			}
		}()
	})
	if *useSSL {
		log.Println("Using SSL")
	}
	log.Println("Connecting to", *server)
	irc.ConnectTo(*server, *password)

	sig = make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill, syscall.SIGTERM)
	<-sig
	quitting = true
	log.Println("Disconnecting")
	irc.Quit("Exiting")
	saveCacheNow()
	<-quit // wait until the QUIT is sent to server
}

func loadCache() {
	if *cacheFile != "" {
		if fh, err := os.Open(*cacheFile); err != nil {
			log.Println("Error opening cache file:", err)
		} else {
			defer fh.Close()
			zr, _ := zlib.NewReader(fh)
			if err = lfm.Cache.Load(zr); err != nil {
				log.Println("Error reading cache file:", err)
			} else {
				log.Println("Loaded", lfm.Cache.ItemCount(), "cache entries")
			}
			zr.Close()
		}
	}
}

func saveCache() {
	if *cacheFile != "" {
		cacheTimer.Reset(10 * time.Second)
	}
}

func saveCacheNow() {
	if *cacheFile != "" {
		if fh, err := os.Create(*cacheFile); err != nil {
			log.Println("Error creating cache file:", err)
		} else {
			defer fh.Close()
			zw, _ := zlib.NewWriterLevel(fh, zlib.BestCompression)
			if err = lfm.Cache.Save(zw); err != nil {
				log.Println("Error storing cache:", err)
			} else {
				log.Println("Cache saved with", lfm.Cache.ItemCount(), "entries")
			}
			zw.Close()
		}
	}
}
