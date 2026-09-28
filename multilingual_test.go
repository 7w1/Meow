package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestLanguagePacksRoundTripGeneratedSounds(t *testing.T) {
	for _, tag := range supportedLanguageTags {
		pack := languagePacks[strings.ToLower(tag)]
		for i := 0; i < 1000; i++ {
			generated, family := generateForLanguage(pack)
			analysis := analyzeForLanguage(generated, pack)
			if !analysis.isStrict || analysis.family != family {
				t.Fatalf("%s generated %q did not strictly round-trip as %q: %#v", tag, generated, family, analysis)
			}
		}
	}
}

func TestLanguageWholeInputRecognition(t *testing.T) {
	tests := []struct {
		name     string
		language string
		input    string
		strict   bool
		fuzzy    bool
	}{
		{name: "legacy English", language: "en", input: "meow", strict: true, fuzzy: true},
		{name: "English echoed meow", language: "en", input: "meeeeooowowwwwww", strict: true, fuzzy: true},
		{name: "legacy punctuation", language: "en", input: "m.e.e.o.w", strict: true, fuzzy: true},
		{name: "French miaou", language: "fr", input: "miaou", strict: true, fuzzy: true},
		{name: "Japanese hiragana", language: "ja", input: "にゃー", strict: true, fuzzy: true},
		{name: "Japanese width folding", language: "ja", input: "ﾆｬｰ", strict: true, fuzzy: true},
		{name: "Turkish meow", language: "tr", input: "miyav", strict: true, fuzzy: true},
		{name: "Turkish purr", language: "tr", input: "mır-mır", strict: true, fuzzy: true},
		{name: "Russian meow", language: "ru", input: "мяу", strict: true, fuzzy: true},
		{name: "Russian purr", language: "ru", input: "мур-мур", strict: true, fuzzy: true},
		{name: "Ukrainian meow", language: "uk", input: "няв-няв", strict: true, fuzzy: true},
		{name: "Ukrainian purr", language: "uk", input: "мур-мур", strict: true, fuzzy: true},
		{name: "Simplified Chinese short form", language: "zh-hans", input: "喵", strict: true, fuzzy: true},
		{name: "Traditional Chinese repeated form", language: "zh-hant", input: "喵喵", strict: true, fuzzy: true},
		{name: "Spanish historical spelling", language: "es", input: "meau", strict: true, fuzzy: true},
		{name: "Spanish compound", language: "es", input: "marramiau", strict: true, fuzzy: true},
		{name: "Spanish purr", language: "es", input: "rrr", strict: true, fuzzy: true},
		{name: "Dutch miauw", language: "nl", input: "miauw", strict: true, fuzzy: true},
		{name: "Dutch mauw", language: "nl", input: "mauw", strict: true, fuzzy: true},
		{name: "German miau", language: "de", input: "miau", strict: true, fuzzy: true},
		{name: "German purr", language: "de", input: "schnurr", strict: true, fuzzy: true},
		{name: "Armenian meow", language: "hy", input: "մյաու", strict: true, fuzzy: true},
		{name: "Armenian purr", language: "hy", input: "մըռռ", strict: true, fuzzy: true},
		{name: "other pack does not see English", language: "fr", input: "meow", strict: false, fuzzy: false},
		{name: "digits are not part of a new-pack sound", language: "fr", input: "miaou1", strict: false, fuzzy: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := analyzeForLanguage(test.input, languagePacks[strings.ToLower(test.language)])
			if analysis.isStrict != test.strict || analysis.isFuzzy != test.fuzzy {
				t.Fatalf("unexpected verdict for %q in %s: strict=%t fuzzy=%t scores=%.1f/%.1f", test.input, test.language, analysis.isStrict, analysis.isFuzzy, analysis.strictScore, analysis.fuzzyScore)
			}
		})
	}
}

