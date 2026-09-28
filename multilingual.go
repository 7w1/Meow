package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const maxRequestTextBytes = 4096

type boundaryPolicy struct {
	allowGuardedSuffix bool
	allowGuardedHanSuffix bool
}

type languagePack struct {
	Tag           string
	Forms         []vocalizationForm
	Generators    []generationFamily
	Boundary      boundaryPolicy
	StretchChars  string
	MaxTypoEdits  int
	EmbeddedDenylist []string

	formsByFamily map[vocalizationFamily][]packForm
	embeddedDenylist map[string]struct{}
	formOrder     []packForm
	longestForm   int
}

type packForm struct {
	vocalizationForm
	normalized []rune
}

var languagePacks map[string]*languagePack
var supportedLanguageTags []string
var automaticLanguagePriority []string

func init() {
	var err error
	languagePacks, err = buildLanguageRegistry()
	if err != nil {
		panic(fmt.Sprintf("invalid language registry: %v", err))
	}
	supportedLanguageTags = make([]string, 0, len(languagePacks))
	for _, pack := range languagePacks {
		supportedLanguageTags = append(supportedLanguageTags, pack.Tag)
	}
	sort.Strings(supportedLanguageTags)
	automaticLanguagePriority = append(automaticLanguagePriority, "de", "en")
	for _, tag := range supportedLanguageTags {
		key := strings.ToLower(tag)
		if key != "de" && key != "en" {
			automaticLanguagePriority = append(automaticLanguagePriority, key)
		}
	}
}

