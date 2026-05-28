package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
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

	fmt.Printf("MaaS is running on port %s (ฅ'ω'ฅ)\n", cfg.Port)
	fmt.Printf("    -> Generation: %s\n", cfg.GenerateEndpoint)
	fmt.Printf("    -> Detection: %s\n", cfg.DetectEndpoint)
	fmt.Printf("    -> 8-Ball:     %s\n", cfg.AskEndpoint)
	fmt.Printf("    -> Meow-Like:  %s\n", cfg.MeowLikeEndpoint)

	http.ListenAndServe(":"+cfg.Port, nil)
}

func generateMeow(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	heads := []string{"m", "n", "p", "mr", "pr", "ny"}
	cores := []string{"e", "eo", "a", "ow", "aw", "u", "ya", "io", "ew"}
	tails := []string{"w", "p", "m", "rp", ""}

	head := heads[rand.Intn(len(heads))]
	core := cores[rand.Intn(len(cores))]
	tail := tails[rand.Intn(len(tails))]

	stretch := func(s string, chance float32, maxStretch int) string {
		if rand.Float32() < chance && len(s) > 0 {
			charToStretch := string(s[len(s)-1])
			repeats := rand.Intn(maxStretch) + 1
			s += strings.Repeat(charToStretch, repeats)
		}
		return s
	}

	head = stretch(head, 0.4, 4)
	core = stretch(core, 0.8, 6)
	tail = stretch(tail, 0.5, 4)

	finalMeow := head + core + tail

	duration := time.Since(start)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"meow":            finalMeow,
		"generation_time": duration.String(),
	})
}

func squeezeLetters(text string) string {
	var builder strings.Builder
	var lastChar rune
	for _, char := range strings.ToLower(text) {
		if char >= 'a' && char <= 'z' {
			if char != lastChar {
				builder.WriteRune(char)
				lastChar = char
			}
		}
	}
	return builder.String()
}

func countLetters(text string) int {
	n := 0
	for _, char := range strings.ToLower(text) {
		if char >= 'a' && char <= 'z' {
			n++
		}
	}
	return n
}

var meowLikeTransitions = map[string]bool{
	"me": true, "eo": true, "ow": true, // meow
	"mi": true, "ia": true, "au": true, "ao": true, // miau / miao
	"ny": true, "ya": true, "an": true, // nya / nyan
	"mr": true, "rp": true, // mrrp
	"pr": true, "ur": true, // purr
	"ew": true, "aw": true, // trailing sounds
}

var meowLikeTokens = []string{
	"miao", "meow", "mrrp", "mrp", "mrow", "mrrow", "nya", "nyan",
	"purr", "prrr", "mew", "rawr",
}