func TestResearchBackedVariantsStayInTheirSoundFamily(t *testing.T) {
	tests := []struct {
		language string
		input    string
		family   vocalizationFamily
	}{
		{language: "tr", input: "miyav-miyav", family: familyMeow},
		{language: "tr", input: "mır-mır", family: familyPrrr},
		{language: "ru", input: "мяу-мяу", family: familyMeow},
		{language: "ru", input: "мур-мур", family: familyPrrr},
		{language: "uk", input: "няв-няв", family: familyMeow},
		{language: "uk", input: "мур-мур", family: familyPrrr},
		{language: "zh-hans", input: "喵喵", family: familyMeow},
		{language: "es", input: "marramiau", family: familyMeow},
		{language: "es", input: "rrr", family: familyPrrr},
		{language: "nl", input: "miauw", family: familyMeow},
		{language: "nl", input: "mauw", family: familyMeow},
		{language: "de", input: "miau", family: familyMeow},
		{language: "de", input: "schnurr", family: familyPrrr},
		{language: "hy", input: "մյաու-մյաու", family: familyMeow},
		{language: "hy", input: "մըռռ", family: familyPrrr},
	}
	for _, test := range tests {
		t.Run(test.language+"/"+test.input, func(t *testing.T) {
			analysis := analyzeForLanguage(test.input, languagePacks[test.language])
			if !analysis.isStrict || analysis.family != test.family {
				t.Fatalf("%q in %s classified as strict=%t family=%q, want %q", test.input, test.language, analysis.isStrict, analysis.family, test.family)
			}
		})
	}
}

func TestChineseHanSuffixAndCanonicalLanguageTags(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/ismeow?text=%E5%96%B5%E5%96%B5&lang=ZH-hAnT", nil)
	recorder := httptest.NewRecorder()
	detectMeow(recorder, request)
	response := decodeJSONResponse(t, recorder)
	if recorder.Code != http.StatusOK || response["language"] != "zh-Hant" || response["is_meow"] != true {
		t.Fatalf("Chinese canonical tag/form did not resolve: status=%d response=%#v", recorder.Code, response)
	}

	text := "喜欢猫喵喵"
	analysis := applyEmbeddedAnalysis(text, languagePacks["zh-hans"], analyzePackWhole(text, languagePacks["zh-hans"]))
	if analysis.isStrict || !analysis.isEmbedded || analysis.matchedText != "喵喵" {
		t.Fatalf("expected exact guarded Han suffix, got %#v", analysis)
	}

	for _, incidental := range []string{"你好喵", "汉喵字", "abc喵"} {
		analysis := applyEmbeddedAnalysis(incidental, languagePacks["zh-hans"], analyzePackWhole(incidental, languagePacks["zh-hans"]))
		if analysis.isFuzzy {
			t.Errorf("unqualified Chinese substring %q matched: %#v", incidental, analysis)
		}
	}
}

func TestEmbeddedMeowLikeCoverageAndBoundaries(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "You’re cute meow", want: true},
		{input: "You’re cute meeeeooowowwwwww", want: true},
		{input: "You are cute meow", want: true},
		{input: "meow meow you", want: true},
		{input: "This is a long unrelated sentence with a single meow at the very end", want: false},
		{input: "meowing", want: true},
		{input: "meows", want: true},
		{input: "purrfect", want: true},
		{input: "somewhere", want: false},
		{input: "You're cute now", want: false},
		{input: "cute \x60meow\x60", want: false},
		{input: "cute https://example.com/meow", want: false},
		{input: "🙂 123 ...", want: false},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			whole := analyzeVocalization(test.input)
			analysis := applyEmbeddedAnalysis(test.input, languagePacks["en"], whole)
			if analysis.isFuzzy != test.want {
				t.Fatalf("input %q got fuzzy=%t, score=%.1f match=%q", test.input, analysis.isFuzzy, analysis.fuzzyScore, analysis.matchedText)
			}
		})
	}

	analysis := applyEmbeddedAnalysis("You’re cute meow", languagePacks["en"], analyzeVocalization("You’re cute meow"))
	if analysis.isStrict || !analysis.isEmbedded || analysis.matchedText != "meow" {
		t.Fatalf("unexpected embedded analysis: %#v", analysis)
	}
	if math.Abs(analysis.coverage-30.7692307692) > 0.1 || analysis.matchType != "embedded" {
		t.Fatalf("unexpected coverage or match type: coverage=%.3f type=%q", analysis.coverage, analysis.matchType)
	}

	japanese := applyEmbeddedAnalysis("かわいいにゃ", languagePacks["ja"], analyzePackWhole("かわいいにゃ", languagePacks["ja"]))
	if japanese.isStrict || !japanese.isEmbedded || japanese.matchedText != "にゃ" || math.Abs(japanese.coverage-100.0*2.0/6.0) > 0.1 {
		t.Fatalf("guarded Japanese suffix did not match as expected: %#v", japanese)
	}

	chineseText := "喜欢猫喵喵"
	chineseWhole := analyzePackWhole(chineseText, languagePacks["zh-hans"])
	chinese := applyEmbeddedAnalysis(chineseText, languagePacks["zh-hans"], chineseWhole)
	if chinese.isStrict || !chinese.isEmbedded || chinese.matchedText != "喵喵" || math.Abs(chinese.coverage-40) > 0.1 {
		t.Fatalf("guarded Chinese suffix did not match as expected: %#v", chinese)
	}
	chineseShort := applyEmbeddedAnalysis("你好喵", languagePacks["zh-hans"], analyzePackWhole("你好喵", languagePacks["zh-hans"]))
	if chineseShort.isFuzzy {
		t.Fatalf("single-character Chinese suffix should not match: %#v", chineseShort)
	}
}

