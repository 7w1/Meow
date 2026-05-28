package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
)

type Config struct {
	Port             string `json:"port"`
	GenerateEndpoint string `json:"generate_endpoint"`
	DetectEndpoint   string `json:"detect_endpoint"`
}

func loadConfig() Config {
	cfg := Config{
		Port:             "8000",
		GenerateEndpoint: "/meow",
		DetectEndpoint:   "/ismeow",
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

	http.HandleFunc(cfg.GenerateEndpoint, generateMeow)
	http.HandleFunc(cfg.DetectEndpoint, detectMeow)

	fmt.Printf("MaaS is running on port %s (ฅ'ω'ฅ)\n", cfg.Port)
	fmt.Printf("    -> Generation: %s\n", cfg.GenerateEndpoint)
	fmt.Printf("    -> Detection: %s\n", cfg.DetectEndpoint)

	http.ListenAndServe(":"+cfg.Port, nil)
}

func generateMeow(w http.ResponseWriter, r *http.Request) {
	heads := []string{"m", "n", "p", "mr", "pr", "ny"}
	cores := []string{"e", "eo", "a", "ow", "aw", "u", "ya", "io", "ew"}
	tails := []string{"w", "p", "m", "f", "rp", "th", ""}

	head := heads[rand.Intn(len(heads))]
	core := cores[rand.Intn(len(cores))]
	tail := tails[rand.Intn(len(tails))]

	stretch := func(s string, chance float32, maxStretch int) string {
		if rand.Float32() < chance && len(s) > 0 {
			charToStretch := s[rand.Intn(len(s)-1)]
			repeats := rand.Intn(maxStretch) + 1
			s += strings.Repeat(string(charToStretch), repeats)
		}
		return s
	}

	head = stretch(head, 0.4, 4)
	core = stretch(core, 0.8, 6)
	tail = stretch(tail, 0.5, 4)

	finalMeow := head + core + tail

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"meow": finalMeow})
}

func detectMeow(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	w.Header().Set("Content-Type", "application/json")

	if text == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "Please provide text, e.g. ?text=mrrp"})
		return
	}

	textClean := strings.ToLower(strings.TrimSpace(text))
	meowAlphabet := "meowrpnay"
	meowChars := 0

	for _, char := range textClean {
		if strings.ContainsRune(meowAlphabet, char) {
			meowChars++
		}
	}

	ratio := float64(meowChars) / float64(len(textClean))
	percentage := ratio * 100
	isMeow := percentage >= 60.0

	percString := fmt.Sprintf("%.1f%%", percentage)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"input":           text,
		"is_meow":         isMeow,
		"meow_percentage": percString,
	})
}