func buildLanguageRegistry() (map[string]*languagePack, error) {
	frForms := []vocalizationForm{
		{family: familyMeow, text: "miaou", anchor: "m", embeddedOK: true},
		{family: familyPrrr, text: "ronron", anchor: "r", embeddedOK: true},
	}
	jaForms := []vocalizationForm{
		{family: familyNya, text: "にゃ", anchor: "に", embeddedOK: true},
		{family: familyNya, text: "にゃん", anchor: "に", embeddedOK: true},
		{family: familyNya, text: "にゃー", anchor: "に", embeddedOK: true},
		{family: familyNya, text: "にゃーん", anchor: "に", embeddedOK: true},
		{family: familyNya, text: "にゃーお", anchor: "に", embeddedOK: true},
		{family: familyNya, text: "にゃあ", anchor: "に", embeddedOK: true},
		{family: familyNya, text: "ニャン", anchor: "ニ", embeddedOK: true},
		{family: familyNya, text: "ニャーン", anchor: "ニ", embeddedOK: true},
		{family: familyNya, text: "ニャー", anchor: "ニ", embeddedOK: true},
		{family: familyNya, text: "ニャーオ", anchor: "ニ", embeddedOK: true},
		{family: familyNya, text: "nya", anchor: "ny", embeddedOK: true},
	}
	trForms := []vocalizationForm{
		{family: familyMeow, text: "miyav", anchor: "m", embeddedOK: true},
		{family: familyPrrr, text: "mır", anchor: "m", embeddedOK: true},
	}
	ruForms := []vocalizationForm{
		{family: familyMeow, text: "мяу", anchor: "м", embeddedOK: true},
		{family: familyPrrr, text: "мур", anchor: "м", embeddedOK: true},
	}
	ukForms := []vocalizationForm{
		{family: familyMeow, text: "няв", anchor: "н", embeddedOK: true},
		{family: familyMeow, text: "нявкати", anchor: "н"},
		{family: familyPrrr, text: "мур", anchor: "м", embeddedOK: true},
	}
	zhHansForms := []vocalizationForm{
		{family: familyMeow, text: "喵", anchor: "喵", exactOnly: true},
		{family: familyMeow, text: "喵喵", anchor: "喵", exactOnly: true, embeddedOK: true},
		{family: familyPrrr, text: "咕噜", anchor: "咕", exactOnly: true, embeddedOK: true},
		{family: familyPrrr, text: "咕噜咕噜", anchor: "咕", exactOnly: true, embeddedOK: true},
	}
	zhHantForms := []vocalizationForm{
		{family: familyMeow, text: "喵", anchor: "喵", exactOnly: true},
		{family: familyMeow, text: "喵喵", anchor: "喵", exactOnly: true, embeddedOK: true},
		{family: familyPrrr, text: "咕嚕", anchor: "咕", exactOnly: true, embeddedOK: true},
		{family: familyPrrr, text: "咕嚕咕嚕", anchor: "咕", exactOnly: true, embeddedOK: true},
	}
	esForms := []vocalizationForm{
		{family: familyMeow, text: "miau", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "miao", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "mau", anchor: "m", exactOnly: true},
		{family: familyMeow, text: "meau", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "miáu", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "marramiau", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "marramáu", anchor: "m", exactOnly: true, embeddedOK: true},
		{family: familyMeow, text: "marramau", anchor: "m", exactOnly: true, embeddedOK: true},
		{family: familyPrrr, text: "rrr", anchor: "r", exactOnly: true, embeddedOK: true},
	}
	nlForms := []vocalizationForm{
		{family: familyMeow, text: "miauw", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "mauw", anchor: "m", embeddedOK: true},
	}
	deForms := []vocalizationForm{
		{family: familyMeow, text: "miau", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "mau", anchor: "m", exactOnly: true},
		{family: familyMeow, text: "maunz", anchor: "m", embeddedOK: true},
		{family: familyPrrr, text: "schnurr", anchor: "s", embeddedOK: true},
	}
	hyForms := []vocalizationForm{
		{family: familyMeow, text: "մյաու", anchor: "մ", embeddedOK: true},
		{family: familyPrrr, text: "մըռռ", anchor: "մ", embeddedOK: true},
	}
	eoForms := []vocalizationForm{
		{family: familyMeow, text: "miaŭ", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "mjaŭ", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "miaux", anchor: "m", embeddedOK: true},
		{family: familyMeow, text: "mjaux", anchor: "m", embeddedOK: true},
		{family: familyPrrr, text: "mur-mur", anchor: "m", embeddedOK: true},
		{family: familyPrrr, text: "ronron", anchor: "r", embeddedOK: true},
	}
	arForms := []vocalizationForm{
		{family: familyMeow, text: "مواء", anchor: "م", embeddedOK: true},
		{family: familyMeow, text: "مياو", anchor: "م", embeddedOK: true},
	}
	hiForms := []vocalizationForm{
		{family: familyMeow, text: "म्याऊँ", anchor: "म", embeddedOK: true},
		{family: familyMeow, text: "म्याऊं", anchor: "म", embeddedOK: true},
		{family: familyMeow, text: "मियाऊँ", anchor: "म", embeddedOK: true},
		{family: familyMeow, text: "म्याँव", anchor: "म", embeddedOK: true},
		{family: familyMeow, text: "मियांउ", anchor: "म", embeddedOK: true},
	}
	registry := map[string]*languagePack{
		"en": {
			Tag: "en", Forms: vocalizationForms, Generators: generationFamilies,
			MaxTypoEdits: 1,
			EmbeddedDenylist: []string{"somewhere", "now"},
		},
		"fr": {
			Tag: "fr", Forms: frForms, StretchChars: "aeiou", MaxTypoEdits: 1,
			EmbeddedDenylist: []string{"miaous"},
			Generators: []generationFamily{{
				family: familyMeow, variants: []string{"miaou"}, weight: 8,
				stretchChars: "aeiou", stretchChance: 0.45, maxExtra: 3,
			}},
		},
		"ja": {
			Tag: "ja", Forms: jaForms, StretchChars: "あいうえおー",
			Generators: []generationFamily{{
				family: familyNya, variants: []string{"にゃー", "にゃあ"}, weight: 10,
				stretchChars: "ーあ", stretchChance: 0.35, maxExtra: 2,
			}},
			Boundary: boundaryPolicy{allowGuardedSuffix: true},
		},
		"tr": {
			Tag: "tr", Forms: trForms, StretchChars: "iaıu", MaxTypoEdits: 1,
			Generators: []generationFamily{
				{family: familyMeow, variants: []string{"miyav", "miyav-miyav"}, weight: 8, stretchChars: "ia", stretchChance: 0.4, maxExtra: 3},
				{family: familyPrrr, variants: []string{"mır", "mır-mır"}, weight: 2, stretchChars: "ı", stretchChance: 0.35, maxExtra: 2},
			},
		},
		"ru": {
			Tag: "ru", Forms: ruForms, StretchChars: "яауур", MaxTypoEdits: 1,
			Generators: []generationFamily{
				{family: familyMeow, variants: []string{"мяу", "мяу-мяу"}, weight: 8, stretchChars: "ау", stretchChance: 0.4, maxExtra: 3},
				{family: familyPrrr, variants: []string{"мур", "мур-мур"}, weight: 2, stretchChars: "ур", stretchChance: 0.4, maxExtra: 3},
			},
		},
		"uk": {
			Tag: "uk", Forms: ukForms, StretchChars: "яау", MaxTypoEdits: 1,
			Generators: []generationFamily{
				{family: familyMeow, variants: []string{"няв", "няв-няв"}, weight: 8},
				{family: familyPrrr, variants: []string{"мур-мур", "мур"}, weight: 2},
			},
		},
		"zh-hans": {
			Tag: "zh-Hans", Forms: zhHansForms,
			Generators: []generationFamily{
			{family: familyMeow, variants: []string{"喵", "喵喵"}, weight: 8},
			{family: familyPrrr, variants: []string{"咕噜", "咕噜咕噜"}, weight: 2},
			},
			Boundary: boundaryPolicy{allowGuardedHanSuffix: true},
		},
		"zh-hant": {
			Tag: "zh-Hant", Forms: zhHantForms,
			Generators: []generationFamily{
			{family: familyMeow, variants: []string{"喵", "喵喵"}, weight: 8},
			{family: familyPrrr, variants: []string{"咕嚕", "咕嚕咕嚕"}, weight: 2},
			},
			Boundary: boundaryPolicy{allowGuardedHanSuffix: true},
		},
		"es": {
			Tag: "es", Forms: esForms, StretchChars: "iaueá", MaxTypoEdits: 1,
			Generators: []generationFamily{
				{family: familyMeow, variants: []string{"miau", "miao", "mau", "meau", "miáu", "marramiau"}, weight: 9, stretchChars: "iau", stretchChance: 0.45, maxExtra: 3},
				{family: familyPrrr, variants: []string{"rrr"}, weight: 1},
			},
		},
		"nl": {
			Tag: "nl", Forms: nlForms, StretchChars: "iauw", MaxTypoEdits: 1,
			Generators: []generationFamily{{family: familyMeow, variants: []string{"miauw", "mauw"}, weight: 10, stretchChars: "iau", stretchChance: 0.4, maxExtra: 3}},
		},
		"de": {
			Tag: "de", Forms: deForms, StretchChars: "iau",
			Generators: []generationFamily{
				{family: familyMeow, variants: []string{"miau", "mau", "maunz"}, weight: 9, stretchChars: "iau", stretchChance: 0.45, maxExtra: 3},
				{family: familyPrrr, variants: []string{"schnurr"}, weight: 1, stretchChars: "u", stretchChance: 0.35, maxExtra: 2},
			},
		},
		"hy": {
			Tag: "hy", Forms: hyForms, StretchChars: "աուըռ", MaxTypoEdits: 1,
			Generators: []generationFamily{
				{family: familyMeow, variants: []string{"մյաու", "մյաու-մյաու"}, weight: 8, stretchChars: "աու", stretchChance: 0.4, maxExtra: 2},
				{family: familyPrrr, variants: []string{"մըռռ"}, weight: 2, stretchChars: "ռ", stretchChance: 0.35, maxExtra: 2},
			},
		},
		"eo": {
			Tag: "eo", Forms: eoForms,
			Generators: []generationFamily{
				{family: familyMeow, variants: []string{"miaŭ", "mjaŭ", "miaŭ-miaŭ"}, weight: 8},
				{family: familyPrrr, variants: []string{"mur-mur", "ronron"}, weight: 2},
			},
		},
		"ar": {
			Tag: "ar", Forms: arForms,
			Generators: []generationFamily{{family: familyMeow, variants: []string{"مواء", "مياو"}, weight: 10}},
		},
		"hi": {
			Tag: "hi", Forms: hiForms,
			Generators: []generationFamily{{family: familyMeow, variants: []string{"म्याऊँ", "म्याऊँ-म्याऊँ"}, weight: 10}},
		},
	}

	for tag, pack := range registry {
		if strings.ToLower(pack.Tag) != tag {
			return nil, fmt.Errorf("registry key %q does not match pack tag %q", tag, pack.Tag)
		}
		if err := prepareLanguagePack(pack); err != nil {
			return nil, fmt.Errorf("%s: %w", tag, err)
		}
		if err := validateLanguagePack(pack); err != nil {
			return nil, fmt.Errorf("%s: %w", tag, err)
		}
	}
	return registry, nil
}