func TestEnglishWordVariantsAreFuzzyOnly(t *testing.T) {
	for _, input := range []string{"meows", "meowing", "purrfect"} {
		strict := httptest.NewRecorder()
		fuzzy := httptest.NewRecorder()
		requestURL := "?text=" + url.QueryEscape(input)
		detectMeow(strict, httptest.NewRequest(http.MethodGet, "/ismeow"+requestURL, nil))
		detectMeowLike(fuzzy, httptest.NewRequest(http.MethodGet, "/meowlike"+requestURL, nil))
		if decodeJSONResponse(t, strict)["is_meow"] != false {
			t.Errorf("%q passed strict detection", input)
		}
		response := decodeJSONResponse(t, fuzzy)
		if response["is_meow_like"] != true || response["matched_text"] != input {
			t.Errorf("%q was not recognized as a fuzzy word variant: %#v", input, response)
		}
	}
}

func decodeJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var response map[string]interface{}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func TestLanguageQueryValidationOnMeowEndpoints(t *testing.T) {
	handlers := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "meow", handler: generateMeow},
		{name: "ismeow", handler: detectMeow},
		{name: "meowlike", handler: detectMeowLike},
	}
	for _, endpoint := range handlers {
		for _, suffix := range []string{"?lang=xx", "?lang=", "?lang=fr&lang=ja"} {
			t.Run(endpoint.name+suffix, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				endpoint.handler(recorder, httptest.NewRequest(http.MethodGet, "/"+endpoint.name+suffix, nil))
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
				}
				response := decodeJSONResponse(t, recorder)
				if response["error"] != "unsupported language" {
					t.Fatalf("unexpected error response: %#v", response)
				}
			})
		}
	}
}