var askMeowAnswers = []string{
	// Strong yes
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
	// Leaning yes
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
	// Uncertain / try again
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
	// Strong no
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
	// Leaning no
	"Probably not meow.",
	"Unlikely, but admire the audacity.",
	"Meow doubtful.",
	"The tail is low on this one.",
	"Not looking meow.",
	"Meow forecast: cloudy with a chance of no.",
	// Playful / thematic
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

const meowLikeThreshold = 40.0

func meowTokenBoost(squeezed string) float64 {
	for _, tok := range meowLikeTokens {
		if strings.Contains(squeezed, tok) {
			return 25.0
		}
	}
	return 0.0
}

func scoreMeowLike(squeezed string, letterCount int) float64 {
	if len(squeezed) == 0 {
		return 0
	}
	if len(squeezed) == 1 {
		if strings.ContainsAny(squeezed, "mny") {
			return 30
		}
		return 0
	}

	totalPairs := len(squeezed) - 1
	matchCount := 0
	for i := 0; i < totalPairs; i++ {
		if meowLikeTransitions[squeezed[i:i+2]] {
			matchCount++
		}
	}

	density := float64(matchCount) / float64(totalPairs)

	const softSqueezedLen = 14
	const hardSqueezedLen = 28
	lengthFactor := 1.0
	if len(squeezed) > softSqueezedLen {
		lengthFactor = 1.0 - float64(len(squeezed)-softSqueezedLen)/float64(hardSqueezedLen-softSqueezedLen)
		if lengthFactor < 0 {
			lengthFactor = 0
		}
	}

	const softLetterCount = 20
	const hardLetterCount = 55
	inputFactor := 1.0
	if letterCount > softLetterCount {
		inputFactor = 1.0 - float64(letterCount-softLetterCount)/float64(hardLetterCount-softLetterCount)
		if inputFactor < 0 {
			inputFactor = 0
		}
	}

	score := density*100.0*lengthFactor*inputFactor + meowTokenBoost(squeezed)
	if score > 100 {
		score = 100
	}
	return score
}

func detectMeow(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	text := r.URL.Query().Get("text")
	w.Header().Set("Content-Type", "application/json")

	if text == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "Please provide text, e.g. ?text=mrrp"})
		return
	}

	squeezed := squeezeLetters(text)

	score := 0.0

	heads := []string{"mr", "pr", "ny", "m", "n", "p"}
	for _, h := range heads {
		if strings.HasPrefix(squeezed, h) {
			score += 30.0
			break
		}
	}

	tails := []string{"rp", "w", "p", "m", "n", "r", "y", "a", "e", "i", "o", "u"}
	for _, t := range tails {
		if strings.HasSuffix(squeezed, t) {
			score += 30.0
			break
		}
	}

	if strings.ContainsAny(squeezed, "aeiouywr") {
		score += 40.0
	}

	allowedChars := "mnpryeouiwa"
	for _, char := range squeezed {
		if !strings.ContainsRune(allowedChars, char) {
			score -= 30.0
		}
	}

	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	isMeow := score >= 70.0
	percString := fmt.Sprintf("%.1f%%", score)

	duration := time.Since(start)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"input":           text,
		"squeezed_form":   squeezed,
		"is_meow":         isMeow,
		"meow_percentage": percString,
		"detection_time":  duration.String(),
	})
}

func detectMeowLike(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	text := r.URL.Query().Get("text")
	w.Header().Set("Content-Type", "application/json")

	if text == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "Please provide text, e.g. ?text=miao"})
		return
	}

	squeezed := squeezeLetters(text)
	letterCount := countLetters(text)
	score := scoreMeowLike(squeezed, letterCount)
	isMeowLike := score >= meowLikeThreshold
	percString := fmt.Sprintf("%.1f%%", score)

	duration := time.Since(start)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"input":           text,
		"squeezed_form":   squeezed,
		"is_meow_like":    isMeowLike,
		"meow_percentage": percString,
		"detection_time":  duration.String(),
	})
}

func askMeow(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	text := r.URL.Query().Get("text")
	w.Header().Set("Content-Type", "application/json")

	if text == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "Please provide a question, e.g. ?text=is it time for food"})
		return
	}

	cleanText := strings.ToLower(strings.TrimSpace(text))

	h := fnv.New32a()
	h.Write([]byte(cleanText))
	hashValue := h.Sum32()

	answer := askMeowAnswers[hashValue%uint32(len(askMeowAnswers))]

	duration := time.Since(start)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"question":     text,
		"answer":       answer,
		"latency_time": duration.String(),
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
            Returns a procedurally generated vocalization.
        </li>
        <li>
            <code>GET <a href="/ismeow?text=mrrp">/ismeow?text={input}</a></code><br><br>
            Returns phonetic trait-analysis confidence score.
        </li>
		<li>
            <code>GET <a href="/meowlike?text=miao">/meowlike?text={input}</a></code><br><br>
            Returns phonetic trait-analysis confidence score for meow-like vocalizations.
        </li>
        <li>
            <code>GET <a href="/askmeow?text=hello">/askmeow?text={question}</a></code><br><br>
            Returns a deterministic response based on the input question.
        </li>
    </ul>
    <br>
    <p>Made by 7w1</p>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
