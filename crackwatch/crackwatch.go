package crackwatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const errUnhandledResponse = "We received a response from crackwatch.com we" +
	" weren't expecting, and couldn't handle. Sorry about that—perhaps try it" +
	" directly from <https://crackwatch.com/games> until we fix this issue?"

var errCouldNotReachSite = errors.New("crackwatch.com could not be reached.")

const maxSearchTermLength = 100

type searchOptions struct {
	crackStatus   crackStatus
	releaseStatus releaseStatus
	studioType    studioType
	orderType     orderType
	sortOrder     sortOrder
}

// Search query parameters.
type crackStatus string

const (
	CrackStatusAll       crackStatus = "0"
	CrackStatusCracked   crackStatus = "1"
	CrackStatusUncracked crackStatus = "2"
)

type releaseStatus string

const (
	ReleaseStatusAll        releaseStatus = "0"
	ReleaseStatusReleased   releaseStatus = "1"
	ReleaseStatusUnreleased releaseStatus = "2"
)

type studioType string

const (
	StudioAll   studioType = "0"
	StudioAAA   studioType = "1"
	StudioIndie studioType = "2"
)

type orderType string

const (
	OrderTypeTitle       orderType = "title"
	OrderTypeReleaseDate orderType = "releaseDate"
	OrderTypeCrackDate   orderType = "crackDate"
	OrderTypeDRM         orderType = "protection"
	OrderTypeGroup       orderType = "group"
	OrderTypeNumNFOs     orderType = "nfo"
	OrderTypePrice       orderType = "price"
	OrderTypeRatings     orderType = "ratings"
	OrderTypeComments    orderType = "comments"
	OrderTypeFollowers   orderType = "followers"
)

type sortOrder string

const (
	SortOrderDesc sortOrder = "true"
	SortOrderAsc  sortOrder = "false"
)

type SearchResults struct {
	Num   int `json:"gameCount"`
	Games []struct {
		Name         string `json:"title"`
		ReleaseDate  Time
		DRM          []string `json:"protections"`
		CrackedBy    []string `json:"groups"`
		CrackDate    Time
		NumFollowers int `json:"followersCount"`
	}
}

type Time struct {
	time.Time
}

// One of the games gives us just the year, and another uses a date which
//  doesn't actually exist (Feb. 30th, 2015...). We return the default time
//  (0001-01-01 00:00:00 +0000 UTC) in case of an error.
func (t *Time) UnmarshalJSON(b []byte) error {
	timeStr := strings.ReplaceAll(string(b), `"`, "")

	if timeStr == "null" || len(timeStr) < 10 {
		t.Time = time.Time{}
		return nil
	}

	// We only care about the date, not the time.
	parsedTime, err := time.Parse("2006-01-02", timeStr[:10])
	if err != nil {
		t.Time = time.Time{}
		return nil
	}

	t.Time = parsedTime
	return nil
}

// The error returned from this function is meant for users.
func Search(term string, page int) (SearchResults, error) {
	if len(term) > maxSearchTermLength {
		return SearchResults{}, fmt.Errorf("Search term was >%d characters.\n",
			maxSearchTermLength)
	}

	ws, err := connectToWebsocket()
	if err != nil {
		return SearchResults{}, err
	}
	defer ws.Close()

	err = sendSearchQuery(ws, term, strconv.Itoa(page), &searchOptions{
		crackStatus:   CrackStatusAll,
		releaseStatus: ReleaseStatusAll,
		studioType:    StudioAll,
		orderType:     OrderTypeTitle,
		sortOrder:     SortOrderAsc,
	})
	if err != nil {
		return SearchResults{}, err
	}

	searchResults, err := waitForSearchResults(ws)
	if err != nil {
		return SearchResults{}, err
	}

	return searchResults, nil
}

