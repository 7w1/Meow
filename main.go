package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port              string `json:"port"`
	GenerateEndpoint  string `json:"generate_endpoint"`
	DetectEndpoint    string `json:"detect_endpoint"`
	AskEndpoint       string `json:"ask_endpoint"`
	EnableLandingPage bool   `json:"enable_landing_page"`
	MeowLikeEndpoint  string `json:"meowlike_endpoint"`
}

func loadConfig() Config {
	cfg := Config{
		Port:              "8000",
		GenerateEndpoint:  "/meow",
		DetectEndpoint:    "/ismeow",
		AskEndpoint:       "/askmeow",
		MeowLikeEndpoint:  "/meowlike",
		EnableLandingPage: true,
	}

	file, err := os.Open("config.json")
	if err == nil {
		defer file.Close()
		json.NewDecoder(file).Decode(&cfg)
		fmt.Println("Config loaded successfully 🌸ヽ(^。^)丿")
	} else {
		fmt.Println("Config file not found, using defaults nya~ (＞人＜；)")
	}

	return cfg
}

func main() {
	cfg := loadConfig()

	if cfg.EnableLandingPage {
		http.HandleFunc("/", rootHandler)
		fmt.Println("   -> Landing Page:   Enabled")
	} else {
		fmt.Println("   -> Landing Page:   Disabled")
	}

	http.HandleFunc(cfg.GenerateEndpoint, generateMeow)
	http.HandleFunc(cfg.DetectEndpoint, detectMeow)
	http.HandleFunc(cfg.AskEndpoint, askMeow)
	http.HandleFunc(cfg.MeowLikeEndpoint, detectMeowLike)
	http.HandleFunc("/languages", languagesHandler)

	fmt.Printf("MaaS is running on port %s (ฅ'ω'ฅ)\n", cfg.Port)
	fmt.Printf("    -> Generation: %s\n", cfg.GenerateEndpoint)
	fmt.Printf("    -> Detection: %s\n", cfg.DetectEndpoint)
	fmt.Printf("    -> 8-Ball:     %s\n", cfg.AskEndpoint)
	fmt.Printf("    -> Meow-Like:  %s\n", cfg.MeowLikeEndpoint)

	http.ListenAndServe(":"+cfg.Port, nil)
}

var askMeowAnswers = []string{
	"It is certain meow.",
	"It is decidedly meow.",
	"Without a meow.",
	"Yes - definitely meow.",
	"You may rely on meow.",
	"As I see it, meow.",
	"Most likely meow.",
	"Outlook good, meow.",
	"Yes, meow.",
	"Signs point to meow.",
	"Absolutely meow.",
	"All signs say meow.",
	"The whiskers agree: meow.",
	"Paws down, meow.",
	"Certified meow.",
	"Meow confirmed.",
	"The council of cats has spoken: meow.",
	"That's a meow from me.",
	"Meow beyond reasonable doubt.",
	"Verdict: meow.",
	"Probably meow.",
	"Looks meow to me.",
	"I'd bet a treat on meow.",
	"The tail twitch says meow.",
	"Meow-ish, in a good way.",
	"Favorable meow conditions.",
	"Meow with reservations, but still meow.",
	"Soft yes, loud meow.",
	"The sunbeam approves: meow.",
	"Meow pending, but likely.",
	"Reply hazy, try meowing again.",
	"Ask again later, meow.",
	"Better not tell you meow.",
	"Cannot predict meow.",
	"Concentrate and ask meow.",
	"The litter box is unclear.",
	"Meow lost in translation.",
	"Schrodinger's meow: both meow and not meow.",
	"Flip a cat coin and meow again.",
	"The cat walked across the keyboard. Ask later.",
	"Insufficient treats for a clear meow.",
	"Meow signal weak. Move closer to the food bowl.",
	"Nap first, meow later.",
	"The yarn ball says: unclear.",
	"Meow static on the line.",
	"Ask after the zoomies.",
	"Results may vary by cat mood.",
	"Meow is loading...",
	"The window stare continues. No answer yet.",
	"Don't count on meow.",
	"My reply is no meow.",
	"My sources say no meow.",
	"Outlook not so meow.",
	"Very doubtful meow.",
	"Hard no meow.",
	"Not meow. Not even a little.",
	"The claws say no meow.",
	"Meow denied.",
	"Negative meow.",
	"That's a hiss, not a meow.",
	"Meow vetoed.",
	"The empty bowl disagrees: not meow.",
	"Unlikely meow.",
	"Meow? In this economy?",
	"Probably not meow.",
	"Unlikely, but admire the audacity.",
	"Meow doubtful.",
	"The tail is low on this one.",
	"Not looking meow.",
	"Meow forecast: cloudy with a chance of no.",
	"Have you tried meowing at it?",
	"Meow is a state of mind.",
	"Only on Tuesdays that land on a meow.",
	"Ask the cat on the fridge.",
	"Meow once for yes, twice for also yes.",
	"The red dot knows, but won't tell.",
	"Meow is always the answer. This question is the exception.",
	"Purr harder.",
	"Meow responsibly.",
	"According to ancient cat law: meow.",
	"Meow detected, but at what cost?",
	"That's between you and the sunbeam.",
	"Meow in the streets, mrrp in the sheets.",
	"Error 404: meow not found.",
	"The cardboard box has no comment.",
	"Meow loudly and carry a small fish.",
	"Yes, but make it fashion. Meow.",
	"No, but have you considered a nap?",
	"Meow is not a number. Wait, yes it is.",
	"The cat distribution system has allocated: meow.",
	"Meow approved by 9 out of 10 cats. The tenth is asleep.",
	"Biologically speaking, meow.",
	"Philosophically, what is meow?",
	"Legally distinct from meow, but spiritually meow.",
	"Meow, but whisper it.",
	"MEOW.",
	"The prophecy foretold meow.",
	"Ask not whether it is meow, but whether you are worthy of meow.",
}

