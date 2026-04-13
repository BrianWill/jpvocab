import { test } from 'node:test';
import assert from 'node:assert/strict';
import { esc, timeAgo, getSortedWords, renderReading } from '../lexicon-utils.js';

// ---------------------------------------------------------------------------
// esc
// ---------------------------------------------------------------------------

test('esc: passthrough — no special chars', () => {
  assert.equal(esc('hello'), 'hello');
});

test('esc: ampersand escape', () => {
  assert.equal(esc('a & b'), 'a &amp; b');
});

test('esc: less-than escape', () => {
  assert.equal(esc('a < b'), 'a &lt; b');
});

test('esc: greater-than escape', () => {
  assert.equal(esc('a > b'), 'a &gt; b');
});

test('esc: combined — XSS-relevant', () => {
  assert.equal(esc('<script>'), '&lt;script&gt;');
});

test('esc: multiple replacements in one string', () => {
  assert.equal(esc('a & <b>'), 'a &amp; &lt;b&gt;');
});

test('esc: non-string input coerced via String()', () => {
  assert.equal(esc(42), '42');
});

test('esc: empty string', () => {
  assert.equal(esc(''), '');
});

test('esc: does not escape double-quote (current behaviour)', () => {
  assert.equal(esc('"quoted"'), '"quoted"');
});

// ---------------------------------------------------------------------------
// timeAgo
// ---------------------------------------------------------------------------

function msAgo(ms) { return new Date(Date.now() - ms).toISOString(); }

test('timeAgo: less than 1 minute → just now', () => {
  assert.equal(timeAgo(msAgo(30_000)), 'just now');
});

test('timeAgo: exactly 1 minute', () => {
  assert.equal(timeAgo(msAgo(61_000)), '1 minute ago');
});

test('timeAgo: plural minutes', () => {
  assert.equal(timeAgo(msAgo(5 * 60_000 + 1000)), '5 minutes ago');
});

test('timeAgo: exactly 1 hour', () => {
  assert.equal(timeAgo(msAgo(61 * 60_000)), '1 hour ago');
});

test('timeAgo: plural hours', () => {
  assert.equal(timeAgo(msAgo(3 * 3_600_000 + 1000)), '3 hours ago');
});

test('timeAgo: exactly 1 day', () => {
  assert.equal(timeAgo(msAgo(25 * 3_600_000)), '1 day ago');
});

test('timeAgo: plural days', () => {
  assert.equal(timeAgo(msAgo(15 * 86_400_000)), '15 days ago');
});

test('timeAgo: exactly 1 month', () => {
  assert.equal(timeAgo(msAgo(31 * 86_400_000)), '1 month ago');
});

test('timeAgo: plural months', () => {
  assert.equal(timeAgo(msAgo(6 * 30 * 86_400_000)), '6 months ago');
});

test('timeAgo: exactly 1 year', () => {
  assert.equal(timeAgo(msAgo(366 * 86_400_000)), '1 year ago');
});

test('timeAgo: plural years', () => {
  assert.equal(timeAgo(msAgo(2 * 365 * 86_400_000)), '2 years ago');
});

// ---------------------------------------------------------------------------
// getSortedWords
// ---------------------------------------------------------------------------

const words = [
  { word: 'A', type: 'noun',        correct: 3, incorrect: 1, target: 8,
    createdAt: '2024-01-01T00:00:00Z', lastDrilled: '2024-03-01T00:00:00Z' },
  { word: 'B', type: 'godan-verb',  correct: 1, incorrect: 0, target: 5,
    createdAt: '2024-02-01T00:00:00Z', lastDrilled: null },
  { word: 'C', type: 'noun',        correct: 1, incorrect: 2, target: 8,
    createdAt: '2024-03-01T00:00:00Z', lastDrilled: '2024-02-01T00:00:00Z' },
  { word: 'D', type: 'i-adjective', correct: 5, incorrect: 0, target: 5,
    createdAt: '2024-01-15T00:00:00Z', lastDrilled: '2024-03-10T00:00:00Z' },
];

function wordNames(arr) { return arr.map(w => w.word); }

// --- sort by added ---

test('getSortedWords: added asc — oldest createdAt first', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'added', 'asc')), ['A', 'D', 'B', 'C']);
});

test('getSortedWords: added desc — newest createdAt first', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'added', 'desc')), ['C', 'B', 'D', 'A']);
});

