package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
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

type dictWordCandidate struct {
	WordID string
	Score  int
}

type dictKanaRow struct {
	Text           string
	Common         int
	AppliesToKanji []string
}

type dictSenseRow struct {
	ID             int64
	PartOfSpeech   []string
	AppliesToKanji []string
	AppliesToKana  []string
}

type dictKanjiReadingCandidate struct {
	Display    string
	Normalized string
}

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

	candidate, matchedByKanji, err := bestDictionaryCandidate(dictDB, word)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, nil
	}

	reading, err := selectDictionaryReading(dictDB, candidate.WordID, word, matchedByKanji)
	if err != nil {
		return nil, err
	}
	if reading == "" {
		reading = word
	}

	partOfSpeech, glosses, err := selectDictionarySenseData(dictDB, candidate.WordID, word, reading, matchedByKanji)
	if err != nil {
		return nil, err
	}

	kanji, err := lookupDictionaryKanjiInfo(dictDB, word, reading)
	if err != nil {
		return nil, err
	}

	return &dictionaryWordInfo{
		Word:         word,
		Reading:      reading,
		PartOfSpeech: partOfSpeech,
		Meaning:      strings.Join(glosses, "; "),
		Glosses:      glosses,
		Kanji:        kanji,
	}, nil
}

func bestDictionaryCandidate(dictDB *sql.DB, word string) (*dictWordCandidate, bool, error) {
	candidates := map[string]*dictWordCandidate{}
	matchedByKanji := map[string]bool{}

	kanjiRows, err := dictDB.Query(`
		SELECT word_id, MAX(common)
		FROM jmdict_kanji
		WHERE text = ?
		GROUP BY word_id
	`, word)
	if err != nil {
		return nil, false, err
	}
	for kanjiRows.Next() {
		var wordID string
		var common int
		if err := kanjiRows.Scan(&wordID, &common); err != nil {
			kanjiRows.Close()
			return nil, false, err
		}
		candidates[wordID] = &dictWordCandidate{WordID: wordID, Score: 300 + common*10}
		matchedByKanji[wordID] = true
	}
	if err := kanjiRows.Err(); err != nil {
		kanjiRows.Close()
		return nil, false, err
	}
	kanjiRows.Close()

	kanaRows, err := dictDB.Query(`
		SELECT word_id, MAX(common)
		FROM jmdict_kana
		WHERE text = ?
		GROUP BY word_id
	`, word)
	if err != nil {
		return nil, false, err
	}
	for kanaRows.Next() {
		var wordID string
		var common int
		if err := kanaRows.Scan(&wordID, &common); err != nil {
			kanaRows.Close()
			return nil, false, err
		}
		score := 200 + common*10
		if cur, ok := candidates[wordID]; ok {
			if score > cur.Score {
				cur.Score = score
			}
			continue
		}
		candidates[wordID] = &dictWordCandidate{WordID: wordID, Score: score}
	}
	if err := kanaRows.Err(); err != nil {
		kanaRows.Close()
		return nil, false, err
	}
	kanaRows.Close()

	if len(candidates) == 0 {
		return nil, false, nil
	}

	var best *dictWordCandidate
	for _, c := range candidates {
		if best == nil || c.Score > best.Score || (c.Score == best.Score && c.WordID < best.WordID) {
			best = c
		}
	}
	return best, matchedByKanji[best.WordID], nil
}

