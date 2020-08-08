package main

import (
	"CrackWatchDiscordBot/crackwatch"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	discordBotToken = flag.String("token", "", "Discord bot token.")
	botCommand      = flag.String("command", "!crack", "Message prefix to"+
		" activate the bot. Can't contain spaces.")
)

type guildID = string

var discordMsgLock = map[guildID]*sync.Mutex{}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if err := parseFlags(); err != nil {
		log.Fatalln("Error while parsing flags: " + err.Error())
	}

	discordBot, err := newDiscordBot(*discordBotToken)
	if err != nil {
		log.Fatalln("Unable to create Discord bot: " + err.Error())
	}
	defer discordBot.Close()

	discordBot.AddHandler(onMessageReceived)

	waitForSignal()
}

func parseFlags() error {
	flag.Parse()

	if *discordBotToken == "" {
		return errors.New("A Discord bot token is required in order for this" +
			" application to function.")
	}

	for _, run := range *botCommand {
		if unicode.IsSpace(run) {
			return errors.New("botCommand has a space in it, which will break" +
				" parsing!")
		}
	}

	return nil
}

func newDiscordBot(token string) (*discordgo.Session, error) {
	discordSession, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, errors.New("Unable to create the Discord session: " +
			err.Error())
	}

	if err = discordSession.Open(); err != nil {
		return nil, errors.New("Unable to open the Discord session: " +
			err.Error())
	}

	return discordSession, nil
}

func onMessageReceived(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore own messages.
	if m.Author.ID == s.State.User.ID {
		return
	}

	messageFields := strings.Fields(strings.ToLower(m.Content))
	if len(messageFields) < 2 ||
		!strings.HasPrefix(messageFields[0], *botCommand) {
		return
	}

	page := 0
	if len(messageFields[0]) > len(*botCommand) {
		pageStr := messageFields[0][len(*botCommand):]
		pageInt, err := strconv.Atoi(pageStr)
		if err != nil {
			sendDiscordMessage(s, m, `Unable to parse a page number from "`+
				pageStr+`"`)
			return
		}

		page = pageInt - 1
	}

	searchTerm := strings.Join(messageFields[1:], " ")
	searchResults, err := crackwatch.Search(searchTerm, page)
	if err != nil {
		sendDiscordMessage(s, m, "A problem occurred : "+err.Error())
		return
	} else if len(searchResults.Games) == 0 {
		sendDiscordMessage(s, m, "No games found which matched your query!")
		return
	}

	for _, messageChunk := range resultsToDiscordChunks(searchResults, page+1) {
		sendDiscordMessage(s, m, messageChunk)
	}
}

func resultsToDiscordChunks(
	searchResults crackwatch.SearchResults, pageNum int,
) []string {
	var strBuilder strings.Builder

	for i, game := range searchResults.Games {
		// Hasn't been cracked yet
		if game.CrackDate.IsZero() {
			nFollowersStr := message.NewPrinter(language.English).
				Sprintf("%d", game.NumFollowers)

			peopleStr := "people"
			if game.NumFollowers == 1 {
				peopleStr = "person"
			}

			strBuilder.WriteString(
				fmt.Sprintf("🛑%q has %s %s waiting for a crack!",
					game.Name, nFollowersStr, peopleStr),
			)

			if i != len(searchResults.Games)-1 {
				strBuilder.WriteString("\n")
			}

			continue
		}

		strBuilder.WriteString(
			fmt.Sprintf("🟢%s | %s | %s | %s | %s",
				game.Name, game.ReleaseDate.Format("2006-01-02"),
				crackwatch.NormalizeDRMNames(game.DRM),
				strings.Join(game.CrackedBy, "+"),
				game.CrackDate.Format("2006-01-02")),
		)

		if i != len(searchResults.Games)-1 {
			strBuilder.WriteString("\n")
		}
	}

	const (
		header = "```Game Name | Release Date | DRM | Cracked By | Date" +
			" Cracked\n"
		discordMaxMsgLen = 2000
	)

	var (
		footer = fmt.Sprintf("\nPage %d/%.0f```", pageNum,
			math.Ceil(float64(searchResults.Num)/float64(30)))
		maxMsgLen     = discordMaxMsgLen - len(header+footer)
		msg           = strBuilder.String()
		messageChunks = []string{}
	)
	for len(msg) > maxMsgLen {
		idxLastNewline := strings.LastIndex(msg[:maxMsgLen], "\n")
		messageChunks = append(messageChunks,
			header+msg[:idxLastNewline]+footer)

		// +1 to skip the newline character.
		msg = msg[idxLastNewline+1:]
	}

	messageChunks = append(messageChunks, header+msg+footer)

	return messageChunks
}

func sendDiscordMessage(
	s *discordgo.Session, m *discordgo.MessageCreate, msg string,
) {
	mutex, ok := discordMsgLock[m.GuildID]
	if !ok {
		discordMsgLock[m.GuildID] = &sync.Mutex{}
		mutex = discordMsgLock[m.GuildID]
	}

	mutex.Lock()

	// We purposely ignore all errors when sending messages to Discord, since in
	//  the rare worst-case scenario, the user just has to send another query.
	_, _ = s.ChannelMessageSend(m.ChannelID, msg)
	time.Sleep(time.Second)

	mutex.Unlock()
}

func waitForSignal() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	log.Printf("CrackWatchDiscordBot has shut down due to the %q signal being"+
		" caught.", <-signalChan)
}
