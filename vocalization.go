package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
	"unicode"
)

type vocalizationFamily string

const (
	familyMeow vocalizationFamily = "meow"
	familyNya  vocalizationFamily = "nya"
	familyPrrr vocalizationFamily = "prrr"

	strictVocalizationThreshold = 75.0
	meowLikeThreshold           = 55.0
	maxVocalizationInputLetters = 512
)

type vocalizationForm struct {
	text      string
	family    vocalizationFamily
	exactOnly bool
}

var vocalizationForms = []vocalizationForm{
	{family: familyMeow, text: "meow"},
	{family: familyMeow, text: "mreow"},
	{family: familyMeow, text: "mrow"},
	{family: familyMeow, text: "mroe"},
	{family: familyMeow, text: "mwor"},
	{family: familyMeow, text: "mweo"},
	{family: familyMeow, text: "mew"},
	{family: familyMeow, text: "miao"},
	{family: familyMeow, text: "miau"},
	{family: familyMeow, text: "rawr", exactOnly: true},
	{family: familyNya, text: "nya"},
	{family: familyNya, text: "nyan"},
	{family: familyPrrr, text: "prr"},
	{family: familyPrrr, text: "purr"},
	{family: familyPrrr, text: "mrrp"},
	{family: familyPrrr, text: "mrp", exactOnly: true},
}

type generationFamily struct {
	family        vocalizationFamily
	variants      []string
	weight        int
	stretchChars  string
	stretchChance float64
	maxExtra      int
}

var generationFamilies = []generationFamily{
	{
		family:        familyMeow,
		variants:      []string{"meow", "mrow", "mreow"},
		weight:        4,
		stretchChars:  "eow",
		stretchChance: 0.65,
		maxExtra:      4,
	},
	{
		family:        familyNya,
		variants:      []string{"nya", "nyan"},
		weight:        3,
		stretchChars:  "an",
		stretchChance: 0.65,
		maxExtra:      4,
	},
	{
		family:        familyPrrr,
		variants:      []string{"prrr", "purr"},
		weight:        3,
		stretchChars:  "ur",
		stretchChance: 0.55,
		maxExtra:      3,
	},
}

func stretchVocalization(text string, allowed string, chance float64, maxExtra int) string {
	var builder strings.Builder
	builder.Grow(len(text) + maxExtra*len(text))

	for _, char := range text {
		builder.WriteRune(char)
		if maxExtra > 0 && strings.ContainsRune(allowed, char) && rand.Float64() < chance {
			extra := rand.Intn(maxExtra) + 1
			builder.WriteString(strings.Repeat(string(char), extra))
		}
	}

	return builder.String()
}

func generateVocalization() (string, vocalizationFamily) {
	totalWeight := 0
	for _, family := range generationFamilies {
		totalWeight += family.weight
	}

	roll := rand.Intn(totalWeight)
	for _, family := range generationFamilies {
		if roll < family.weight {
			variant := family.variants[rand.Intn(len(family.variants))]
			return stretchVocalization(
				variant,
				family.stretchChars,
				family.stretchChance,
				family.maxExtra,
			), family.family
		}
		roll -= family.weight
	}

	return "meow", familyMeow
}

func generateMeow(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	finalMeow, family := generateVocalization()

	duration := time.Since(start)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"meow":            finalMeow,
		"family":          string(family),
		"generation_time": duration.String(),
	})
}

type normalizedVocalization struct {
	letters      string
	squeezed     string
	unknownCount int
}

func normalizeVocalization(text string) normalizedVocalization {
	var letters strings.Builder
	var squeezed strings.Builder
	letters.Grow(len(text))
	squeezed.Grow(len(text))

	var lastChar byte
	runLength := 0
	unknownCount := 0

	for _, char := range strings.ToLower(text) {
		if char >= 'a' && char <= 'z' {
			letter := byte(char)
			letters.WriteByte(letter)

			if letter != lastChar {
				squeezed.WriteByte(letter)
				lastChar = letter
				runLength = 1
				continue
			}

			if letter == 'r' && runLength < 2 {
				squeezed.WriteByte(letter)
			}
			runLength++
			continue
		}

		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			unknownCount++
		}
	}

	return normalizedVocalization{
		letters:      letters.String(),
		squeezed:     squeezed.String(),
		unknownCount: unknownCount,
	}
}

