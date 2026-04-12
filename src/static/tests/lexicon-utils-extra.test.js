import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  detailItemExInput,
  detailItemInput,
  detailItemKanjiReadings,
  detailItemPosSelect,
  getSortedWords,
  getFirstImageFile,
  isKanji,
  isImageFile,
  renderReading,
} from '../lexicon-utils.js';

test('isKanji: matches unified ideographs and rejects kana/latin', () => {
  assert.equal(isKanji('猫'), true);
  assert.equal(isKanji('あ'), false);
  assert.equal(isKanji('A'), false);
});

test('getSortedWords: reading asc - Japanese locale compare with createdAt desc tie-breaker', () => {
  const readingWords = [
    { word: 'A', reading: 'ねこ', createdAt: '2024-01-01T00:00:00Z' },
    { word: 'B', reading: 'いぬ', createdAt: '2024-02-01T00:00:00Z' },
    { word: 'C', reading: 'ねこ', createdAt: '2024-03-01T00:00:00Z' },
    { word: 'D', reading: '', createdAt: '2024-04-01T00:00:00Z' },
  ];
  assert.deepEqual(getSortedWords(readingWords, 'reading', 'asc').map(w => w.word), ['D', 'B', 'C', 'A']);
});

test('getSortedWords: reading desc - Japanese locale compare descending', () => {
  const readingWords = [
    { word: 'A', reading: 'ねこ', createdAt: '2024-01-01T00:00:00Z' },
    { word: 'B', reading: 'いぬ', createdAt: '2024-02-01T00:00:00Z' },
    { word: 'C', reading: 'ねこ', createdAt: '2024-03-01T00:00:00Z' },
    { word: 'D', reading: '', createdAt: '2024-04-01T00:00:00Z' },
  ];
  assert.deepEqual(getSortedWords(readingWords, 'reading', 'desc').map(w => w.word), ['C', 'A', 'B', 'D']);
});

test('renderReading: escapes fallback text when kanji data is absent', () => {
  assert.equal(renderReading('<b>', 'あ', []), '&lt;b&gt;');
});

test('detailItemPosSelect: includes placeholder for unknown values and uppercases labels', () => {
  const html = detailItemPosSelect('mystery', {
    noun: 'noun (thing)',
    'godan-verb': 'godan-verb (u-verb)',
  });
  assert.match(html, /<option value="" selected>/);
  assert.match(html, /<option value="noun">NOUN<\/option>/);
  assert.match(html, /<option value="godan-verb">GODAN-VERB<\/option>/);
});

test('detailItemKanjiReadings: renders one editable pair per kanji and trims readings', () => {
  const html = detailItemKanjiReadings('大好き', [
    { id: 1, reading: ' ダイ ' },
    { id: 2, reading: ' す ' },
  ]);
  assert.equal(html,
    '<span class="detail-item detail-kanji">' +
    '<span class="detail-label">kanji readings</span> ' +
    '<span class="kanji-reading-pair">' +
    '<span class="kanji-reading-char">大</span>' +
    '<span class="detail-input kanji-reading-input" contenteditable="true" data-kanji-id="1">ダイ</span>' +
    '</span>' +
    '<span class="kanji-reading-pair">' +
    '<span class="kanji-reading-char">好</span>' +
    '<span class="detail-input kanji-reading-input" contenteditable="true" data-kanji-id="2">す</span>' +
    '</span>' +
    '</span>');
});

test('detailItemInput: trims and escapes editable text', () => {
  assert.equal(detailItemInput('meaning', ' <cat> ', 'detail-meaning'),
    '<span class="detail-item detail-meaning">' +
    '<span class="detail-label">meaning</span> ' +
    '<span class="detail-input" contenteditable="true">&lt;cat&gt;</span>' +
    '</span>');
});

test('detailItemExInput: trims and escapes both example fields', () => {
  const html = detailItemExInput('  猫です  ', '  <cat>  ');
  assert.match(html, /猫です/);
  assert.match(html, /&lt;cat&gt;/);
  assert.doesNotMatch(html, /  猫です  /);
  assert.doesNotMatch(html, /  &lt;cat&gt;  /);
});

test('isImageFile: accepts MIME-typed image files', () => {
  assert.equal(isImageFile({ name: 'cat.bin', type: 'image/png' }), true);
});

test('isImageFile: rejects non-image files', () => {
  assert.equal(isImageFile({ name: 'notes.txt', type: 'text/plain' }), false);
});

test('getFirstImageFile: prefers the first valid image file', () => {
  const files = [
    { name: 'notes.txt', type: 'text/plain' },
    { name: 'cat.jpeg', type: 'image/jpeg' },
    { name: 'dog.png', type: 'image/png' },
  ];
  assert.deepEqual(getFirstImageFile(files), files[1]);
});

test('getFirstImageFile: returns null when there are no files or no image files', () => {
  assert.equal(getFirstImageFile([]), null);
  assert.equal(getFirstImageFile([{ name: 'notes.txt', type: 'text/plain' }]), null);
});
