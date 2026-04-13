import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  formatElapsedSeconds,
  formatTokenCount,
  getTranslationCounts,
  getTranslationCountsText,
  getTranslationTarget,
} from '../story-generate-utils.js';

const story = {
  sentences: [
    { chunkPosition: 1, orig_lang: 'jp', words: [{ base: '猫' }, { base: '犬' }, { base: '猫' }] },
    { chunkPosition: 1, orig_lang: 'en', words: [{ base: 'cat' }] },
    { chunkPosition: 2, orig_lang: 'jp', words: [{ base: '鳥' }, {}, { base: '' }] },
    { chunkPosition: 2, orig_lang: 'jp' },
  ],
};

test('formatElapsedSeconds: rounds and clamps at zero', () => {
  assert.equal(formatElapsedSeconds(-1), '0s');
  assert.equal(formatElapsedSeconds(1.2), '1s');
  assert.equal(formatElapsedSeconds(1.6), '2s');
});

test('formatTokenCount: handles zero, null, and large numbers', () => {
  assert.equal(formatTokenCount(0), '0');
  assert.equal(formatTokenCount(null), '0');
  assert.equal(formatTokenCount(1234567), '1,234,567');
});

test('getTranslationTarget: returns all sentences when no chunk is selected', () => {
  assert.deepEqual(getTranslationTarget(story, null), { sentences: story.sentences });
});

test('getTranslationTarget: filters by chunk position', () => {
  assert.deepEqual(getTranslationTarget(story, 2), { sentences: story.sentences.slice(2) });
});

test('getTranslationCounts: counts unique Japanese base words only', () => {
  assert.deepEqual(getTranslationCounts(story, null), {
    sentenceCount: 4,
    uniqueWordCount: 3,
  });
});

test('getTranslationCounts: handles missing story or words safely', () => {
  assert.deepEqual(getTranslationCounts(null, null), {
    sentenceCount: 0,
    uniqueWordCount: 0,
  });
});

test('getTranslationCountsText: uses singular labels when appropriate', () => {
  const oneSentenceStory = {
    sentences: [{ chunkPosition: 1, orig_lang: 'jp', words: [{ base: '猫' }] }],
  };
  assert.equal(getTranslationCountsText(oneSentenceStory, 1), '1 sentence, 1 unique word');
});
