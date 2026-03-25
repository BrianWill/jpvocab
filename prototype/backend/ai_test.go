package main

import (
	"strings"
	"testing"
)

// --- autoFillWord ---

func TestAutoFillWord_InvalidProviderModelFormat(t *testing.T) {
	cases := []string{
		"badformat",
		"",
		"noslash",
	}
	for _, pm := range cases {
		_, err := autoFillWord("食べる", pm)
		if err == nil {
			t.Errorf("autoFillWord(%q): expected error, got nil", pm)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid ai_model value") {
			t.Errorf("autoFillWord(%q): unexpected error: %v", pm, err)
		}
	}
}
