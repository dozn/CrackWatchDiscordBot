## FIXME: This no longer works since crackwatch.com has implemented Cloudflare's DDos Protection.
## I'd be more than happy to merge a pull request which fixes this, but I don't care enough to do it myself.
## Feel free to take a look at some of the following repos for pointers.
## https://github.com/VeNoMouS/cloudscraper
## https://github.com/Anorov/cloudflare-scrape
##			https://github.com/devgianlu/cloudflare-bypass
****
****
****

![Example of what the results appear like in Discord](/images/results.png)

# Description
A bot which uses the websocket connection at [crackwatch.com](https://crackwatch.com) to query results.\
Should work with no issues on any platform supported by Go.

# Why?
Because
[the other guys who were making one](https://old.reddit.com/r/CrackWatch/comments/i34eel/discord_bot_prototype)
thought it would be a good idea to\
keep the source code closed, even though this is a trivial application.\
In case anyone isn't aware, these bots are able to log any and all messages on\
the channels they have access to, so it's imperative you run them yourselves.\
It irritated me enough that I wrote this in a few hours, even though I have\
no use for it.\
Feel free to fork the repo and change it for your own usecase, just follow the\
simple terms of the
[ISC](https://en.wikipedia.org/wiki/ISC_license)
license by making sure you keep an exact copy of the\
LICENSE file in your repo.

# Usage
In Discord, enter in the bot command (`!crack` by default) followed by your\
search term: `!crack test`\
If there's multiple pages, enter the page number you'd like right after the bot\
command: `!crack2 test`

# Run From Source
`go run . -token=<DiscordBotToken>`

# Build and Run
`go build && ./CrackWatchDiscordBot -token=<DiscordBotToken>`

# I _REALLY_ Want to Try Before I Compile
Please be aware that this bot (or anyone else's) should NOT be trusted, as they\
**all** have the ability to log any and all messages on the channels they have\
access to. This is here only to demonstrate the functionality before running it\
yourself.

NOTE: The following test bot is no longer running since crackwatch.com implemented Cloudflare's DDoS Protection.
~~https://discord.com/oauth2/authorize?client_id=741678700033998888&permissions=2048&scope=bot~~

# Getting Your Very Own Bot Token
1) Create a new application at https://discord.com/developers/applications
2) From the menu on the left, click on "Bot"
3) Click on "Add Bot"
4) Click on "Click to Reveal Token"

# Creating the Bot Invite Link
1) Select the bot from https://discord.com/developers/applications
2) Go to the following URL after replacing "CLIENT_ID" with your Client ID:\
https://discord.com/oauth2/authorize?client_id=CLIENT_ID&permissions=2048&scope=bot

# Possible Improvements
- I can't fathom many people using this application, so it doesn't reuse a\
single websocket connection, and instead opens a new connection each query. If\
you have an extremely high-traffic usecase for this, please ensure you respect\
[crackwatch.com](https://crackwatch.com) by reusing a single connection. It may be easier to use [nhooyr's\
library](https://github.com/nhooyr/websocket) if this is wanted, not too sure.
- I was contemplating whether or not I wanted to use an embedded message when\
there was only one result, but ultimately was against it since it would have\
taken up a bunch of vertical messaging space where it wasn't necessary. Feel\
free to fork it and add it to your own version, however.
- If you're planning on supporting multiple guilds, you'll want to add a\
command (as well as a data store) so users from guilds can change the bot\
command prefix. I didn't do this for my own version because I don't want people\
using it—I want them running their own.

# Design Decisions
Q: Why don't you use a tabwriter to align the columns for each result?\
A: 1) Due to the nature of the need behind the search, your eyes wouldn't be\
scanning to compare columns of different rows—you'd instead be looking for a\
game's name (which is at the beginning of each row, sorted alphabetically),\
then scanning to the right from there for more information about that game.\
2) It would use up a lot more of Discord's allotted characters (2000 per\
message), allowing _far_ less information to be presented per message.