func connectToWebsocket() (*websocket.Conn, error) {
	// The format of the first two segments after "sockjs" are _very_ flexible.
	ws, _, err := websocket.DefaultDialer.Dial(
		"wss://crackwatch.com/sockjs/crackwatch/discord_bot/websocket", nil)
	if err != nil {
		log.Println("Unable to dial the websocket: " + err.Error())
		return nil, errors.New("crackwatch.com did not accept our websocket" +
			" connection.")
	}

	err = ws.WriteMessage(websocket.TextMessage,
		[]byte(`["{\"msg\":\"connect\",\"version\":\"1\",\"support\":[\"1\",\"`+
			`pre2\",\"pre1\"]}"]`))
	if err != nil {
		log.Println("Unable to write connect message: " + err.Error())
		return nil, errors.New("crackwatch.com did not accept our websocket" +
			" connection.")
	}

	return ws, nil
}

// NOTE: I _really_ want to give each of these parameters separate types so they
//  can't be confused. _Really_ really.
func sendSearchQuery(
	ws *websocket.Conn, term, page string, options *searchOptions,
) error {
	// Ivan \\\"Ironman Stewart's\\\" Super Off-Road
	term = strings.ReplaceAll(term, `"`, `\\\"`)
	term = strings.TrimSpace(term)

	err := ws.WriteMessage(
		websocket.TextMessage,
		[]byte(`["{\"msg\":\"method\",\"method\":\"games.page\",\"params\":[{\`+
			`"page\":`+page+`,\"orderType\":\"`+string(options.orderType)+`\",`+
			`\"orderDown\":`+string(options.sortOrder)+`,\"search\":\"`+term+
			`\",\"unset\":0,\"released\":`+string(options.releaseStatus)+`,\"c`+
			`racked\":`+string(options.crackStatus)+`,\"isAAA\":`+
			string(options.studioType)+`}],\"id\":\"1\"}"]`))
	if err != nil {
		log.Printf("Unable to write search message to websocket with the"+
			" search term %q: %s", term, err)
		return errCouldNotReachSite
	}

	return nil
}

