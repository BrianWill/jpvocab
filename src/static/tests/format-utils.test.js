import { test } from 'node:test';
import assert from 'node:assert/strict';
import { formatRelativeTime, normalizeDateInput, pluralize } from '../format-utils.js';

function msAgo(ms) {
  return new Date(Date.now() - ms).toISOString();
}

test('formatRelativeTime: handles ISO strings and timestamps', () => {
  assert.equal(formatRelativeTime(msAgo(30_000)), 'just now');
  assert.equal(formatRelativeTime(Date.now() - 61_000), '1 minute ago');
});

test('formatRelativeTime: handles longer ranges', () => {
  assert.equal(formatRelativeTime(msAgo(3 * 3_600_000 + 1000)), '3 hours ago');
  assert.equal(formatRelativeTime(msAgo(40 * 86_400_000)), '1 month ago');
});

test('pluralize: singular and plural forms', () => {
  assert.equal(pluralize(1, 'word'), '1 word');
  assert.equal(pluralize(2, 'word'), '2 words');
  assert.equal(pluralize(2, 'person', 'people'), '2 people');
});

test('normalizeDateInput: normalizes space-separated timestamps', () => {
  assert.equal(normalizeDateInput('2026-01-02 03:04:05Z').toISOString(), '2026-01-02T03:04:05.000Z');
});
