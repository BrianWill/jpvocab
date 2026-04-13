import { test } from 'node:test';
import assert from 'node:assert/strict';
import { escapeHtml } from '../html-utils.js';

test('escapeHtml: escapes html-sensitive characters with quotes by default', () => {
  assert.equal(escapeHtml(`"<tag>'&`), '&quot;&lt;tag&gt;\'&amp;');
});

test('escapeHtml: can leave double quotes unescaped', () => {
  assert.equal(escapeHtml(`"<tag>"`, { escapeQuotes: false }), '"&lt;tag&gt;"');
});

test('escapeHtml: can escape apostrophes when requested', () => {
  assert.equal(escapeHtml(`it's`, { escapeApostrophe: true }), 'it&#39;s');
});