func prepareLanguagePack(pack *languagePack) error {
	if pack.Tag == "" || len(pack.Forms) == 0 || len(pack.Generators) == 0 {
		return fmt.Errorf("tag, forms, and generators must be nonempty")
	}
	if pack.MaxTypoEdits < 0 || pack.MaxTypoEdits > 1 {
		return fmt.Errorf("maximum typo distance must be between zero and one")
	}
	pack.formsByFamily = make(map[vocalizationFamily][]packForm)
	pack.embeddedDenylist = make(map[string]struct{}, len(pack.EmbeddedDenylist))
	for _, word := range pack.EmbeddedDenylist {
		canonical := canonicalTokenForPack(word, pack)
		if canonical == "" {
			return fmt.Errorf("embedded denylist word %q normalizes to empty", word)
		}
		pack.embeddedDenylist[canonical] = struct{}{}
	}
	seen := make(map[string]bool)
	for _, form := range pack.Forms {
		if form.text == "" {
			return fmt.Errorf("empty vocalization form")
		}
		canonical := canonicalTokenForPack(form.text, pack)
		if canonical == "" {
			return fmt.Errorf("form %q normalizes to empty", form.text)
		}
		key := string(form.family) + "\x00" + canonical
		if seen[key] {
			return fmt.Errorf("duplicate form %q in family %q", form.text, form.family)
		}
		seen[key] = true
		compiled := packForm{vocalizationForm: form, normalized: []rune(canonical)}
		if !form.embeddedOnly {
			pack.formsByFamily[form.family] = append(pack.formsByFamily[form.family], compiled)
		}
		pack.formOrder = append(pack.formOrder, compiled)
		if len(compiled.normalized) > pack.longestForm {
			pack.longestForm = len(compiled.normalized)
		}
	}
	for _, family := range pack.Generators {
		if family.weight <= 0 || len(family.variants) == 0 {
			return fmt.Errorf("generator family %q must have positive weight and variants", family.family)
		}
		for _, variant := range family.variants {
			if variant == "" {
				return fmt.Errorf("generator family %q has an empty variant", family.family)
			}
		}
	}
	return nil
}

