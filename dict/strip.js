#!/usr/bin/env node
// One-shot script to strip unneeded fields from jmdict-eng.json and kanjidic2-en.json.
// Run from the dict/ directory: node strip.js

const fs = require('fs');
const path = require('path');

function stripJmdict() {
  console.log('Reading jmdict-eng.json...');
  const data = JSON.parse(fs.readFileSync(path.join(__dirname, 'jmdict-eng.json'), 'utf8'));

  data.words = data.words.map(entry => {
    // Strip tags[] from each kanji spelling and kana reading
    const kanji = entry.kanji.map(k => ({ common: k.common, text: k.text }));
    const kana  = entry.kana.map(k  => ({ common: k.common, text: k.text, appliesToKanji: k.appliesToKanji }));

    // Strip rarely-useful fields from each sense
    const sense = entry.sense.map(s => ({
      partOfSpeech:   s.partOfSpeech,
      appliesToKanji: s.appliesToKanji,
      appliesToKana:  s.appliesToKana,
      field:          s.field,
      misc:           s.misc,
      info:           s.info,
      gloss:          s.gloss,
      // omitting: related, antonym, dialect, languageSource
    }));

    return { id: entry.id, kanji, kana, sense };
  });

  console.log('Writing stripped jmdict-eng.json...');
  fs.writeFileSync(path.join(__dirname, 'jmdict-eng.json'), JSON.stringify(data));
  console.log('Done.');
}

function stripKanjidic() {
  console.log('Reading kanjidic2-en.json...');
  const data = JSON.parse(fs.readFileSync(path.join(__dirname, 'kanjidic2-en.json'), 'utf8'));

  data.characters = data.characters.map(ch => {
    const rm = ch.readingMeaning;
    const readingMeaning = rm ? {
      groups: rm.groups.map(g => ({
        // Keep only on'yomi and kun'yomi; drop pinyin, korean, vietnamese, etc.
        readings: g.readings.filter(r => r.type === 'ja_on' || r.type === 'ja_kun')
                            .map(r => ({ type: r.type, value: r.value })),
        meanings: g.meanings,
      })),
      nanori: rm.nanori,
    } : null;

    return {
      literal:        ch.literal,
      misc: {
        grade:        ch.misc.grade,
        strokeCounts: ch.misc.strokeCounts,
        frequency:    ch.misc.frequency,
        jlptLevel:    ch.misc.jlptLevel,
      },
      readingMeaning,
      // omitting: codepoints, radicals, dictionaryReferences, queryCodes
    };
  });

  console.log('Writing stripped kanjidic2-en.json...');
  fs.writeFileSync(path.join(__dirname, 'kanjidic2-en.json'), JSON.stringify(data));
  console.log('Done.');
}

stripJmdict();
stripKanjidic();