// --- sort by drilled ---

test('getSortedWords: drilled asc — null lastDrilled sorts first', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'drilled', 'asc')), ['B', 'C', 'A', 'D']);
});

test('getSortedWords: drilled desc — null lastDrilled sorts last', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'drilled', 'desc')), ['D', 'A', 'C', 'B']);
});

// --- sort by correct ---

test('getSortedWords: correct asc — tie on correct=1 broken by createdAt desc (C then B)', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'correct', 'asc')), ['C', 'B', 'A', 'D']);
});

test('getSortedWords: correct desc', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'correct', 'desc')), ['D', 'A', 'C', 'B']);
});

// --- sort by incorrect ---

test('getSortedWords: incorrect asc — tie on incorrect=0 broken by createdAt desc (B then D, B is newer)', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'incorrect', 'asc')), ['B', 'D', 'A', 'C']);
});

test('getSortedWords: incorrect desc', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'incorrect', 'desc')), ['C', 'A', 'B', 'D']);
});

// --- sort by target ---

test('getSortedWords: target asc — ties broken by createdAt desc (B newer than D, C newer than A)', () => {
  // B and D tie on 5 (B created Feb, D Jan → B first); A and C tie on 8 (C created Mar, A Jan → C first)
  assert.deepEqual(wordNames(getSortedWords(words, 'target', 'asc')), ['B', 'D', 'C', 'A']);
});

test('getSortedWords: target desc — ties broken by createdAt desc', () => {
  // A and C tie on 8 (C created Mar, A Jan → C first); B and D tie on 5 (B created Feb, D Jan → B first)
  assert.deepEqual(wordNames(getSortedWords(words, 'target', 'desc')), ['C', 'A', 'B', 'D']);
});

// --- sort by type ---

test('getSortedWords: type asc — alphabetical (godan-verb < i-adjective < noun); within noun most-recently drilled first', () => {
  // 'g' < 'i' < 'n' → B (godan-verb), D (i-adjective), then nouns A/C
  // within noun: A.lastDrilled=Mar, C.lastDrilled=Feb → A before C
  assert.deepEqual(wordNames(getSortedWords(words, 'type', 'asc')), ['B', 'D', 'A', 'C']);
});

test('getSortedWords: type desc — dir is ignored by the type branch, result same as asc', () => {
  // type sort uses a.type vs b.type without the asc flag, so dir has no effect on inter-type order
  assert.deepEqual(wordNames(getSortedWords(words, 'type', 'desc')), ['C', 'A', 'D', 'B']);
});

// --- unknown sort key ---

test('getSortedWords: unknown key preserves original order', () => {
  assert.deepEqual(wordNames(getSortedWords(words, 'unknown', 'asc')), ['A', 'B', 'C', 'D']);
});

// --- input not mutated ---

test('getSortedWords: does not mutate the input array', () => {
  const original = words.map(w => w.word);
  getSortedWords(words, 'correct', 'asc');
  assert.deepEqual(words.map(w => w.word), original);
});

test('getSortedWords: reading handles missing reading and missing createdAt fields', () => {
  const rows = [
    { word: 'A', reading: 'ねこ', createdAt: '2024-01-01T00:00:00Z' },
    { word: 'B', createdAt: '2024-02-01T00:00:00Z' },
    { word: 'C', reading: 'ねこ' },
  ];
  assert.deepEqual(wordNames(getSortedWords(rows, 'reading', 'asc')), ['B', 'A', 'C']);
});

test('getSortedWords: type sort remains stable across unexpected types and missing lastDrilled', () => {
  const rows = [
    { word: 'A', type: 'zzz', lastDrilled: null },
    { word: 'B', type: 'aaa', lastDrilled: '2024-01-01T00:00:00Z' },
    { word: 'C', type: 'aaa', lastDrilled: '2024-02-01T00:00:00Z' },
  ];
  assert.deepEqual(wordNames(getSortedWords(rows, 'type', 'asc')), ['C', 'B', 'A']);
});

test('getSortedWords: correct ties across multiple rows are broken by createdAt desc', () => {
  const rows = [
    { word: 'A', correct: 1, createdAt: '2024-01-01T00:00:00Z' },
    { word: 'B', correct: 1, createdAt: '2024-03-01T00:00:00Z' },
    { word: 'C', correct: 1, createdAt: '2024-02-01T00:00:00Z' },
  ];
  assert.deepEqual(wordNames(getSortedWords(rows, 'correct', 'asc')), ['B', 'C', 'A']);
});

