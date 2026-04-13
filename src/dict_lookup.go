package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

type dictionaryKanjiInfo struct {
	Character string   `json:"character"`
	Reading   string   `json:"reading,omitempty"`
	Readings  []string `json:"readings"`
	Meanings  []string `json:"meanings"`
}

type dictionaryWordInfo struct {
	Word         string                `json:"word"`
	Reading      string                `json:"reading"`
	PartOfSpeech string                `json:"part_of_speech"`
	Meaning      string                `json:"meaning"`
	Glosses      []string              `json:"glosses"`
	Kanji        []dictionaryKanjiInfo `json:"kanji"`
}

type kanjiReadingCandidate struct {
	Display    string
	Normalized string
}

var (
	errMissingWordLookupTable  = errors.New("dictionary runtime table dict_word_lookup is missing")
	errMissingKanjiLookupTable = errors.New("dictionary runtime table dict_kanji_lookup is missing")
)

var dictLookupCache sync.Map

func lookupDictionaryWord(word string) (*dictionaryWordInfo, error) {
	if !dictIsReady() {
		return nil, errors.New("dictionary not ready")
	}
	db, err := openDictDB()
	if err != nil {
		return nil, err
	}
	return lookupDictionaryWordInDB(db, strings.TrimSpace(word))
}

func lookupDictionaryWordInDB(dictDB *sql.DB, word string) (*dictionaryWordInfo, error) {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil, nil
	}
	if cached, ok := dictLookupCache.Load(word); ok {
		if cached == nil {
			return nil, nil
		}
		return cached.(*dictionaryWordInfo), nil
	}
	info, err := lookupWordFromTable(dictDB, word)
	if err != nil {
		return nil, err
	}
	if info == nil {
		dictLookupCache.Store(word, nil)
		return nil, nil
	}
	dictLookupCache.Store(word, info)
	return info, nil
}

func lookupDictionaryKanji(character string) (*dictionaryKanjiInfo, error) {
	if !dictIsReady() {
		return nil, errors.New("dictionary not ready")
	}
	db, err := openDictDB()
	if err != nil {
		return nil, err
	}
	return lookupDictionaryKanjiInDB(db, strings.TrimSpace(character))
}

func lookupDictionaryKanjiInDB(dictDB *sql.DB, character string) (*dictionaryKanjiInfo, error) {
	character = strings.TrimSpace(character)
	if character == "" {
		return nil, nil
	}
	info, _, err := lookupKanjiFromTable(dictDB, character)
	if err != nil {
		return nil, err
	}
	if info.Character == "" {
		return nil, nil
	}
	return &info, nil
}

func canonicalPartOfSpeechFromDict(tags []string) string {
	lowered := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			lowered = append(lowered, tag)
		}
	}
	return canonicalPartOfSpeech(lowered)
}

func lookupWordFromTable(dictDB *sql.DB, word string) (*dictionaryWordInfo, error) {
	var info dictionaryWordInfo
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
		return nil, errMissingWordLookupTable
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
		info.Kanji = []dictionaryKanjiInfo{}
	}
	return &info, nil
}

func lookupKanjiFromTable(dictDB *sql.DB, character string) (dictionaryKanjiInfo, []kanjiReadingCandidate, error) {
	info := dictionaryKanjiInfo{
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
		return info, nil, errMissingKanjiLookupTable
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

func canonicalPartOfSpeech(tags []string) string {
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
