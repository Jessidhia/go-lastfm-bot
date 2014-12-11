package main

import (
	"flag"
	"fmt"
	"github.com/fluffle/goirc/client"
	"log"
	"strings"
)

func addWhoHandlers(irc *client.Conn) {
	if !flag.Parsed() {
		flag.Parse()
	}

	irc.HandleFunc("352", whoHandler)
	irc.HandleFunc("315", whoHandler)
	return
}

// >> :irc.cccp-project.net 352 Ziltoid #qqkthx Southrop h.i.n.a * Southrop Gr+ :0 Southrop
// >> :irc.cccp-project.net 352 Ziltoid #qqkthx kovensky Rizon-180E162B.bluebottle.net.au * Kov|abx G :0 Diogo Franco
// >> :irc.cccp-project.net 315 Ziltoid #qqkthx :End of /WHO list.

var whoHandlerLimit = make(chan bool, 1)

func whoHandler(irc *client.Conn, line *client.Line) {
	whoHandlerLimit <- true

	switch line.Cmd {
	case "352":
		if _, ok := whoResult[line.Args[1]]; !ok {
			log.Println("Creating result slice for channel", line.Args[1])
			whoResult[line.Args[1]] = []string{}
		}
		whoResult[line.Args[1]] = append(whoResult[line.Args[1]], line.Args[5])
	case "315":
		log.Println("End of WHO for channel", line.Args[1])
		close(whoChannel[line.Args[1]])
	}
	<-whoHandlerLimit
	return
}

var (
	whoChannel = make(map[string]chan bool)
	whoResult  = make(map[string][]string)
	rateLimit  = make(chan bool, 6)
)

func reportAllNowPlaying(irc *client.Conn, asker, channel string) {
	if !(strings.HasPrefix(channel, "#") || strings.HasPrefix(channel, "&")) {
		log.Println("User", asker, "asked What's Playing...... via PM")
		irc.Privmsg(channel, fmt.Sprintf("%s: this only works on channels", asker))
		return
	}
	log.Println("User", asker, "requested What's Playing on channel", channel)

	if !checkIdentified(irc, asker) {
		r := fmt.Sprintf("%s: you must be identified with NickServ to use this command", asker)
		log.Println(r)
		irc.Privmsg(channel, r)
		return
	}

	if _, ok := whoChannel[channel]; ok {
		log.Println("Channel", channel, "is already executing a What's Playing request")
		return
	}

	whoChannel[channel] = make(chan bool, 1)

	go irc.Who(channel)
	for _ = range whoChannel[channel] { // wait until channel is closed
	}
	delete(whoChannel, channel)

	reportChan := make(chan bool)
	totalReport := len(whoResult[channel]) - 1
	msg := fmt.Sprintf("Reporting now playing for %d nicks in channel %s", totalReport, channel)
	log.Println(msg)
	irc.Notice(asker, msg)

	for _, nick := range whoResult[channel] {
		if nick != irc.Me().Nick {
			n := nick
			go func() {
				rateLimit <- true
				reportChan <- reportNowPlaying(irc, channel, asker, n, true)
				<-rateLimit
			}()
		}
	}
	delete(whoResult, channel)

	okCount, totalCount := 0, 0
	for r := range reportChan {
		if r {
			okCount++
		}
		if totalCount++; totalCount == totalReport {
			break
		}
	}
	close(reportChan)

	msg = fmt.Sprintf("Reported for %d of %d nicks", okCount, totalCount)
	log.Println(msg)
	irc.Notice(asker, msg)

	return
}
