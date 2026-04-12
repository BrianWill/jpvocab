import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  DEFAULT_ROUND_SIZE,
  applySidebarAnswer,
  buildRoundState,
  createDrillState,
  createSidebarItems,
  getFilteredWords,
  getNextRevealState,
  isSessionComplete,
  matchesFilter,
  serializeSessionState,
} from '../drill-state.js';

const filterKeys = ['katakana', 'verbs', 'nouns', 'other'];

const katakanaWord = { id: 1, word: 'テレビ', type: 'noun' };
const verbWord = { id: 2, word: '食べる', type: 'ichidan-verb' };
const nounWord = { id: 3, word: '猫', type: 'noun' };
const otherWord = { id: 4, word: '静か', type: 'na-adjective' };

test('createDrillState: seeds defaults from filter keys', () => {
  const state = createDrillState(filterKeys);
  assert.deepEqual([...state.activeFilters], filterKeys);
  assert.equal(state.round, 1);
  assert.equal(state.requestedRoundSize, DEFAULT_ROUND_SIZE);
  assert.equal(state.roundSize, DEFAULT_ROUND_SIZE);
  assert.equal(state.currentWord, null);
  assert.deepEqual(state.pool, []);
  assert.deepEqual(state.redo, []);
});

test('matchesFilter: classifies katakana, verbs, nouns, and other buckets', () => {
  assert.equal(matchesFilter(katakanaWord, 'katakana'), true);
  assert.equal(matchesFilter(katakanaWord, 'nouns'), true);
  assert.equal(matchesFilter(verbWord, 'verbs'), true);
  assert.equal(matchesFilter(nounWord, 'nouns'), true);
  assert.equal(matchesFilter(otherWord, 'other'), true);
  assert.equal(matchesFilter(otherWord, 'verbs'), false);
  assert.equal(matchesFilter(nounWord, 'missing'), false);
});

test('getFilteredWords: keeps words matching any active filter in filter order', () => {
  const words = [katakanaWord, verbWord, nounWord, otherWord];
  const activeFilters = new Set(['verbs', 'other']);
  assert.deepEqual(getFilteredWords(words, activeFilters, filterKeys), [verbWord, otherWord]);
});

test('createSidebarItems: marks redo words with unseen-redo status', () => {
  const items = createSidebarItems([nounWord, verbWord], new Set([verbWord.word]));
  assert.deepEqual(items, [
    { word: nounWord, status: 'unseen' },
    { word: verbWord, status: 'unseen-redo' },
  ]);
});

test('applySidebarAnswer: updates existing item status and word payload', () => {
  const originalWord = { word: '猫', type: 'noun', meaning: 'cat' };
  const updatedWord = { ...originalWord, meaning: 'kitty' };
  const items = [{ word: originalWord, status: 'unseen' }];
  assert.deepEqual(applySidebarAnswer(items, updatedWord, true), [
    { word: updatedWord, status: 'known' },
  ]);
  assert.deepEqual(items, [{ word: originalWord, status: 'unseen' }]);
});

test('applySidebarAnswer: appends a new item when word was not present', () => {
  assert.deepEqual(applySidebarAnswer([], nounWord, false), [
    { word: nounWord, status: 'missed' },
  ]);
});

test('isSessionComplete: requires no current word and empty queues', () => {
  assert.equal(isSessionComplete({
    currentWord: null,
    remaining: [],
    redo: [],
    pool: [],
  }), true);

  assert.equal(isSessionComplete({
    currentWord: nounWord,
    remaining: [],
    redo: [],
    pool: [],
  }), false);
});