func squeezeLetters(text string) string {
	return normalizeVocalization(text).squeezed
}

func countLetters(text string) int {
	return len(normalizeVocalization(text).letters)
}

type segmentationState struct {
	valid         bool
	distance      int
	segments      int
	exactSegments int
}

type familyMatch struct {
	family        vocalizationFamily
	matched       bool
	distance      int
	segments      int
	exactSegments int
}

func betterSegmentation(candidate, current segmentationState) bool {
	if !current.valid {
		return true
	}
	if candidate.distance != current.distance {
		return candidate.distance < current.distance
	}
	if candidate.exactSegments != current.exactSegments {
		return candidate.exactSegments > current.exactSegments
	}
	return candidate.segments < current.segments
}

func levenshteinDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}

	for leftIndex := 1; leftIndex <= len(left); leftIndex++ {
		current[0] = leftIndex
		for rightIndex := 1; rightIndex <= len(right); rightIndex++ {
			cost := 0
			if left[leftIndex-1] != right[rightIndex-1] {
				cost = 1
			}

			deletion := previous[rightIndex] + 1
			insertion := current[rightIndex-1] + 1
			substitution := previous[rightIndex-1] + cost
			current[rightIndex] = minInt(deletion, insertion, substitution)
		}
		previous, current = current, previous
	}

	return previous[len(right)]
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func formsForFamily(family vocalizationFamily) []vocalizationForm {
	forms := make([]vocalizationForm, 0)
	for _, form := range vocalizationForms {
		if form.family == family {
			forms = append(forms, form)
		}
	}
	return forms
}

func bestFamilyMatch(squeezed string, family vocalizationFamily, maxEditPerSegment int) familyMatch {
	result := familyMatch{family: family}
	if squeezed == "" {
		return result
	}

	states := make([]segmentationState, len(squeezed)+1)
	states[0] = segmentationState{valid: true}

	for position := 0; position < len(squeezed); position++ {
		if !states[position].valid {
			continue
		}

		for _, form := range formsForFamily(family) {
			editLimit := maxEditPerSegment
			if form.exactOnly {
				editLimit = 0
			}

			minLength := len(form.text) - editLimit
			if minLength < 1 {
				minLength = 1
			}
			maxLength := len(form.text) + editLimit

			for length := minLength; length <= maxLength && position+length <= len(squeezed); length++ {
				candidateText := squeezed[position : position+length]
				distance := levenshteinDistance(candidateText, form.text)
				if distance > editLimit {
					continue
				}

				candidate := segmentationState{
					valid:         true,
					distance:      states[position].distance + distance,
					segments:      states[position].segments + 1,
					exactSegments: states[position].exactSegments,
				}
				if distance == 0 {
					candidate.exactSegments++
				}

				nextPosition := position + length
				if betterSegmentation(candidate, states[nextPosition]) {
					states[nextPosition] = candidate
				}
			}
		}
	}

	finalState := states[len(squeezed)]
	if !finalState.valid {
		return result
	}

	result.matched = true
	result.distance = finalState.distance
	result.segments = finalState.segments
	result.exactSegments = finalState.exactSegments
	return result
}

func familyAnchorOK(squeezed string, match familyMatch) bool {
	if !match.matched || len(squeezed) < 3 {
		return false
	}

	switch match.family {
	case familyMeow:
		return strings.HasPrefix(squeezed, "m") || squeezed == "rawr"
	case familyNya:
		return strings.HasPrefix(squeezed, "ny")
	case familyPrrr:
		return (strings.HasPrefix(squeezed, "p") || strings.HasPrefix(squeezed, "m")) &&
			(strings.Contains(squeezed, "rr") || squeezed == "mrp")
	default:
		return false
	}
}

