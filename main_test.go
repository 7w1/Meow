package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestMeowConfidence(t *testing.T) {
	iterations := 100000

	fmt.Printf("Commencing bulk test of %d meows...\n", iterations)

	for i := 0; i < iterations; i++ {
		reqGen := httptest.NewRequest(http.MethodGet, "/meow", nil)
		recGen := httptest.NewRecorder()
		generateMeow(recGen, reqGen)

		var genResult map[string]string
		json.NewDecoder(recGen.Body).Decode(&genResult)
		generatedMeow := genResult["meow"]

		reqDetect := httptest.NewRequest(http.MethodGet, "/ismeow?text="+generatedMeow, nil)
		recDetect := httptest.NewRecorder()
		detectMeow(recDetect, reqDetect)

		var detectResult map[string]interface{}
		json.NewDecoder(recDetect.Body).Decode(&detectResult)

		isMeow := detectResult["is_meow"].(bool)
		if !isMeow {
			t.Fatalf("Failed on iteration %d: '%s' was NOT classified as a meow!", i, generatedMeow)
		}

		percStr := detectResult["meow_percentage"].(string)
		percStr = strings.TrimSuffix(percStr, "%")
		confidence, err := strconv.ParseFloat(percStr, 64)

		if err != nil {
			t.Fatalf("Failed to parse percentage: %v", err)
		}

		if confidence < 80.0 {
			t.Fatalf("Low confidence on iteration %d: '%s' scored only %.1f%%", i, generatedMeow, confidence)
		}
	}
}