func selectDictionaryReading(dictDB *sql.DB, wordID, word string, matchedByKanji bool) (string, error) {
	rows, err := dictDB.Query(`
		SELECT text, common, applies_to_kanji
		FROM jmdict_kana
		WHERE word_id = ?
		ORDER BY common DESC, text
	`, wordID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var best string
	bestScore := -1
	for rows.Next() {
		var row dictKanaRow
		var appliesJSON string
		if err := rows.Scan(&row.Text, &row.Common, &appliesJSON); err != nil {
			return "", err
		}
		if err := json.Unmarshal([]byte(appliesJSON), &row.AppliesToKanji); err != nil {
			row.AppliesToKanji = nil
		}
		score := row.Common * 10
		if row.Text == word {
			score += 20
		}
		if matchedByKanji && jsonListApplies(row.AppliesToKanji, word) {
			score += 50
		}
		if !matchedByKanji {
			score += 5
		}
		if score > bestScore {
			best = row.Text
			bestScore = score
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return best, nil
}

func selectDictionarySenseData(dictDB *sql.DB, wordID, word, reading string, matchedByKanji bool) (string, []string, error) {
	rows, err := dictDB.Query(`
		SELECT id, pos, applies_to_kanji, applies_to_kana
		FROM jmdict_senses
		WHERE word_id = ?
		ORDER BY id
	`, wordID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	var senses []dictSenseRow
	var fallbackSenses []dictSenseRow
	for rows.Next() {
		var row dictSenseRow
		var posJSON string
		var appliesKanjiJSON string
		var appliesKanaJSON string
		if err := rows.Scan(&row.ID, &posJSON, &appliesKanjiJSON, &appliesKanaJSON); err != nil {
			return "", nil, err
		}
		if err := json.Unmarshal([]byte(posJSON), &row.PartOfSpeech); err != nil {
			row.PartOfSpeech = nil
		}
		if err := json.Unmarshal([]byte(appliesKanjiJSON), &row.AppliesToKanji); err != nil {
			row.AppliesToKanji = nil
		}
		if err := json.Unmarshal([]byte(appliesKanaJSON), &row.AppliesToKana); err != nil {
			row.AppliesToKana = nil
		}
		fallbackSenses = append(fallbackSenses, row)
		if matchedByKanji {
			if jsonListApplies(row.AppliesToKanji, word) && jsonListApplies(row.AppliesToKana, reading) {
				senses = append(senses, row)
			}
			continue
		}
		if jsonListApplies(row.AppliesToKana, reading) {
			senses = append(senses, row)
		}
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if len(senses) == 0 {
		senses = fallbackSenses
	}

	senseIDs := make([]int64, 0, len(senses))
	var posTags []string
	for _, sense := range senses {
		senseIDs = append(senseIDs, sense.ID)
		posTags = append(posTags, sense.PartOfSpeech...)
	}
	glosses, err := loadSenseGlosses(dictDB, senseIDs)
	if err != nil {
		return "", nil, err
	}
	return canonicalPartOfSpeechFromDict(posTags), glosses, nil
}

func loadSenseGlosses(dictDB *sql.DB, senseIDs []int64) ([]string, error) {
	seen := map[string]struct{}{}
	var glosses []string
	for _, senseID := range senseIDs {
		rows, err := dictDB.Query(`
			SELECT text
			FROM jmdict_glosses
			WHERE sense_id = ?
			ORDER BY rowid
		`, senseID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var gloss string
			if err := rows.Scan(&gloss); err != nil {
				rows.Close()
				return nil, err
			}
			gloss = strings.TrimSpace(gloss)
			if gloss == "" {
				continue
			}
			if _, ok := seen[gloss]; ok {
				continue
			}
			seen[gloss] = struct{}{}
			glosses = append(glosses, gloss)
			if len(glosses) >= 3 {
				rows.Close()
				return glosses, nil
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return glosses, nil
}

func lookupDictionaryKanjiInfo(dictDB *sql.DB, word, reading string) ([]dictionaryKanjiInfo, error) {
	wordRunes := []rune(word)
	kanjiIdxs := make([]int, 0, len(wordRunes))
	for i, r := range wordRunes {
		if isDictionaryKanjiRune(r) {
			kanjiIdxs = append(kanjiIdxs, i)
		}
	}
	if len(kanjiIdxs) == 0 {
		return []dictionaryKanjiInfo{}, nil
	}

	out := make([]dictionaryKanjiInfo, 0, len(kanjiIdxs))
	candidates := make([][]dictKanjiReadingCandidate, 0, len(kanjiIdxs))
	for _, idx := range kanjiIdxs {
		info, readingCandidates, err := loadDictionaryKanjiInfo(dictDB, string(wordRunes[idx]))
		if err != nil {
			return nil, err
		}
		out = append(out, info)
		candidates = append(candidates, readingCandidates)
	}

	if inferred, ok := inferDictionaryKanjiReadings(wordRunes, kanaToHiragana(reading), kanjiIdxs, candidates); ok {
		for i := range out {
			out[i].Reading = inferred[i]
		}
	}
	return out, nil
}

func loadDictionaryKanjiInfo(dictDB *sql.DB, character string) (dictionaryKanjiInfo, []dictKanjiReadingCandidate, error) {
	info := dictionaryKanjiInfo{
		Character: character,
		Readings:  []string{},
		Meanings:  []string{},
	}

	readingRows, err := dictDB.Query(`
		SELECT value
		FROM kanjidic_readings
		WHERE literal = ?
		ORDER BY type, value
	`, character)
	if err != nil {
		return info, nil, err
	}
	defer readingRows.Close()

	var candidates []dictKanjiReadingCandidate
	seenReadings := map[string]struct{}{}
	for readingRows.Next() {
		var raw string
		if err := readingRows.Scan(&raw); err != nil {
			return info, nil, err
		}
		display := dictionaryReadingDisplayForm(raw)
		if display == "" {
			continue
		}
		if _, ok := seenReadings[display]; !ok {
			seenReadings[display] = struct{}{}
			info.Readings = append(info.Readings, display)
		}
		normalized := kanaToHiragana(display)
		if normalized != "" {
			candidates = append(candidates, dictKanjiReadingCandidate{
				Display:    display,
				Normalized: normalized,
			})
		}
	}
	if err := readingRows.Err(); err != nil {
		return info, nil, err
	}

	meaningRows, err := dictDB.Query(`
		SELECT value
		FROM kanjidic_meanings
		WHERE literal = ?
		ORDER BY value
	`, character)
	if err != nil {
		return info, nil, err
	}
	defer meaningRows.Close()

	seenMeanings := map[string]struct{}{}
	for meaningRows.Next() {
		var meaning string
		if err := meaningRows.Scan(&meaning); err != nil {
			return info, nil, err
		}
		meaning = strings.TrimSpace(meaning)
		if meaning == "" {
			continue
		}
		if _, ok := seenMeanings[meaning]; ok {
			continue
		}
		seenMeanings[meaning] = struct{}{}
		info.Meanings = append(info.Meanings, meaning)
	}
	if err := meaningRows.Err(); err != nil {
		return info, nil, err
	}

	sort.Strings(info.Meanings)
	return info, candidates, nil
}

func inferDictionaryKanjiReadings(wordRunes []rune, reading string, kanjiIdxs []int, candidates [][]dictKanjiReadingCandidate) ([]string, bool) {
	readingRunes := []rune(reading)
	memo := map[string]bool{}
	var walk func(wordPos, kanjiPos, readingPos int) ([]string, bool)
	walk = func(wordPos, kanjiPos, readingPos int) ([]string, bool) {
		key := fmt.Sprintf("%d:%d:%d", wordPos, kanjiPos, readingPos)
		if memo[key] {
			return nil, false
		}
		if wordPos == len(wordRunes) {
			if readingPos == len(readingRunes) && kanjiPos == len(kanjiIdxs) {
				return []string{}, true
			}
			memo[key] = true
			return nil, false
		}

		r := wordRunes[wordPos]
		if !isDictionaryKanjiRune(r) {
			kana := []rune(kanaToHiragana(string(r)))
			if len(kana) == 0 || readingPos+len(kana) > len(readingRunes) {
				memo[key] = true
				return nil, false
			}
			for i := range kana {
				if readingRunes[readingPos+i] != kana[i] {
					memo[key] = true
					return nil, false
				}
			}
			return walk(wordPos+1, kanjiPos, readingPos+len(kana))
		}

		if kanjiPos >= len(candidates) {
			memo[key] = true
			return nil, false
		}

		for _, candidate := range candidates[kanjiPos] {
			candidateRunes := []rune(candidate.Normalized)
			if len(candidateRunes) == 0 {
				continue
			}
			maxPrefix := len(candidateRunes)
			if remaining := len(readingRunes) - readingPos; maxPrefix > remaining {
				maxPrefix = remaining
			}
			for prefixLen := maxPrefix; prefixLen >= 1; prefixLen-- {
				matched := true
				for i := 0; i < prefixLen; i++ {
					if readingRunes[readingPos+i] != candidateRunes[i] {
						matched = false
						break
					}
				}
				if !matched {
					continue
				}
				tail, ok := walk(wordPos+1, kanjiPos+1, readingPos+prefixLen)
				if !ok {
					continue
				}
				return append([]string{firstRunes(candidate.Display, prefixLen)}, tail...), true
			}
		}

		memo[key] = true
		return nil, false
	}
	return walk(0, 0, 0)
}

func jsonListApplies(values []string, target string) bool {
	if len(values) == 0 {
		return true
	}
	for _, v := range values {
		if v == "*" || v == target {
			return true
		}
	}
	return false
}

func dictionaryReadingDisplayForm(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.IndexRune(raw, '.'); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.ReplaceAll(raw, "-", "")
	return strings.TrimSpace(raw)
}

func firstRunes(s string, count int) string {
	runes := []rune(s)
	if count >= len(runes) {
		return s
	}
	return string(runes[:count])
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

func isDictionaryKanjiRune(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

func canonicalPartOfSpeechFromDict(tags []string) string {
	lowered := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			lowered = append(lowered, tag)
		}
	}

	for _, tag := range lowered {
		if strings.Contains(tag, "ichidan verb") {
			return "ichidan-verb"
		}
	}
	for _, tag := range lowered {
		if strings.Contains(tag, "godan verb") {
			return "godan-verb"
		}
	}
	for _, tag := range lowered {
		if strings.Contains(tag, "na-adjective") || strings.Contains(tag, "adjectival noun") {
			return "na-adjective"
		}
	}
	for _, tag := range lowered {
		if strings.Contains(tag, "i-adjective") {
			return "i-adjective"
		}
	}
	for _, tag := range lowered {
		if strings.Contains(tag, "adverb") {
			return "adverb"
		}
	}
	for _, tag := range lowered {
		if strings.Contains(tag, "noun") || strings.Contains(tag, "pronoun") {
			return "noun"
		}
	}
	if len(lowered) == 0 {
		return ""
	}
	return "other"
}
