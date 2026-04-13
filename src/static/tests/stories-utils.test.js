import { test } from 'node:test';
import assert from 'node:assert/strict';
import { escStoryHtml, sentenceCountLabel, sortStories, storyTimestamp, wordCountLabel } from '../stories-utils.js';

test('storyTimestamp: parses ISO and space-separated timestamps and rejects invalid input', () => {
  assert.equal(storyTimestamp({ createdAt: '2026-01-02T03:04:05Z' }), Date.parse('2026-01-02T03:04:05Z'));
  assert.equal(storyTimestamp({ createdAt: '2026-01-02 03:04:05Z' }), Date.parse('2026-01-02T03:04:05Z'));
  assert.equal(storyTimestamp({ createdAt: 'nope' }), 0);
  assert.equal(storyTimestamp({}), 0);
});

test('sentenceCountLabel and wordCountLabel: pluralize correctly', () => {
  assert.equal(sentenceCountLabel(1), '1 sentence');
  assert.equal(sentenceCountLabel(2), '2 sentences');
  assert.equal(wordCountLabel(1), '1 unique lexicon word');
  assert.equal(wordCountLabel(2), '2 unique lexicon words');
});

test('sortStories: sorts newest first, then by highest id on ties', () => {
  const stories = [
    { id: 1, createdAt: '2026-01-01T00:00:00Z' },
    { id: 3, createdAt: '2026-01-02T00:00:00Z' },
    { id: 2, createdAt: '2026-01-02T00:00:00Z' },
  ];
  assert.deepEqual(sortStories(stories).map(story => story.id), [3, 2, 1]);
});

test('escStoryHtml: escapes html-sensitive characters including quotes', () => {
  assert.equal(escStoryHtml(`"<tag>'&`), '&quot;&lt;tag&gt;&#39;&amp;');
});
