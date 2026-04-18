import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  clampPlaybackRate,
  playbackModeForStory,
  speechPlaybackLangForStory,
  splitByClause,
  storyCanUseVoicevoxPlayback,
} from '../story-playback-utils.js';

test('clampPlaybackRate: clamps to configured min and max', () => {
  assert.equal(clampPlaybackRate(0.2), 0.5);
  assert.equal(clampPlaybackRate(2.4), 2.0);
});

test('clampPlaybackRate: rounds to two decimals', () => {
  assert.equal(clampPlaybackRate(1.234), 1.23);
  assert.equal(clampPlaybackRate(1.236), 1.24);
});

test('splitByClause: keeps sentence as one clause when there are no commas', () => {
  const sentence = { words: [{ display: '猫' }, { display: 'です' }] };
  assert.deepEqual(splitByClause(sentence), [sentence.words]);
});

test('splitByClause: splits on one or more Japanese commas', () => {
  const words = [{ display: '猫、' }, { display: '犬' }, { display: '鳥、' }, { display: 'です' }];
  assert.deepEqual(splitByClause({ words }).map(clause => clause.map(word => word.display)), [
    ['猫、'],
    ['犬', '鳥、'],
    ['です'],
  ]);
});

test('splitByClause: handles punctuation-only and trailing comma tokens', () => {
  const words = [{ display: '、' }, { display: '猫' }, { display: 'です、' }];
  assert.deepEqual(splitByClause({ words }).map(clause => clause.map(word => word.display)), [
    ['、'],
    ['猫', 'です、'],
  ]);
});

test('speechPlaybackLangForStory: selects english only for all-English stories', () => {
  assert.equal(speechPlaybackLangForStory({ sentences: [{ orig_lang: 'en' }, { orig_lang: 'en' }] }), 'en-US');
  assert.equal(speechPlaybackLangForStory({ sentences: [{ orig_lang: 'jp' }, { orig_lang: 'en' }] }), 'ja-JP');
  assert.equal(speechPlaybackLangForStory({ sentences: [] }), 'ja-JP');
});

test('playbackModeForStory: prefers youtube, then local media, then voicevox, then speech', () => {
  assert.equal(playbackModeForStory({ mediaType: 'youtube', mediaUrl: 'https://www.youtube.com/embed/abc?enablejsapi=1' }), 'youtube');
  assert.equal(
    playbackModeForStory({ mediaType: 'local', mediaUrl: 'D:\\clips\\story.mp4' }, { voicevoxAvailable: true }),
    'local-media',
  );
  assert.equal(playbackModeForStory({ sentences: [{ orig_lang: 'jp' }] }, { voicevoxAvailable: true }), 'voicevox');
  assert.equal(playbackModeForStory({ sentences: [] }), 'speech');
});

test('storyCanUseVoicevoxPlayback: requires Japanese speech without attached media', () => {
  assert.equal(storyCanUseVoicevoxPlayback({ sentences: [{ orig_lang: 'jp' }] }), true);
  assert.equal(storyCanUseVoicevoxPlayback({ sentences: [{ orig_lang: 'en' }] }), false);
  assert.equal(storyCanUseVoicevoxPlayback({ mediaType: 'youtube', mediaUrl: 'https://www.youtube.com/embed/abc?enablejsapi=1', sentences: [{ orig_lang: 'jp' }] }), false);
  assert.equal(storyCanUseVoicevoxPlayback({ mediaType: 'local', mediaUrl: 'D:\\clips\\story.mp4', sentences: [{ orig_lang: 'jp' }] }), false);
});