func TestAutomaticDetectionFindsLanguageAndKeepsAmbiguousFormsStable(t *testing.T) {
	tests := []struct {
		input    string
		language string
	}{
		{input: "miaou", language: "fr"},
		{input: "にゃー", language: "ja"},
		{input: "miyav", language: "tr"},
		{input: "мяу", language: "ru"},
		{input: "няв", language: "uk"},
		{input: "喵喵", language: "zh-Hans"},
		{input: "marramiau", language: "es"},
		{input: "miauw", language: "nl"},
		{input: "maunz", language: "de"},
		{input: "մյաու", language: "hy"},
		{input: "miau", language: "de"},
	}
	for _, test := range tests {
		for _, endpoint := range []struct {
			path string
			fn   http.HandlerFunc
		}{
			{path: "/ismeow", fn: detectMeow},
			{path: "/meowlike", fn: detectMeowLike},
		} {
			t.Run(endpoint.path+"/"+test.input, func(t *testing.T) {
				requestURL := endpoint.path + "?text=" + url.QueryEscape(test.input) + "&lang=auto"
				recorder := httptest.NewRecorder()
				endpoint.fn(recorder, httptest.NewRequest(http.MethodGet, requestURL, nil))
				response := decodeJSONResponse(t, recorder)
				if recorder.Code != http.StatusOK || response["language"] != test.language {
					t.Fatalf("automatic detection returned status=%d language=%v for %q", recorder.Code, response["language"], test.input)
				}
				if endpoint.path == "/ismeow" && response["is_meow"] != true {
					t.Fatalf("strict auto detection missed %q: %#v", test.input, response)
				}
				if endpoint.path == "/meowlike" && response["is_meow_like"] != true {
					t.Fatalf("fuzzy auto detection missed %q: %#v", test.input, response)
				}
			})
		}
	}

	recorder := httptest.NewRecorder()
	generateMeow(recorder, httptest.NewRequest(http.MethodGet, "/meow?lang=auto", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("/meow accepted detection-only lang=auto: %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	detectMeowLike(recorder, httptest.NewRequest(http.MethodGet, "/meowlike?text=ordinary+words&lang=auto", nil))
	response := decodeJSONResponse(t, recorder)
	if recorder.Code != http.StatusOK || response["language"] != "auto" || response["is_meow_like"] != false {
		t.Fatalf("unexpected no-match auto response: status=%d response=%#v", recorder.Code, response)
	}
}

func TestDetectionHTTPFieldsAndLimits(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/meowlike?text=You%E2%80%99re+cute+meow", nil)
	recorder := httptest.NewRecorder()
	detectMeowLike(recorder, request)
	response := decodeJSONResponse(t, recorder)
	if recorder.Code != http.StatusOK || response["language"] != "en" || response["is_meow_like"] != true {
		t.Fatalf("unexpected embedded HTTP response: status=%d response=%#v", recorder.Code, response)
	}
	if response["matched_text"] != "meow" || response["match_type"] != "embedded" {
		t.Fatalf("missing embedded diagnostics: %#v", response)
	}
	if math.Abs(response["coverage_percentage"].(float64)-30.7692307692) > 0.1 {
		t.Fatalf("unexpected HTTP coverage: %#v", response["coverage_percentage"])
	}

	for _, target := range []struct {
		path string
		fn   http.HandlerFunc
	}{
		{path: "/ismeow", fn: detectMeow},
		{path: "/meowlike", fn: detectMeowLike},
	} {
		recorder := httptest.NewRecorder()
		target.fn(recorder, httptest.NewRequest(http.MethodGet, target.path+"?text="+strings.Repeat("a", 513), nil))
		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s accepted oversized text: status=%d", target.path, recorder.Code)
		}
	}

	for _, query := range []string{"?text=", "?text=%20", "?text=%FF", "?text=" + strings.Repeat("!", maxRequestTextBytes+1)} {
		recorder := httptest.NewRecorder()
		detectMeowLike(recorder, httptest.NewRequest(http.MethodGet, "/meowlike"+query, nil))
		if recorder.Code == http.StatusOK {
			t.Fatalf("invalid or oversized query was accepted: %q", query[:minInt(len(query), 40)])
		}
	}

	recorder = httptest.NewRecorder()
	detectMeow(recorder, httptest.NewRequest(http.MethodPost, "/ismeow?text=meow", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("unexpected method handling: status=%d allow=%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}

func TestAskMeowRemainsEnglishOnly(t *testing.T) {
	recorder := httptest.NewRecorder()
	askMeow(recorder, httptest.NewRequest(http.MethodGet, "/askmeow?text=Is+it+time+for+food", nil))
	response := decodeJSONResponse(t, recorder)
	if response["answer"] != "You may rely on meow." {
		t.Fatalf("English 8-ball fixture changed: got %q", response["answer"])
	}
	if _, ok := response["language"]; ok {
		t.Fatal("8-ball response advertises a language")
	}
	for _, tag := range []string{"en", "fr", "auto"} {
		recorder = httptest.NewRecorder()
		askMeow(recorder, httptest.NewRequest(http.MethodGet, "/askmeow?text=hello&lang="+tag, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("8-ball accepted lang=%s: %d", tag, recorder.Code)
		}
	}
}

func TestConcurrentLanguageDetection(t *testing.T) {
	const workers = 24
	var group sync.WaitGroup
	errors := make(chan string, workers)
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			analysis := applyEmbeddedAnalysis("You’re cute meow", languagePacks["en"], analyzeVocalization("You’re cute meow"))
			if !analysis.isEmbedded || analysis.matchedText != "meow" {
				errors <- "concurrent English detection returned a different result"
			}
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