func askMeow(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	start := time.Now()
	values, queryErr := parseRequestQuery(r)
	if queryErr != nil {
		writeAPIError(w, queryErr)
		return
	}
	if _, supplied := values["lang"]; supplied {
		writeAPIJSON(w, http.StatusBadRequest, map[string]string{"error": "lang is not supported on /askmeow"})
		return
	}
	text, apiErr := getRequestText(r, "a question", "is it time for food")
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(text))))
	answer := askMeowAnswers[h.Sum32()%uint32(len(askMeowAnswers))]
	writeAPIJSON(w, http.StatusOK, map[string]interface{}{
		"question": text, "answer": answer,
		"latency_time": time.Since(start).String(),
	})
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>MaaS API</title>
    <style>
        body { font-family: monospace; background: #111; color: #eee; padding: 2rem; max-width: 600px; margin: auto; }
        a { color: #ffd1dc; text-decoration: none; }
        a:hover { text-decoration: underline; }
        ul { list-style-type: none; padding-left: 0; }
        li { margin-bottom: 1rem; background: #222; padding: 1rem; border-radius: 4px; }
    </style>
</head>
<body>
    <p>meow.</p>
    <p>contribute: <a href="https://github.com/7w1/Meow">github.com/7w1/Meow</a></p>
    <br>
    <h3>Endpoints</h3>
	<p>There is a per-ip rate limit of 100 requests per 10 seconds.</p>
    <ul>
        <li>
            <code>GET <a href="/meow">/meow</a></code><br><br>
            Generates a vocalization. Add a supported <code>?lang=...</code> tag to choose its language.
        </li>
        <li>
            <code>GET <a href="/ismeow?text=mrrp">/ismeow?text={input}</a></code><br><br>
            Strictly checks the complete input in the selected language (default: English). Example: <code>/ismeow?text=にゃー&amp;lang=ja</code>.
        </li>
		<li>
            <code>GET <a href="/meowlike?text=miao">/meowlike?text={input}</a></code><br><br>
            Checks the complete input and substantial word-level vocalizations. An embedded result reports Unicode-letter coverage; this score is not a probability.
        </li>
        <li>
            <code>GET <a href="/askmeow?text=hello">/askmeow?text={question}</a></code><br><br>
            Returns a deterministic English response.
        </li>
    </ul>
    <p>Supported meow languages: <code>en</code>, <code>fr</code>, <code>ja</code>, <code>tr</code>, <code>ru</code>, <code>uk</code>, <code>zh-Hans</code>, <code>zh-Hant</code>, <code>es</code>, <code>nl</code>, <code>de</code>, <code>hy</code>, <code>eo</code>, <code>ar</code>, <code>hi</code>. Detection accepts <code>lang=auto</code>. Use <a href="/languages">/languages</a> for the current list.</p>
    <br>
    <p>Made by 7w1</p>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
