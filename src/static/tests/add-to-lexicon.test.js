import { test } from 'node:test';
import assert from 'node:assert/strict';
import { getFieldLanguageErrorMsg, getFieldLanguageFilter, sanitizeFieldInput } from '../add-to-lexicon-utils.js';

test('sanitizeFieldInput: strips latin letters from Japanese-only fields', () => {
  assert.equal(sanitizeFieldInput('abc猫def', 'example-jp'), '猫');
  assert.equal(sanitizeFieldInput('taberuたべる', 'reading'), 'たべる');
  assert.equal(sanitizeFieldInput('Daiダイ', 'kanji-reading'), 'ダイ');
});

test('sanitizeFieldInput: strips Japanese characters from English example fields', () => {
  assert.equal(sanitizeFieldInput('cat猫です dog', 'example-en'), 'cat dog');
});

test('getFieldLanguageFilter: returns null for unrelated fields', () => {
  assert.equal(getFieldLanguageFilter('meaning'), null);
});

test('getFieldLanguageErrorMsg: uses the expected user-facing copy', () => {
  assert.equal(getFieldLanguageErrorMsg('example-en'), 'English only - Japanese characters are not allowed here');
  assert.equal(getFieldLanguageErrorMsg('reading'), 'Japanese only - Latin letters are not allowed here');
});