func scoreFamilyMatch(match familyMatch, normalized normalizedVocalization) float64 {
	if !familyAnchorOK(normalized.squeezed, match) {
		return 0
	}

	score := 100.0
	if len(normalized.squeezed) > 0 {
		score -= 100.0 * float64(match.distance) / float64(len(normalized.squeezed))
	}
	score -= 35.0 * float64(normalized.unknownCount)

	if len(normalized.letters) > maxVocalizationInputLetters {
		score -= 40.0
	}

	if match.segments == 1 && len(normalized.squeezed) < 4 && match.distance > 0 {
		score -= 15.0
	}

	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func allowedStrictErrors(segments int) int {
	allowed := segments / 4
	if allowed < 1 {
		return 1
	}
	return allowed
}

func allowedFuzzyErrors(segments int) int {
	allowed := segments / 2
	if allowed < 1 {
		return 1
	}
	return allowed
}

type vocalizationAnalysis struct {
	normalized  normalizedVocalization
	family      vocalizationFamily
	matchType   string
	strictScore float64
	fuzzyScore  float64
	isStrict    bool
	isFuzzy     bool
}

func analyzeVocalization(text string) vocalizationAnalysis {
	normalized := normalizeVocalization(text)
	analysis := vocalizationAnalysis{
		normalized: normalized,
		family:     "unknown",
		matchType:  "none",
	}

	var bestMatch familyMatch
	bestScore := 0.0
	for _, family := range []vocalizationFamily{familyMeow, familyNya, familyPrrr} {
		match := bestFamilyMatch(normalized.squeezed, family, 1)
		score := scoreFamilyMatch(match, normalized)
		if score > bestScore {
			bestScore = score
			bestMatch = match
			analysis.family = match.family
		}

		if score >= analysis.strictScore {
			analysis.strictScore = score
		}
		if score >= analysis.fuzzyScore {
			analysis.fuzzyScore = score
		}

		strictEligible := match.matched &&
			familyAnchorOK(normalized.squeezed, match) &&
			normalized.unknownCount == 0 &&
			len(normalized.letters) <= maxVocalizationInputLetters &&
			match.distance <= allowedStrictErrors(match.segments)
		fuzzyEligible := match.matched &&
			familyAnchorOK(normalized.squeezed, match) &&
			normalized.unknownCount <= 2 &&
			match.distance <= allowedFuzzyErrors(match.segments)

		if strictEligible && score >= strictVocalizationThreshold {
			analysis.isStrict = true
		}
		if fuzzyEligible && score >= meowLikeThreshold {
			analysis.isFuzzy = true
		}
	}

	if bestScore > 0 {
		if bestMatch.distance > 0 || normalized.unknownCount > 0 {
			analysis.matchType = "typo"
		} else if normalized.letters != normalized.squeezed {
			analysis.matchType = "stretched"
		} else {
			analysis.matchType = "exact"
		}
	}

	return analysis
}

func detectMeow(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	text := r.URL.Query().Get("text")
	w.Header().Set("Content-Type", "application/json")

	if text == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "Please provide text, e.g. ?text=mrrp"})
		return
	}

	analysis := analyzeVocalization(text)
	percString := fmt.Sprintf("%.1f%%", analysis.strictScore)

	duration := time.Since(start)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"input":           text,
		"squeezed_form":   analysis.normalized.squeezed,
		"is_meow":         analysis.isStrict,
		"meow_percentage": percString,
		"family":          string(analysis.family),
		"match_type":      analysis.matchType,
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

	analysis := analyzeVocalization(text)
	percString := fmt.Sprintf("%.1f%%", analysis.fuzzyScore)

	duration := time.Since(start)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"input":           text,
		"squeezed_form":   analysis.normalized.squeezed,
		"is_meow_like":    analysis.isFuzzy,
		"meow_percentage": percString,
		"family":          string(analysis.family),
		"match_type":      analysis.matchType,
		"detection_time":  duration.String(),
	})
}
