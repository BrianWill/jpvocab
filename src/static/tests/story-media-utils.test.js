import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  sentenceHasStartTime,
  storyCanSeekSentenceInMedia,
  storyHasLocalMedia,
  storyMediaTypeLabel,
  storyUsesYouTubeMedia,
} from '../story-media-utils.js';

test('story media helpers: identify youtube and local media stories', () => {
  assert.equal(storyUsesYouTubeMedia({ mediaType: 'youtube', mediaUrl: 'https://www.youtube.com/embed/abc?enablejsapi=1' }), true);
  assert.equal(storyUsesYouTubeMedia({ mediaType: 'youtube', mediaUrl: '' }), false);
  assert.equal(storyHasLocalMedia({ mediaType: 'local', mediaUrl: 'D:\\media\\story.mp3' }), true);
  assert.equal(storyHasLocalMedia({ mediaType: '', mediaUrl: '' }), false);
});

test('story media helpers: label media types for the UI', () => {
  assert.equal(storyMediaTypeLabel('youtube'), 'YouTube');
  assert.equal(storyMediaTypeLabel('local'), 'Local media');
  assert.equal(storyMediaTypeLabel(''), '');
});

test('story media helpers: sentence seek requires youtube media plus a start time', () => {
  const story = { mediaType: 'youtube', mediaUrl: 'https://www.youtube.com/embed/abc?enablejsapi=1' };
  const timedSentence = { startTimeMs: 3210 };
  const untimedSentence = {};

  assert.equal(sentenceHasStartTime(timedSentence), true);
  assert.equal(sentenceHasStartTime(untimedSentence), false);
  assert.equal(storyCanSeekSentenceInMedia(story, timedSentence), true);
  assert.equal(storyCanSeekSentenceInMedia(story, untimedSentence), false);
  assert.equal(storyCanSeekSentenceInMedia({ mediaType: 'local', mediaUrl: 'D:\\media\\story.mp3' }, timedSentence), false);
});