test('buildRoundState: fills the round from redo first, then pool', () => {
  const redo = [verbWord];
  const pool = [nounWord, otherWord, katakanaWord];
  const result = buildRoundState({
    roundSize: 3,
    redo,
    pool,
  });

  assert.deepEqual(result.remaining, [verbWord, nounWord, otherWord]);
  assert.equal(result.currentWord, verbWord);
  assert.deepEqual(result.pool, [katakanaWord]);
  assert.deepEqual(result.redo, []);
  assert.deepEqual(result.sidebarItems, [
    { word: verbWord, status: 'unseen-redo' },
    { word: nounWord, status: 'unseen' },
    { word: otherWord, status: 'unseen' },
  ]);
  assert.deepEqual(pool, [nounWord, otherWord, katakanaWord]);
  assert.deepEqual(redo, [verbWord]);
});

test('getNextRevealState: advances within the same round on a known answer', () => {
  const state = {
    currentWord: nounWord,
    remaining: [nounWord, otherWord],
    redo: [],
    pool: [katakanaWord],
    round: 2,
    doneCount: 4,
    sidebarItems: createSidebarItems([nounWord, otherWord]),
  };

  assert.deepEqual(getNextRevealState(state, true), {
    doneCount: 5,
    lastAnswered: { word: nounWord, knew: true },
    sidebarItems: [
      { word: nounWord, status: 'known' },
      { word: otherWord, status: 'unseen' },
    ],
    redo: [],
    remaining: [otherWord],
    currentWord: otherWord,
    round: 2,
    pool: [katakanaWord],
  });
});

test('getNextRevealState: carries missed words into the next round', () => {
  const state = {
    currentWord: nounWord,
    remaining: [nounWord],
    redo: [],
    pool: [otherWord, katakanaWord],
    roundSize: 3,
    round: 1,
    doneCount: 0,
    sidebarItems: createSidebarItems([nounWord]),
  };

  assert.deepEqual(getNextRevealState(state, false), {
    doneCount: 0,
    lastAnswered: { word: nounWord, knew: false },
    sidebarItems: [
      { word: nounWord, status: 'unseen-redo' },
      { word: otherWord, status: 'unseen' },
      { word: katakanaWord, status: 'unseen' },
    ],
    redo: [],
    remaining: [nounWord, otherWord, katakanaWord],
    currentWord: nounWord,
    round: 2,
    pool: [],
  });
});

test('getNextRevealState: finishes the session when nothing remains', () => {
  const state = {
    currentWord: nounWord,
    remaining: [nounWord],
    redo: [],
    pool: [],
    round: 3,
    doneCount: 2,
    sidebarItems: createSidebarItems([nounWord]),
  };

  assert.deepEqual(getNextRevealState(state, true), {
    doneCount: 3,
    lastAnswered: { word: nounWord, knew: true },
    sidebarItems: [
      { word: nounWord, status: 'known' },
    ],
    redo: [],
    remaining: [],
    currentWord: null,
    round: 3,
    pool: [],
  });
});

test('getNextRevealState: returns null when there is no current word', () => {
  assert.equal(getNextRevealState({
    currentWord: null,
  }, true), null);
});

test('serializeSessionState: keeps durable progress fields and converts filters to an array', () => {
  const state = {
    poolSize: 25,
    requestedRoundSize: 10,
    roundSize: 7,
    round: 3,
    doneCount: 11,
    activeFilters: new Set(['verbs', 'other']),
    pool: [verbWord],
    redo: [nounWord],
    remaining: [otherWord],
    sidebarItems: [{ word: katakanaWord, status: 'known' }],
    lastAnswered: { word: katakanaWord, knew: true },
    currentWord: otherWord,
    sessionId: 99,
  };

  assert.deepEqual(serializeSessionState(state), {
    poolSize: 25,
    requestedRoundSize: 10,
    roundSize: 7,
    round: 3,
    doneCount: 11,
    activeFilters: ['verbs', 'other'],
    pool: [verbWord],
    redo: [nounWord],
    remaining: [otherWord],
    sidebarItems: [{ word: katakanaWord, status: 'known' }],
    lastAnswered: { word: katakanaWord, knew: true },
  });
});
