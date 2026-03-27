package main

import (
	"strings"
	"testing"
)

var invalidProviderModels = []string{"badformat", "", "noslash"}

// --- autoFillWord ---

func TestAutoFillWord_InvalidProviderModelFormat(t *testing.T) {
	for _, pm := range invalidProviderModels {
		_, err := autoFillWord("食べる", pm)
		if err == nil {
			t.Errorf("autoFillWord(%q): expected error, got nil", pm)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid ai_model value") {
			t.Errorf("autoFillWord(%q): unexpected error: %v", pm, err)
		}
	}
}

// --- rerollMeaning ---

func TestRerollMeaning_InvalidProviderModelFormat(t *testing.T) {
	for _, pm := range invalidProviderModels {
		_, err := rerollMeaning("食べる", "to eat", pm)
		if err == nil {
			t.Errorf("rerollMeaning(%q): expected error, got nil", pm)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid ai_model value") {
			t.Errorf("rerollMeaning(%q): unexpected error: %v", pm, err)
		}
	}
}

// --- rerollExamples ---

func TestRerollExamples_InvalidProviderModelFormat(t *testing.T) {
	for _, pm := range invalidProviderModels {
		_, err := rerollExamples("食べる", pm)
		if err == nil {
			t.Errorf("rerollExamples(%q): expected error, got nil", pm)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid ai_model value") {
			t.Errorf("rerollExamples(%q): unexpected error: %v", pm, err)
		}
	}
}
