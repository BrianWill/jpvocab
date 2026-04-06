package main

import (
	"testing"
)

// --- parseWordList ---

func TestParseWordList(t *testing.T) {
	cases := []struct {
		desc  string
		input string
		want  []string
	}{
		{
			desc:  "newline-separated (typical paste)",
			input: "食べる\n飲む\n走る",
			want:  []string{"食べる", "飲む", "走る"},
		},
		{
			desc:  "comma-separated",
			input: "猫,犬,鳥",
			want:  []string{"猫", "犬", "鳥"},
		},
		{
			desc:  "duplicates removed, first-seen order preserved",
			input: "猫\n犬\n猫",
			want:  []string{"猫", "犬"},
		},
		{
			desc:  "mixed delimiters",
			input: "一,二 三\t四\n五",
			want:  []string{"一", "二", "三", "四", "五"},
		},
		{
			desc:  "empty string",
			input: "",
			want:  []string{},
		},
		{
			desc:  "whitespace only",
			input: "  \n\t  ",
			want:  []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := parseWordList(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