func validateLanguagePack(pack *languagePack) error {
	for _, family := range pack.Generators {
		for _, variant := range family.variants {
			var analysis vocalizationAnalysis
			if pack.Tag == "en" {
				analysis = analyzeVocalization(variant)
			} else {
				analysis = analyzePackWhole(variant, pack)
			}
			if !analysis.isStrict || analysis.family != family.family {
				return fmt.Errorf("generated variant %q does not strictly round-trip as %q", variant, family.family)
			}
		}
	}
	return nil
}

func canonicalTokenForPack(text string, pack *languagePack) string {
	if pack.Tag == "en" {
		return normalizeVocalization(text).squeezed
	}
	return normalizePackToken(text, pack)
}

func normalizePackToken(text string, pack *languagePack) string {
	folded := cases.Fold().String(norm.NFKC.String(text))
	var result strings.Builder
	result.Grow(len(folded))
	var previous rune
	for _, r := range folded {
		if !unicode.IsLetter(r) && !(pack.Tag == "hi" && unicode.IsMark(r)) {
			previous = 0
			continue
		}
		if r == previous && strings.ContainsRune(pack.StretchChars, r) {
			continue
		}
		result.WriteRune(r)
		previous = r
	}
	return result.String()
}

func normalizedPackInput(text string, pack *languagePack) normalizedVocalization {
	folded := cases.Fold().String(norm.NFKC.String(text))
	var letters strings.Builder
	var previous rune
	var squeezed strings.Builder
	unknownCount := 0
	for _, r := range folded {
		if unicode.IsLetter(r) || pack.Tag == "hi" && unicode.IsMark(r) {
			letters.WriteRune(r)
			if r != previous || !strings.ContainsRune(pack.StretchChars, r) {
				squeezed.WriteRune(r)
				previous = r
			}
		} else if unicode.IsNumber(r) {
			unknownCount++
			previous = 0
		} else {
			previous = 0
		}
	}
	return normalizedVocalization{letters: letters.String(), squeezed: squeezed.String(), unknownCount: unknownCount}
}

type packSegmentationState struct {
	valid    bool
	distance int
	segments int
}

type packFamilyMatch struct {
	family   vocalizationFamily
	matched  bool
	distance int
	segments int
}