func waitForSearchResults(ws *websocket.Conn) (SearchResults, error) {
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Unable to read message from server: %s", err)
			return SearchResults{}, errCouldNotReachSite
		}

		if string(message) == `a["{\"msg\":\"error\",\"reason\":\"Bad request\`+
			`"}"]` {
			log.Printf(`Received a "Bad request" response.`)
			return SearchResults{}, errors.New(`Received a "Bad request"` +
				" response from crackwatch.com.")
		}

		if string(message[:22]) != `a["{\"msg\":\"result\"` {
			continue
		}

		// Remove the unnecessary wrapping array from the response.
		escapedResponse := strings.TrimPrefix(string(message), `a["`)
		escapedResponse = strings.TrimSuffix(escapedResponse, `"]`)
		// Correct escaped quotation marks and slashes.
		escapedResponse = strings.ReplaceAll(escapedResponse, `\"`, `"`)
		escapedResponse = strings.ReplaceAll(escapedResponse, `\\`, `\`)

		// We unmarshal to a map first, as we want to unmarshal an inner struct
		//  into our own custom struct.
		var responseMap map[string]json.RawMessage
		err = json.Unmarshal([]byte(escapedResponse), &responseMap)
		if err != nil {
			log.Printf("Unable to unmarshal response into map: %s: %s\n", err,
				escapedResponse)
			return SearchResults{}, errors.New(errUnhandledResponse)
		}

		results := SearchResults{}
		err = json.Unmarshal(responseMap["result"], &results)
		if err != nil {
			log.Printf("Unable to unmarshal response: %s: %s\n", err,
				escapedResponse)
			return SearchResults{}, errors.New(errUnhandledResponse)
		}

		return results, nil
	}
}

const (
	DRMUnknown   = "Unknown"
	DRMDiscCheck = "Disc Check"
	DRMNone      = "None"
	DRMConsole   = "Console"
)

var DRMNameMapping = map[string]string{
	"":                    DRMUnknown,
	"-":                   DRMUnknown,
	"activation":          DRMUnknown,
	"activision":          DRMUnknown,
	"amazon":              "Amazon",
	"andmicrosoftwindows": DRMUnknown,
	"arcade":              DRMUnknown,
	"arcsystemworks":      DRMUnknown,
	"armadillo":           "Armadillo",
	"arxan":               "Arxan",
	"ascgames":            DRMUnknown,
	"atarisa":             DRMUnknown,
	"battleeye":           DRMUnknown,
	"battle.net":          "Battle.net",
	"battlenet":           "Battle.net",
	"battlenet-arxan":     "Battle.net/Arxan",
	"bethesda":            DRMUnknown,
	"bigfish":             DRMUnknown,
	"blitsgames":          DRMUnknown,
	"catalyst":            "Catalyst",
	"cdautokey":           DRMDiscCheck,
	"cd-check":            DRMDiscCheck,
	"cdcheck":             DRMDiscCheck,
	"cdcheck/re-index":    DRMDiscCheck,
	"cd-checks":           DRMDiscCheck,
	"cdchecks":            DRMDiscCheck,
	"cd-cops":             "CD-Cops",
	"cddilla":             "C-Dilla",
	"cdilla":              "C-Dilla",
	"cd-key":              "Serial",
	"cd rom":              DRMDiscCheck,
	"cd-rom":              DRMDiscCheck,
	"codecheck":           "Code Check",
	"codewheel":           "Code Wheel",
	"colorcodes":          "Color Codes",
	"copylock":            "CopyLok",
	"copylok":             "CopyLok",
	"coredesign":          DRMUnknown,
	"denuvo":              "Denuvo",
	"denuvo+origin":       "Denuvo/Origin",
	"denuvo+uplay":        "Denuvo/Uplay",
	"denuvo+vmpotect":     "Denuvo/VMProtect",
	"deutschland-spielt":  DRMUnknown,
	"disc check":          DRMDiscCheck,
	"disccheck":           DRMDiscCheck,
	// Might be referring to looking inside of the manual to pass the DRM?
	"doccheck":                DRMUnknown,
	"dos":                     DRMUnknown,
	"dreamcast":               DRMConsole,
	"dreamforgeintertainment": DRMUnknown,
	"drm":                     DRMUnknown,
	"drm free":                DRMNone,
	"drm-free":                DRMNone,
	"drmfree":                 DRMNone,
	"drmfreegog":              DRMNone,
	// I have no idea if they're talking about DVD CSS or something else.
	"dvd drm":          DRMUnknown,
	"dvddrm":           DRMUnknown,
	"dvd-rom":          DRMUnknown,
	"eac":              DRMUnknown,
	"eappx":            "EAppX",
	"eidosinteractive": DRMUnknown,
	"electronicarts":   DRMUnknown,
	"e-license":        "eLicense",
	"epic":             "Epic Games",
	"epicgames":        "Epic Games",
	"false":            DRMUnknown,
	"fileintegrity":    "File Integrity",
	// Simply because a game is free, does not mean it's free of DRM.
	"free":         DRMUnknown,
	"free2play":    DRMUnknown,
	"free-to-play": DRMUnknown,
	// NOTE: This may change in the future, see:
	//  https://gamejolt.com/f/is-it-possible-to-add-drm-to-game-files-and-sell-steam-keys/344840?sort=top
	"gamejolt":          DRMNone,
	"games for windows": "Games for Windows Live",
	"gameshield":        "GameShield",
	"gog":               DRMNone,
	"gog.com":           DRMNone,
	"gog/steam":         DRMNone,
	// "Die drei ??? Kids - Jagd auf das Phantom" uses this, apparently :D
	"icantfindthisgameonanygamestoreplatform": DRMUnknown,
	// I can't find anything about "IGC-DVD", even though quite a few
	//  entries seem to return it.
	"igc-dvd":             DRMUnknown,
	"interactivision a/s": DRMUnknown,
	"ios/android":         "Mobile",
	"ironwrap":            "GameShield",
	"jowood":              DRMUnknown,
	"konami":              DRMUnknown,
	"laserlock":           "LaserLock",
	"magnussoft":          DRMUnknown,
	"microids":            DRMUnknown,
	"microsoft":           DRMUnknown,
	"microsoftslps":       "Microsoft SLPS",
	"microsoftstore":      "Microsoft Store",
	"microsoftwindows":    DRMUnknown,
	"mmo":                 DRMUnknown,
	"moby":                DRMUnknown,
	"ms-dos":              DRMUnknown,
	"myswooop":            DRMUnknown,
	"n/a":                 DRMUnknown,
	"nes":                 DRMConsole,
	"nintendo":            DRMConsole,
	"nintendo exclusive":  DRMConsole,
	"nintendoswitch":      DRMConsole,
	"no-drm":              DRMNone,
	"nodrm":               DRMNone,
	"none":                DRMNone,
	"nothing":             DRMNone,
	"notspecified":        DRMUnknown,
	"novalogic":           DRMUnknown,
	// Oculus ditched their DRM, so who knows what these games use now.
	"oculus":               DRMUnknown,
	"origin":               "Origin",
	"patreon":              DRMUnknown,
	"pc":                   DRMUnknown,
	"pc-dos":               DRMUnknown,
	"pc-spiel":             DRMUnknown,
	"play+smile":           DRMUnknown,
	"playstation3/xbox360": DRMConsole,
	"playstation/ios":      DRMConsole,
	"popcap":               DRMUnknown,
	"protectcd":            "ProtectDISC CD",
	"protectcd8":           "ProtectDISC CD",
	"protectdvd":           "ProtectDISC DVD",
	// Can't find any information about this DRM.
	"reroute":       DRMUnknown,
	"re-route/size": DRMUnknown,
	"retail":        DRMUnknown,
	// "20000 Meilen unter dem Meer" uses this, although it doesn't actually
	//  appear to be a DRM scheme.
	"ring":     DRMUnknown,
	"rockstar": "Rockstar Social Club",
	"safedisc": "SafeDisc",
	// They had 4 versions, although I haven't come across any other games
	//  which state the version yet.
	"safedisc2":     "SafeDisc v2",
	"safedisc4":     "SafeDisc v4",
	"safedisk":      "SafeDisc",
	"securom":       "SecuROM",
	"serial":        "Serial",
	"serialnumber":  "Serial",
	"solidshield":   "Solidshield",
	"stadia":        "Google Stadia",
	"starforce":     "StarForce",
	"steam":         "Steam",
	"steam/arc":     "Steam",
	"steam/free":    "Steam",
	"steam/origin":  "Steam/Origin",
	"steam+uplay":   "Steam/Uplay",
	"tlgames":       DRMUnknown,
	"tages":         "Tagès",
	"tbd":           DRMUnknown,
	"themida":       "Themida",
	"ubisoft":       DRMUnknown,
	"ump":           DRMUnknown,
	"unknown":       DRMUnknown,
	"uplay":         "Uplay",
	"uplay/denuvo":  "Uplay/Denuvo",
	"uwp":           "UWP",
	"uwp-arxan":     "UWP/Arxan",
	"uwp/steam":     "UWP/Steam",
	"valeroa":       "Valeroa",
	"vista":         DRMUnknown,
	"vmprotect":     "VMProtect",
	"vob/protectcd": "ProtectDISC CD",
	"wildgames":     DRMUnknown,
	"wildtangent":   "WildTangent",
	"windows":       DRMUnknown,
	"xbox":          DRMConsole,
	"xboxlive":      DRMConsole,
	"ysiphus":       DRMUnknown,
	"zagravagames":  DRMUnknown,
}

// The DRM names are user-submitted with the worst capitalization and spelling
//  you could imagine, if they're even correct in the first place! This attempts
//  to normalize them somewhat, but we don't care about nailing every edge case
//  since it would be impossible.
func NormalizeDRMNames(names []string) string {
	if len(names) == 0 {
		return DRMUnknown
	}

	// Translate the DRM names using the table above.
	properDRMs := []string{}
	for _, name := range names {
		name = strings.ToLower(name)
		value, ok := DRMNameMapping[name]
		if !ok {
			log.Printf("First time coming across the DRM name %q.\n", name)
		}

		if value != DRMUnknown {
			properDRMs = append(properDRMs, value)
		}
	}

	// If there's no DRM entries left, then the DRM scheme is unknown.
	if len(properDRMs) == 0 {
		return DRMUnknown
	}

	return strings.Join(properDRMs, "+")
}
