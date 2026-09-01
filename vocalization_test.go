package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMeowDetectionCorpus(t *testing.T) {
	shouldBeMeow := []string{
		"meow",
		"mew",
		"mrow",
		"mreow",
		"mroe",
		"mwor",
		"mweo",
		"miao",
		"miau",
		"rawr",
		"meox",
		"nyam",
		"nya",
		"nyan",
		"nyaaaann",
		"prr",
		"prrr",
		"prrrrr",
		"purr",
		"mrrp",
		"mrrrrpp",
		"m.e.e.o.w",
		"nya!!!",
		"p.u.r.r",
	}

	for _, input := range shouldBeMeow {
		analysis := analyzeVocalization(input)
		if !analysis.isStrict {
			t.Errorf("expected %q to be strict meow, got family=%q score=%.1f squeezed=%q", input, analysis.family, analysis.strictScore, analysis.normalized.squeezed)
		}
		if !analysis.isFuzzy {
			t.Errorf("expected %q to be fuzzy meow, got family=%q score=%.1f squeezed=%q", input, analysis.family, analysis.fuzzyScore, analysis.normalized.squeezed)
		}
	}
}

func TestOrdinaryWordDetectionCorpus(t *testing.T) {
	shouldNotBeMeow := []string{
		"poop",
		"poor",
		"people",
		"power",
		"poppy",
		"pop",
		"peep",
		"pip",
		"pap",
		"pup",
		"prep",
		"prop",
		"proper",
		"pure",
		"pump",
		"paper",
		"map",
		"mop",
		"men",
		"mix",
		"mud",
		"moon",
		"more",
		"move",
		"merry",
		"meat",
		"meowing",
		"nylon",
		"nyet",
		"near",
		"now",
		"new",
		"nap",
		"not",
		"pan",
		"pin",
		"pun",
		"penny",
		"never",
		"pen",
		"cat",
		"dog",
		"hiss",
		"hello",
		"purrfect",
	}

	for _, input := range shouldNotBeMeow {
		analysis := analyzeVocalization(input)
		if analysis.isStrict || analysis.isFuzzy {
			t.Errorf("expected ordinary word %q to be rejected, got strict=%t fuzzy=%t family=%q strict_score=%.1f fuzzy_score=%.1f squeezed=%q", input, analysis.isStrict, analysis.isFuzzy, analysis.family, analysis.strictScore, analysis.fuzzyScore, analysis.normalized.squeezed)
		}
	}
}

func TestLongMeowSpamAndTypos(t *testing.T) {
	longMeow := "mreowmroemwormeowmreowmroemwormweo"
	analysis := analyzeVocalization(longMeow)
	if !analysis.isStrict || !analysis.isFuzzy {
		t.Fatalf("expected long meow spam to pass both detectors, got strict=%t fuzzy=%t family=%q strict_score=%.1f fuzzy_score=%.1f", analysis.isStrict, analysis.isFuzzy, analysis.family, analysis.strictScore, analysis.fuzzyScore)
	}
	if analysis.family != familyMeow {
		t.Fatalf("expected long meow spam to use the meow family, got %q", analysis.family)
	}

	longWithOneTypo := "mreoxmroemwormeowmreowmroemwormweo"
	analysis = analyzeVocalization(longWithOneTypo)
	if !analysis.isStrict || !analysis.isFuzzy {
		t.Fatalf("expected one typo in long meow spam to pass both detectors, got strict=%t fuzzy=%t strict_score=%.1f fuzzy_score=%.1f", analysis.isStrict, analysis.isFuzzy, analysis.strictScore, analysis.fuzzyScore)
	}

	longWithSeveralTypos := "mreoxmroemwqrmeowmreowmreemwormwep"
	analysis = analyzeVocalization(longWithSeveralTypos)
	if analysis.isStrict {
		t.Fatalf("expected several typos to fall out of strict detection, got score=%.1f", analysis.strictScore)
	}
	if !analysis.isFuzzy {
		t.Fatalf("expected several typos in long meow spam to remain fuzzy-detectable, got score=%.1f", analysis.fuzzyScore)
	}
}

func TestGeneratedVocalizationsStayInTheirFamilies(t *testing.T) {
	seenFamilies := make(map[vocalizationFamily]bool)

	for _, family := range generationFamilies {
		for _, variant := range family.variants {
			analysis := analyzeVocalization(variant)
			if !analysis.isStrict || analysis.family != family.family {
				t.Errorf("generator variant %q did not match its family %q: strict=%t detected_family=%q score=%.1f", variant, family.family, analysis.isStrict, analysis.family, analysis.strictScore)
			}
		}
	}

	for i := 0; i < 20000; i++ {
		generated, expectedFamily := generateVocalization()
		analysis := analyzeVocalization(generated)
		seenFamilies[expectedFamily] = true
		if !analysis.isStrict {
			t.Fatalf("generated %q was not strict meow: family=%q score=%.1f squeezed=%q", generated, analysis.family, analysis.strictScore, analysis.normalized.squeezed)
		}
		if analysis.family != expectedFamily {
			t.Fatalf("generated %q was classified as %q instead of %q", generated, analysis.family, expectedFamily)
		}
	}

	for _, family := range []vocalizationFamily{familyMeow, familyNya, familyPrrr} {
		if !seenFamilies[family] {
			t.Errorf("random generation did not produce family %q", family)
		}
	}
}

func TestDetectionHandlerIncludesCompatibilityAndDiagnostics(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/ismeow?text=poop", nil)
	recorder := httptest.NewRecorder()
	detectMeow(recorder, request)

	var response map[string]interface{}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode detection response: %v", err)
	}

	if response["is_meow"] != false {
		t.Fatalf("expected poop to be rejected by the HTTP detector: %#v", response)
	}
	for _, field := range []string{"input", "squeezed_form", "is_meow", "meow_percentage", "family", "match_type", "detection_time"} {
		if _, ok := response[field]; !ok {
			t.Errorf("detection response is missing %q: %#v", field, response)
		}
	}
}