func levenshteinRunes(left, right []rune) int {
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for i := range previous {
		previous[i] = i
	}
	for i := 1; i <= len(left); i++ {
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 0
			if left[i-1] != right[j-1] {
				cost = 1
			}
			current[j] = minInt(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(right)]
}

func betterPackSegmentation(candidate, current packSegmentationState) bool {
	if !current.valid {
		return true
	}
	if candidate.distance != current.distance {
		return candidate.distance < current.distance
	}
	return candidate.segments < current.segments
}

func bestPackFamilyMatch(input []rune, family vocalizationFamily, pack *languagePack) packFamilyMatch {
	result := packFamilyMatch{family: family}
	if len(input) == 0 {
		return result
	}
	forms := pack.formsByFamily[family]
	states := make([]packSegmentationState, len(input)+1)
	states[0] = packSegmentationState{valid: true}
	for position := 0; position < len(input); position++ {
		if !states[position].valid {
			continue
		}
		for _, form := range forms {
			editLimit := pack.MaxTypoEdits
			if form.exactOnly || len(form.normalized) < 4 {
				editLimit = 0
			}
			minLength := len(form.normalized) - editLimit
			if minLength < 1 {
				minLength = 1
			}
			for length := minLength; length <= len(form.normalized)+editLimit && position+length <= len(input); length++ {
				candidateRunes := input[position : position+length]
				distance := levenshteinRunes(candidateRunes, form.normalized)
				if distance > editLimit || !formAnchorOK(string(candidateRunes), form, pack) {
					continue
				}
				candidate := packSegmentationState{valid: true, distance: states[position].distance + distance, segments: states[position].segments + 1}
				next := position + length
				if betterPackSegmentation(candidate, states[next]) {
					states[next] = candidate
				}
			}
		}
	}
	last := states[len(input)]
	if last.valid {
		result.matched, result.distance, result.segments = true, last.distance, last.segments
	}
	return result
}

func analyzePackWhole(text string, pack *languagePack) vocalizationAnalysis {
	normalized := normalizedPackInput(text, pack)
	analysis := vocalizationAnalysis{normalized: normalized, family: "unknown", matchType: "none"}
	input := []rune(normalized.squeezed)
	bestScore := 0.0
	var best packFamilyMatch
	seenFamilies := make(map[vocalizationFamily]bool)
	families := make([]vocalizationFamily, 0, len(pack.formsByFamily))
	for _, form := range pack.Forms {
		if !seenFamilies[form.family] {
			seenFamilies[form.family] = true
			families = append(families, form.family)
		}
	}
	for _, family := range families {
		match := bestPackFamilyMatch(input, family, pack)
		if !match.matched {
			continue
		}
		score := 100.0
		if len(input) > 0 {
			score -= 100 * float64(match.distance) / float64(len(input))
		}
		if normalized.unknownCount > 0 {
			score -= 35 * float64(normalized.unknownCount)
		}
		if score < 0 {
			score = 0
		}
		if score > analysis.strictScore {
			analysis.strictScore = score
		}
		if score > analysis.fuzzyScore {
			analysis.fuzzyScore = score
		}
		strictEligible := match.distance <= allowedStrictErrors(match.segments) && normalized.unknownCount == 0 && len(input) <= maxVocalizationInputLetters
		unknownLimit := 2
		if pack.Tag != "en" {
			unknownLimit = 0
		}
		fuzzyEligible := match.distance <= allowedFuzzyErrors(match.segments) && normalized.unknownCount <= unknownLimit
		if strictEligible && score >= strictVocalizationThreshold {
			analysis.isStrict = true
		}
		if fuzzyEligible && score >= meowLikeThreshold {
			analysis.isFuzzy = true
		}
		if score > bestScore {
			bestScore, best, analysis.family = score, match, family
		}
	}
	if bestScore > 0 {
		if best.distance > 0 || normalized.unknownCount > 0 {
			analysis.matchType = "typo"
		} else if normalized.letters != normalized.squeezed {
			analysis.matchType = "stretched"
		} else {
			analysis.matchType = "exact"
		}
	}
	return analysis
}

func analyzeForLanguage(text string, pack *languagePack) vocalizationAnalysis {
	if pack.Tag == "en" {
		return analyzeVocalization(text)
	}
	return analyzePackWhole(text, pack)
}

func generateForLanguage(pack *languagePack) (string, vocalizationFamily) {
	if pack.Tag == "en" {
		return generateVocalization()
	}
	totalWeight := 0
	for _, family := range pack.Generators {
		totalWeight += family.weight
	}
	roll := rand.Intn(totalWeight)
	for _, family := range pack.Generators {
		if roll < family.weight {
			variant := family.variants[rand.Intn(len(family.variants))]
			return stretchVocalization(variant, family.stretchChars, family.stretchChance, family.maxExtra), family.family
		}
		roll -= family.weight
	}
	return pack.Generators[0].variants[0], pack.Generators[0].family
}

func parseRequestQuery(r *http.Request) (url.Values, *apiError) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, &apiError{status: http.StatusBadRequest, body: map[string]interface{}{"error": "malformed query"}}
	}
	return values, nil
}

type apiError struct {
	status int
	body   map[string]interface{}
}

func resolveLanguage(r *http.Request) (*languagePack, *apiError) {
	values, err := parseRequestQuery(r)
	if err != nil {
		return nil, err
	}
	languages, supplied := values["lang"]
	if !supplied {
		return languagePacks["en"], nil
	}
	if len(languages) != 1 {
		return nil, unsupportedLanguageError()
	}
	tag := strings.ToLower(languages[0])
	pack, ok := languagePacks[tag]
	if !ok {
		return nil, unsupportedLanguageError()
	}
	return pack, nil
}

func resolveDetectionLanguage(r *http.Request) (*languagePack, bool, *apiError) {
	values, err := parseRequestQuery(r)
	if err != nil {
		return nil, false, err
	}
	languages, supplied := values["lang"]
	if !supplied {
		return languagePacks["en"], false, nil
	}
	if len(languages) != 1 {
		return nil, false, unsupportedLanguageError()
	}
	tag := strings.ToLower(languages[0])
	if tag == "auto" {
		return nil, true, nil
	}
	pack, ok := languagePacks[tag]
	if !ok {
		return nil, false, unsupportedLanguageError()
	}
	return pack, false, nil
}

func unsupportedLanguageError() *apiError {
	return &apiError{status: http.StatusBadRequest, body: map[string]interface{}{
		"error": "unsupported language", "supported_languages": supportedLanguageTags,
	}}
}

func writeAPIJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, apiErr *apiError) {
	writeAPIJSON(w, apiErr.status, apiErr.body)
}

func getRequestText(r *http.Request, label, example string) (string, *apiError) {
	values, err := parseRequestQuery(r)
	if err != nil {
		return "", err
	}
	text := values.Get("text")
	if strings.TrimSpace(text) == "" {
		return "", &apiError{status: http.StatusBadRequest, body: map[string]interface{}{"error": fmt.Sprintf("Please provide %s, e.g. ?text=%s", label, example)}}
	}
	if !utf8.ValidString(text) {
		return "", &apiError{status: http.StatusBadRequest, body: map[string]interface{}{"error": "text must be valid UTF-8"}}
	}
	if len(text) > maxRequestTextBytes || countNormalizedLetters(text) > maxVocalizationInputLetters {
		return "", &apiError{status: http.StatusRequestEntityTooLarge, body: map[string]interface{}{"error": "text is too large", "max_bytes": maxRequestTextBytes, "max_letters": maxVocalizationInputLetters}}
	}
	return text, nil
}

func countNormalizedLetters(text string) int {
	count := 0
	for _, r := range cases.Fold().String(norm.NFKC.String(text)) {
		if unicode.IsLetter(r) {
			count++
		}
	}
	return count
}

func requireGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	writeAPIJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"error": "method not allowed"})
	return false
}

func serveGenerateMeow(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	start := time.Now()
	pack, apiErr := resolveLanguage(r)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	meow, family := generateForLanguage(pack)
	writeAPIJSON(w, http.StatusOK, map[string]interface{}{
		"meow": meow, "family": string(family), "language": pack.Tag,
		"generation_time": time.Since(start).String(),
	})
}

func serveDetectMeow(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	start := time.Now()
	pack, automatic, apiErr := resolveDetectionLanguage(r)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	text, apiErr := getRequestText(r, "text", "mrrp")
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	language := ""
	var closestLanguage interface{}
	var analysis vocalizationAnalysis
	if automatic {
		var matched bool
		analysis, pack, matched = analyzeAcrossLanguagePacks(text, false)
		language = "auto"
		if matched {
			language = pack.Tag
		}
		if pack != nil {
			closestLanguage = pack.Tag
		}
	} else {
		analysis = analyzeForLanguage(text, pack)
		language = pack.Tag
		if analysis.family != "unknown" {
			closestLanguage = pack.Tag
		}
	}
	writeAPIJSON(w, http.StatusOK, map[string]interface{}{
		"input": text, "squeezed_form": analysis.normalized.squeezed,
		"is_meow": analysis.isStrict, "meow_percentage": fmt.Sprintf("%.1f%%", analysis.strictScore),
		"family": string(analysis.family), "match_type": analysis.matchType,
		"language": language, "closest_language": closestLanguage,
		"detection_time": time.Since(start).String(),
	})
}

func serveDetectMeowLike(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	start := time.Now()
	pack, automatic, apiErr := resolveDetectionLanguage(r)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	text, apiErr := getRequestText(r, "text", "miao")
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	language := ""
	var closestLanguage interface{}
	var analysis vocalizationAnalysis
	if automatic {
		var matched bool
		analysis, pack, matched = analyzeAcrossLanguagePacks(text, true)
		language = "auto"
		if matched {
			language = pack.Tag
		}
		if pack != nil {
			closestLanguage = pack.Tag
		}
	} else {
		analysis = analyzeForLanguage(text, pack)
		if !analysis.isFuzzy {
			analysis = applyEmbeddedAnalysis(text, pack, analysis)
		}
		language = pack.Tag
		if analysis.family != "unknown" {
			closestLanguage = pack.Tag
		}
	}
	coverage := interface{}(nil)
	matchedText := interface{}(nil)
	if analysis.isEmbedded {
		coverage = analysis.coverage
		matchedText = analysis.matchedText
	} else if analysis.isFuzzy {
		coverage = float64(100)
	}
	writeAPIJSON(w, http.StatusOK, map[string]interface{}{
		"input": text, "squeezed_form": analysis.normalized.squeezed,
		"is_meow_like": analysis.isFuzzy, "meow_percentage": fmt.Sprintf("%.1f%%", analysis.fuzzyScore),
		"family": string(analysis.family), "match_type": analysis.matchType,
		"language": language, "closest_language": closestLanguage,
		"matched_text": matchedText, "coverage_percentage": coverage,
		"detection_time": time.Since(start).String(),
	})
}

