# Last.fm IRC bot

Runs a Last.fm IRC bot that replies to simple query commands.

# Install

Run `go get github.com/Kovensky/go-lastfm-bot`.

# IRC Commands

Assuming the default `-cmd-prefix` of `"."`:

* `.help`: Sends this help to the user through NOTICEs.
* `.np ($user)?`: Shows your now playing song. If you give `$user`, queries for that `$user`.
* `.compare ($user1) ($user2)?`: Runs a tasteometer compare between you and `$user1`, or between `$user1` and `$user2` if present.
* `.top5 ((overall|year|month|week) ($user)?)?`: Shows the top5 artists in the chosen period for you or the `$user`.
* `.whois ($nick)?`: Shows your associated last.fm username, or the username associated with `$nick`.
* `.aka ($username)`: Shows the nicks that have been associated with `$username`.

A nick can be used in place of a username if it's associated with a last.fm account.

If `-require-auth` is enabled (default), the following commands require that the user
be authenticated to nickserv:

* `.ignore`: Makes the bot ignore you for most commands. Use `.setuser` or `.deluser` to be unignored.
* `.setuser ($username)`: Associates your nick with the given last.fm `$username`.
* `.deluser`: Removes your nick's association, if any.

This command is shown in the .help output to help avoid abuse by random people:
* `.wp`: Shows what's playing for everyone in the channel, requires authentication.

# Command-Line Options

* `-api-key=""`: The Last.fm API key. Required.
* `-server=""`: The IRC server to connect to. Required. If a `:` is present, uses the right side as the port.
* `-ssl=false`: Whether to use explicit SSL. Changes the default port to 6697.
* `-password=""`: The password needed to connect to the server, if any.
* `-channels=""`: Comma-separated list of channels to join on the server.
* `-nick="Lastfm_bot"`: The nickname the bot should use.
* `-nickserv-password=""`: A NickServ password to authenticate the bot, if any. Tested on Freenode and SynIRC.

* `-cmd-prefix="."`: The prefix to user commands.

* `-save-nicks=true`: Whether to persist the user-nick mappings
* `-nick-file=""`: JSON file where user-nick map is stored. If blank, `{{server}}.nicks.json` is used.
* `-require-auth=true`: Requires that nicknames be authenticated for using the user/nick mapping. Disable on networks that don't implement a NickServ, such as EFNet.

If a `-nickserv-password` is present, the bot will also try to GHOST to acquire the nick if it
isn't available, and will try to join the channels again after authenticating, in case there
are any channels that require authenticated users.
