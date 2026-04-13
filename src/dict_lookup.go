package main

import (
	"database/sql"
	"errors"
	"strings"
	"sync"

	"jpvocab/internal/dictlookup"
)

type dictionaryKanjiInfo = dictlookup.KanjiInfo
type dictionaryWordInfo = dictlookup.WordInfo

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
	info, err := dictlookup.LookupWordInDB(dictDB, word)
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
	info, _, err := dictlookup.LookupKanjiInDB(dictDB, character)
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
	return dictlookup.CanonicalPartOfSpeech(lowered)
}
