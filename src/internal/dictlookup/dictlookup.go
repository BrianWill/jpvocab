package dictlookup

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

type KanjiInfo struct {
	Character string   `json:"character"`
	Reading   string   `json:"reading,omitempty"`
	Readings  []string `json:"readings"`
	Meanings  []string `json:"meanings"`
}

type WordInfo struct {
	Word         string      `json:"word"`
	Reading      string      `json:"reading"`
	PartOfSpeech string      `json:"part_of_speech"`
	Meaning      string      `json:"meaning"`
	Glosses      []string    `json:"glosses"`
	Kanji        []KanjiInfo `json:"kanji"`
}

type kanjiReadingCandidate struct {
	Display    string
	Normalized string
}

var (
	ErrMissingWordLookupTable  = errors.New("dictionary runtime table dict_word_lookup is missing")
	ErrMissingKanjiLookupTable = errors.New("dictionary runtime table dict_kanji_lookup is missing")
)

func LookupWordInDB(dictDB *sql.DB, word string) (*WordInfo, error) {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil, nil
	}
	return lookupWordFromTable(dictDB, word)
}

func LookupKanjiInDB(dictDB *sql.DB, character string) (KanjiInfo, []kanjiReadingCandidate, error) {
	return lookupKanjiFromTable(dictDB, character)
}

func lookupWordFromTable(dictDB *sql.DB, word string) (*WordInfo, error) {
	var info WordInfo
	var glossesJSON string
	var kanjiJSON string
	err := dictDB.QueryRow(`
		SELECT word, reading, part_of_speech, meaning, glosses_json, kanji_json
		FROM dict_word_lookup
		WHERE lookup_text = ?
	`, word).Scan(
		&info.Word,
		&info.Reading,
		&info.PartOfSpeech,
		&info.Meaning,
		&glossesJSON,
		&kanjiJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if isMissingWordLookupTableErr(err) {
		return nil, ErrMissingWordLookupTable
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(glossesJSON), &info.Glosses); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(kanjiJSON), &info.Kanji); err != nil {
		return nil, err
	}
	if info.Glosses == nil {
		info.Glosses = []string{}
	}
	if info.Kanji == nil {
		info.Kanji = []KanjiInfo{}
	}
	return &info, nil
}

func lookupKanjiFromTable(dictDB *sql.DB, character string) (KanjiInfo, []kanjiReadingCandidate, error) {
	info := KanjiInfo{
		Character: strings.TrimSpace(character),
		Readings:  []string{},
		Meanings:  []string{},
	}
	if info.Character == "" {
		return info, nil, nil
	}

	var meaningsJSON string
	var readingsJSON string
	err := dictDB.QueryRow(`
		SELECT meanings_json, readings_json
		FROM dict_kanji_lookup
		WHERE literal = ?
	`, info.Character).Scan(&meaningsJSON, &readingsJSON)
	if err == sql.ErrNoRows {
		return info, nil, nil
	}
	if isMissingKanjiLookupTableErr(err) {
		return info, nil, ErrMissingKanjiLookupTable
	}
	if err != nil {
		return info, nil, err
	}
	if err := json.Unmarshal([]byte(meaningsJSON), &info.Meanings); err != nil {
		return info, nil, err
	}
	if err := json.Unmarshal([]byte(readingsJSON), &info.Readings); err != nil {
		return info, nil, err
	}
	if info.Meanings == nil {
		info.Meanings = []string{}
	}
	if info.Readings == nil {
		info.Readings = []string{}
	}

	candidates := make([]kanjiReadingCandidate, 0, len(info.Readings))
	for _, display := range info.Readings {
		normalized := kanaToHiragana(display)
		if normalized == "" {
			continue
		}
		candidates = append(candidates, kanjiReadingCandidate{
			Display:    display,
			Normalized: normalized,
		})
	}
	return info, candidates, nil
}

func isMissingWordLookupTableErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such table: dict_word_lookup")
}

func isMissingKanjiLookupTableErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such table: dict_kanji_lookup")
}

func kanaToHiragana(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x30A1 && r <= 0x30F6 {
			b.WriteRune(r - 0x60)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func CanonicalPartOfSpeech(tags []string) string {
	for _, tag := range tags {
		if tag == "v1" || strings.Contains(tag, "ichidan verb") {
			return "ichidan-verb"
		}
	}
	for _, tag := range tags {
		if strings.HasPrefix(tag, "v5") || strings.Contains(tag, "godan verb") {
			return "godan-verb"
		}
	}
	for _, tag := range tags {
		if tag == "adj-na" || strings.Contains(tag, "na-adjective") || strings.Contains(tag, "adjectival noun") {
			return "na-adjective"
		}
	}
	for _, tag := range tags {
		if tag == "adj-i" || strings.Contains(tag, "i-adjective") {
			return "i-adjective"
		}
	}
	for _, tag := range tags {
		if tag == "adv" || strings.Contains(tag, "adverb") {
			return "adverb"
		}
	}
	for _, tag := range tags {
		if tag == "n" || tag == "pn" || strings.Contains(tag, "noun") || strings.Contains(tag, "pronoun") {
			return "noun"
		}
	}
	if len(tags) == 0 {
		return ""
	}
	return "other"
}
