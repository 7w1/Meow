package main

import (
	"encoding/json"
	"fmt"
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
	EnableLandingPage bool   `json:"enable_landing_page"`
}

func loadConfig() Config {
	cfg := Config{
		Port:              "8000",
		GenerateEndpoint:  "/meow",
		DetectEndpoint:    "/ismeow",
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

	fmt.Printf("MaaS is running on port %s (ฅ'ω'ฅ)\n", cfg.Port)
	fmt.Printf("    -> Generation: %s\n", cfg.GenerateEndpoint)
	fmt.Printf("    -> Detection: %s\n", cfg.DetectEndpoint)

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

func detectMeow(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	text := r.URL.Query().Get("text")
	w.Header().Set("Content-Type", "application/json")

	if text == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "Please provide text, e.g. ?text=mrrp"})
		return
	}

	textClean := strings.ToLower(strings.TrimSpace(text))

	var builder strings.Builder
	var lastChar rune
	for _, char := range textClean {
		if char != lastChar {
			builder.WriteRune(char)
			lastChar = char
		}
	}
	squeezed := builder.String()

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
    <ul>
        <li>
            <code>GET <a href="/meow">/meow</a></code><br><br>
            Returns a procedurally generated vocalization.
        </li>
        <li>
            <code>GET <a href="/ismeow?text=mrrp">/ismeow?text={input}</a></code><br><br>
            Returns phonetic trait-analysis confidence score.
        </li>
    </ul>
    <br>
    <p>Made by 7w1</p>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
