package main

import (
	"math/rand"
	"net/http"
	"strings"
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
	anchor    string
	embeddedOK bool
	embeddedOnly bool
}

var vocalizationForms = []vocalizationForm{
	{family: familyMeow, text: "meow", embeddedOK: true},
	{family: familyMeow, text: "meows", exactOnly: true, embeddedOK: true, embeddedOnly: true},
	{family: familyMeow, text: "meowing", exactOnly: true, embeddedOK: true, embeddedOnly: true},
	{family: familyMeow, text: "mreow", embeddedOK: true},
	{family: familyMeow, text: "mrow", embeddedOK: true},
	{family: familyMeow, text: "mroe"},
	{family: familyMeow, text: "mwor"},
	{family: familyMeow, text: "mweo"},
	{family: familyMeow, text: "mew", embeddedOK: true},
	{family: familyMeow, text: "miao", embeddedOK: true},
	{family: familyMeow, text: "miau", embeddedOK: true},
	{family: familyMeow, text: "miaow", exactOnly: true, embeddedOK: true},
	{family: familyMeow, text: "rawr", exactOnly: true, embeddedOK: true},
	{family: familyNya, text: "nya", embeddedOK: true},
	{family: familyNya, text: "nyan", embeddedOK: true},
	{family: familyPrrr, text: "prr", embeddedOK: true},
	{family: familyPrrr, text: "purr", embeddedOK: true},
	{family: familyPrrr, text: "purrfect", exactOnly: true, embeddedOK: true, embeddedOnly: true},
	{family: familyPrrr, text: "mrrp", embeddedOK: true},
	{family: familyPrrr, text: "mrp", exactOnly: true},
}

var vocalizationFormsByFamily = indexVocalizationForms()

func indexVocalizationForms() map[vocalizationFamily][]vocalizationForm {
	indexed := make(map[vocalizationFamily][]vocalizationForm)
	for _, form := range vocalizationForms {
		if !form.embeddedOnly {
			indexed[form.family] = append(indexed[form.family], form)
		}
	}
	return indexed
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
	serveGenerateMeow(w, r)
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
	return vocalizationFormsByFamily[family]
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
	isEmbedded  bool
	matchedText string
	coverage    float64
}

func analyzeVocalization(text string) vocalizationAnalysis {
	normalized := normalizeVocalization(text)
	analysis := vocalizationAnalysis{
		normalized: normalized,
		family:     "unknown",
		matchType:  "none",
	}
	for _, form := range vocalizationForms {
		if form.embeddedOnly && normalized.squeezed == form.text {
			return analysis
		}
	}
	if isMeowEcho(normalized.squeezed) {
		score := 100 - 35*float64(normalized.unknownCount)
		if len(normalized.letters) > maxVocalizationInputLetters {
			score -= 40
		}
		analysis.family = familyMeow
		analysis.matchType = "stretched"
		analysis.strictScore = max(0, score)
		analysis.fuzzyScore = analysis.strictScore
		analysis.isStrict = normalized.unknownCount == 0 && len(normalized.letters) <= maxVocalizationInputLetters && score >= strictVocalizationThreshold
		analysis.isFuzzy = normalized.unknownCount <= 2 && score >= meowLikeThreshold
		return analysis
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
	if !analysis.isStrict && !analysis.isFuzzy && normalized.unknownCount == 0 && isMixedMeow(normalized.squeezed) {
		analysis.family = familyMeow
		analysis.matchType = "mixed"
		analysis.fuzzyScore = 85
		analysis.isFuzzy = true
	}

	return analysis
}

func isMixedMeow(squeezed string) bool {
	for _, purr := range []string{"mrrp", "mrp", "prr", "purr"} {
		if strings.HasPrefix(squeezed, purr) && isLooseMeowSyllable(strings.TrimPrefix(squeezed, purr)) ||
			strings.HasSuffix(squeezed, purr) && isLooseMeowSyllable(strings.TrimSuffix(squeezed, purr)) {
			return true
		}
	}
	return strings.HasPrefix(squeezed, "mrr") && isLooseMeowTail(strings.TrimPrefix(squeezed, "mrr"))
}

func isLooseMeowSyllable(syllable string) bool {
	if len(syllable) < 4 || len(syllable) > 10 || syllable[0] != 'm' {
		return false
	}
	return isLooseMeowTail(syllable[1:])
}

func isLooseMeowTail(tail string) bool {
	if len(tail) < 3 || len(tail) > 9 || !strings.ContainsRune("aeiou", rune(tail[0])) {
		return false
	}
	vowels, wCount := 0, 0
	for i := 0; i < len(tail); i++ {
		switch tail[i] {
		case 'a', 'e', 'i', 'o', 'u':
			vowels++
		case 'w':
			wCount++
		default:
			return false
		}
	}
	return vowels >= 2 && wCount >= 1 && wCount <= 3
}

func isMeowEcho(squeezed string) bool {
	if !strings.HasPrefix(squeezed, "meow") {
		return false
	}
	tail := strings.TrimPrefix(squeezed, "meow")
	if len(tail) < 2 || len(tail)%2 != 0 {
		return false
	}
	for len(tail) > 0 {
		if !strings.HasPrefix(tail, "ow") {
			return false
		}
		tail = tail[2:]
	}
	return true
}

func detectMeow(w http.ResponseWriter, r *http.Request) {
	serveDetectMeow(w, r)
}

func detectMeowLike(w http.ResponseWriter, r *http.Request) {
	serveDetectMeowLike(w, r)
}