func analyzeAcrossLanguagePacks(text string, embedded bool) (vocalizationAnalysis, *languagePack, bool) {
	var closestAnalysis vocalizationAnalysis
	var closestPack *languagePack
	closestScore := 0.0
	var matchedAnalysis vocalizationAnalysis
	var matchedPack *languagePack
	bestScore := -1.0
	for _, tag := range automaticLanguagePriority {
		pack := languagePacks[tag]
		analysis := analyzeForLanguage(text, pack)
		if embedded && !analysis.isFuzzy {
			analysis = applyEmbeddedAnalysis(text, pack, analysis)
		}
		score := analysis.strictScore
		if embedded {
			score = analysis.fuzzyScore
		}
		if score > closestScore {
			closestAnalysis, closestPack, closestScore = analysis, pack, score
		}
		matched := analysis.isFuzzy
		if !embedded {
			matched = analysis.isStrict
		}
		if matched {
			if score > bestScore {
				matchedAnalysis, matchedPack, bestScore = analysis, pack, score
			}
		}
	}
	if matchedPack != nil {
		return matchedAnalysis, matchedPack, true
	}
	if closestPack != nil {
		return closestAnalysis, closestPack, false
	}
	return analyzeForLanguage(text, languagePacks["en"]), nil, false
}

func languagesHandler(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	languages := make([]map[string]interface{}, 0, len(supportedLanguageTags))
	for _, tag := range supportedLanguageTags {
		languages = append(languages, map[string]interface{}{"tag": tag, "generate": true, "detect": true})
	}
	writeAPIJSON(w, http.StatusOK, map[string]interface{}{
		"languages": languages, "automatic_detection": true,
		"automatic_detection_parameter": "lang=auto on /ismeow and /meowlike",
	})
}

var embeddedExclusions = regexp.MustCompile("(?s)```.*?```|`[^`]*`|https?://[^\\s]+|www\\.[^\\s]+|@[A-Za-z0-9_:.!/-]+")

type textSpan struct{ start, end int }

type wordToken struct {
	start, end  int
	text        string
	letterCount int
}

func excludedSpans(text string) []textSpan {
	indices := embeddedExclusions.FindAllStringIndex(text, -1)
	spans := make([]textSpan, 0, len(indices))
	for _, index := range indices {
		spans = append(spans, textSpan{start: index[0], end: index[1]})
	}
	return spans
}

func spanIsExcluded(start, end int, spans []textSpan) bool {
	for _, span := range spans {
		if start < span.end && end > span.start {
			return true
		}
	}
	return false
}

func letterTokens(text string) []wordToken {
	tokens := make([]wordToken, 0, 16)
	start := -1
	for offset, r := range text {
		if unicode.IsLetter(r) {
			if start < 0 {
				start = offset
			}
			continue
		}
		if unicode.IsMark(r) && start >= 0 {
			continue
		}
		if start >= 0 {
			part := text[start:offset]
			tokens = append(tokens, wordToken{start: start, end: offset, text: part, letterCount: countNormalizedLetters(part)})
			start = -1
		}
	}
	if start >= 0 {
		part := text[start:]
		tokens = append(tokens, wordToken{start: start, end: len(text), text: part, letterCount: countNormalizedLetters(part)})
	}
	return tokens
}

type embeddedCandidate struct {
	family      vocalizationFamily
	text        string
	letters     int
	distance    int
	offset      int
	formOrder   int
	isSuffix    bool
}

func formAnchorOK(token string, form packForm, pack *languagePack) bool {
	if form.anchor != "" {
		return strings.HasPrefix(token, normalizePackToken(form.anchor, pack))
	}
	if pack.Tag == "en" {
		return familyAnchorOK(token, familyMatch{family: form.family, matched: true})
	}
	return true
}

