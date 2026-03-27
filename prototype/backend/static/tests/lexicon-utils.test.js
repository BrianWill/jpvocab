import { test } from 'node:test';
import assert from 'node:assert/strict';
import { esc, timeAgo, getSortedWords } from '../lexicon-utils.js';

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
  assert.deepEqual(wordNames(getSortedWords(words, 'type', 'desc')), ['B', 'D', 'A', 'C']);
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