// ---------------------------------------------------------------------------
// renderReading
// ---------------------------------------------------------------------------

test('renderReading: empty reading returns empty string', () => {
  assert.equal(renderReading('', '会う', [{ id: 1, reading: 'あ' }]), '');
});

test('renderReading: no kanji data → plain text', () => {
  assert.equal(renderReading('あう', '会う', []), 'あう');
});

test('renderReading: pure kana word → plain text', () => {
  assert.equal(renderReading('きれい', 'きれい', []), 'きれい');
});

test('renderReading: kun\'yomi verb — 会う: 会(kun あ) + う(kana)', () => {
  const result = renderReading('あう', '会う', [{ id: 1, reading: 'あ' }]);
  assert.equal(result,
    '<span class="reading-seg reading-kun">あ</span>' +
    '<span class="reading-seg reading-kana">う</span>');
});

test('renderReading: adjacent kana grouped — 忘れる: 忘(kun わす) + れる(kana)', () => {
  const result = renderReading('わすれる', '忘れる', [{ id: 1, reading: 'わす' }]);
  assert.equal(result,
    '<span class="reading-seg reading-kun">わす</span>' +
    '<span class="reading-seg reading-kana">れる</span>');
});

test('renderReading: on\'yomi noun — 電話: 電(on デン) + 話(on ワ)', () => {
  const result = renderReading('でんわ', '電話', [
    { id: 1, reading: 'デン' },
    { id: 2, reading: 'ワ' },
  ]);
  assert.equal(result,
    '<span class="reading-seg reading-on">デン</span>' +
    '<span class="reading-seg reading-on">ワ</span>');
});

test('renderReading: mixed on+kun — 大好き: 大(on ダイ) + 好(kun す) + き(kana)', () => {
  const result = renderReading('だいすき', '大好き', [
    { id: 1, reading: 'ダイ' },
    { id: 2, reading: 'す' },
  ]);
  assert.equal(result,
    '<span class="reading-seg reading-on">ダイ</span>' +
    '<span class="reading-seg reading-kun">す</span>' +
    '<span class="reading-seg reading-kana">き</span>');
});

test('renderReading: reading field content is ignored when kanjiData present', () => {
  // The reading param value does not affect the coloured output
  const r1 = renderReading('だいすき', '大好き', [{ id: 1, reading: 'ダイ' }, { id: 2, reading: 'す' }]);
  const r2 = renderReading('anything', '大好き', [{ id: 1, reading: 'ダイ' }, { id: 2, reading: 'す' }]);
  assert.equal(r1, r2);
});

test('renderReading: extra kanji in word beyond kanjiData entries are skipped', () => {
  // Only one kanji entry but word has two kanji
  const result = renderReading('てんき', '天気', [{ id: 1, reading: 'テン' }]);
  assert.equal(result, '<span class="reading-seg reading-on">テン</span>');
});

test('renderReading: pitch accent zero marks all but first mora high', () => {
  const result = renderReading('さかな', 'さかな', [], 0);
  assert.match(result, /pitch-rise/);
  assert.match(result, /pitch-high/);
});

test('renderReading: pitch accent one creates an immediate drop after the first mora', () => {
  const result = renderReading('あめ', '雨', [{ id: 1, reading: 'あめ' }], 1);
  assert.match(result, /pitch-drop/);
  assert.match(result, /pitch-high/);
});

test('renderReading: pitch accent with small kana keeps combined morae together', () => {
  const result = renderReading('きょう', '今日', [], 2);
  assert.match(result, />きょ</);
  assert.match(result, />う</);
});

test('renderReading: pitch-connected class appears across high segments', () => {
  const result = renderReading('だいじ', '大事', [{ id: 1, reading: 'ダイ' }, { id: 2, reading: 'ジ' }], 0);
  assert.match(result, /pitch-connected/);
});

test('renderReading: malformed kanjiData falls back to escaped reading text', () => {
  const result = renderReading('て<すと>', '試験', [{ id: 1 }, { id: 2, reading: '' }]);
  assert.equal(result, 'て&lt;すと&gt;');
});