func bestEmbeddedToken(token wordToken, pack *languagePack) (embeddedCandidate, bool) {
	canonical := []rune(canonicalTokenForPack(token.text, pack))
	if len(canonical) == 0 {
		return embeddedCandidate{}, false
	}
	if _, denied := pack.embeddedDenylist[string(canonical)]; denied {
		return embeddedCandidate{}, false
	}
	if pack.Tag == "en" && (isMeowEcho(string(canonical)) || isMixedMeow(string(canonical))) {
		return embeddedCandidate{family: familyMeow, text: token.text, letters: token.letterCount, offset: token.start}, true
	}
	if len(canonical) > pack.longestForm+1 {
		return embeddedCandidate{}, false
	}
	var best embeddedCandidate
	found := false
	for formIndex, form := range pack.formOrder {
		if !form.embeddedOK {
			continue
		}
		editLimit := pack.MaxTypoEdits
		if form.exactOnly || len(canonical) < 4 || len(form.normalized) < 4 {
			editLimit = 0
		}
		if absInt(len(canonical)-len(form.normalized)) > editLimit {
			continue
		}
		distance := levenshteinRunes(canonical, form.normalized)
		if distance > editLimit || !formAnchorOK(string(canonical), form, pack) {
			continue
		}
		candidate := embeddedCandidate{family: form.family, text: token.text, letters: token.letterCount, distance: distance, offset: token.start, formOrder: formIndex}
		if !found || candidate.distance < best.distance || candidate.distance == best.distance && candidate.formOrder < best.formOrder {
			best, found = candidate, true
		}
	}
	return best, found
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func kanaOnly(text string, pack *languagePack) bool {
	count := 0
	for _, r := range []rune(normalizePackToken(text, pack)) {
		if unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || r == 'ー' {
			count++
			continue
		}
		return false
	}
	return count >= 2
}

func hanOnly(text string, pack *languagePack) bool {
	count := 0
	for _, r := range []rune(normalizePackToken(text, pack)) {
		if unicode.Is(unicode.Han, r) {
			count++
			continue
		}
		return false
	}
	return count >= 2
}

func guardedSuffixCandidates(text string, tokens []wordToken, excluded []textSpan, pack *languagePack) []embeddedCandidate {
	if !pack.Boundary.allowGuardedSuffix && !pack.Boundary.allowGuardedHanSuffix {
		return nil
	}
	var results []embeddedCandidate
	for _, token := range tokens {
		if spanIsExcluded(token.start, token.end, excluded) {
			continue
		}
		runes := []rune(token.text)
		minimum := len(runes) - pack.longestForm - 2
		if minimum < 0 {
			minimum = 0
		}
		for start := minimum; start < len(runes); start++ {
			suffix := string(runes[start:])
			canonical := normalizePackToken(suffix, pack)
			for order, form := range pack.formOrder {
				kanaSuffix := pack.Boundary.allowGuardedSuffix && form.family == familyNya && kanaOnly(form.text, pack)
				hanSuffix := pack.Boundary.allowGuardedHanSuffix && (form.family == familyMeow || form.family == familyPrrr) && hanOnly(form.text, pack)
				if (!kanaSuffix && !hanSuffix) || canonical != string(form.normalized) {
					continue
				}
				if start == 0 && (!hanSuffix || canonical != normalizePackToken(token.text, pack)) {
					continue
				}
				letters := countNormalizedLetters(suffix)
				if letters == 0 || float64(letters)/float64(token.letterCount) < 0.25 {
					continue
				}
				results = append(results, embeddedCandidate{family: form.family, text: text[token.start+byteOffsetForRune(runes, start):token.end], letters: letters, offset: token.start + byteOffsetForRune(runes, start), formOrder: order, isSuffix: true})
			}
		}
	}
	return results
}

func byteOffsetForRune(runes []rune, index int) int {
	if index <= 0 {
		return 0
	}
	return len(string(runes[:index]))
}

func applyEmbeddedAnalysis(text string, pack *languagePack, whole vocalizationAnalysis) vocalizationAnalysis {
	if whole.isFuzzy {
		whole.coverage = 100
		return whole
	}
	spans := excludedSpans(text)
	tokens := letterTokens(text)
	totalLetters := countNormalizedLetters(text)
	if totalLetters == 0 {
		return whole
	}
	candidates := make([]embeddedCandidate, 0, len(tokens))
	for _, token := range tokens {
		if token.letterCount == 0 || spanIsExcluded(token.start, token.end, spans) {
			continue
		}
		if candidate, ok := bestEmbeddedToken(token, pack); ok {
			if pack.Boundary.allowGuardedHanSuffix && hanOnly(token.text, pack) {
				continue
			}
			candidates = append(candidates, candidate)
		}
	}
	candidates = append(candidates, guardedSuffixCandidates(text, tokens, spans, pack)...)
	if len(candidates) == 0 {
		return whole
	}

	matchedLetters := 0
	ordinaryMatchedLetters := 0
	suffixMatchedLetters := 0
	var strongest embeddedCandidate
	for i, candidate := range candidates {
		matchedLetters += candidate.letters
		if candidate.isSuffix {
			suffixMatchedLetters += candidate.letters
		} else {
			ordinaryMatchedLetters += candidate.letters
		}
		if i == 0 || candidate.letters > strongest.letters || candidate.letters == strongest.letters && candidate.distance < strongest.distance || candidate.letters == strongest.letters && candidate.distance == strongest.distance && candidate.offset < strongest.offset || candidate.letters == strongest.letters && candidate.distance == strongest.distance && candidate.offset == strongest.offset && candidate.formOrder < strongest.formOrder {
			strongest = candidate
		}
	}
	coverage := 100 * float64(matchedLetters) / float64(totalLetters)
	if ordinaryMatchedLetters < 4 && suffixMatchedLetters < 2 || coverage < 25 {
		return whole
	}
	whole.isFuzzy = true
	whole.isEmbedded = true
	whole.family = strongest.family
	whole.matchType = "embedded"
	whole.matchedText = strongest.text
	whole.coverage = coverage
	whole.fuzzyScore = coverage
	return whole
}
