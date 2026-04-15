import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  advanceAfterRevealState,
  attemptMatchingPair,
  buildMatchingRoundState,
  DEFAULT_ROUND_SIZE,
  applySidebarAnswer,
  buildRoundState,
  createDrillState,
  createSidebarItems,
  getAnswerFeedbackState,
  getFilteredWords,
  getNextRevealState,
  isMatchingRoundComplete,
  isSessionComplete,
  matchesFilter,
  selectMatchingWord,
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
  assert.equal(state.skipAnswerReveal, false);
  assert.equal(state.awaitingAdvance, false);
  assert.deepEqual(state.matchingRoundWords, []);
  assert.equal(state.matchingSelectedWordId, null);
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

test('getFilteredWords: returns no words when no filters are active', () => {
  assert.deepEqual(getFilteredWords([katakanaWord, verbWord], new Set(), filterKeys), []);
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

test('buildRoundState: allows redo to exceed round size without pulling from pool', () => {
  const result = buildRoundState({
    roundSize: 1,
    redo: [verbWord, nounWord],
    pool: [otherWord],
  });

  assert.deepEqual(result.remaining, [verbWord, nounWord]);
  assert.deepEqual(result.pool, [otherWord]);
});

test('buildMatchingRoundState: fills the round and shuffles info cards independently', () => {
  const result = buildMatchingRoundState({
    roundSize: 2,
    redo: [verbWord],
    pool: [nounWord, otherWord],
  }, words => [...words].reverse());

  assert.deepEqual(result.remaining, [verbWord, nounWord]);
  assert.deepEqual(result.matchingRoundWords, [verbWord, nounWord]);
  assert.deepEqual(result.matchingInfoWords, [nounWord, verbWord]);
  assert.deepEqual(result.matchingRedoWordIds, [verbWord.id]);
  assert.equal(result.matchingSelectedWordId, null);
  assert.deepEqual(result.pool, [otherWord]);
});

test('selectMatchingWord: selects unmatched words and ignores matched ones', () => {
  const state = {
    matchingRoundWords: [nounWord, verbWord],
    matchingMatchedPairs: { [verbWord.id]: verbWord.id },
    matchingSelectedWordId: null,
  };

  assert.equal(selectMatchingWord(state, nounWord.id).matchingSelectedWordId, nounWord.id);
  assert.equal(selectMatchingWord(state, verbWord.id).matchingSelectedWordId, null);
});

test('attemptMatchingPair: first-try correct match increments done count and locks pair', () => {
  const result = attemptMatchingPair({
    matchingPairsMode: true,
    doneCount: 0,
    round: 1,
    roundSize: 2,
    pool: [],
    redo: [],
    remaining: [nounWord, verbWord],
    matchingRoundWords: [nounWord, verbWord],
    matchingInfoWords: [verbWord, nounWord],
    matchingSelectedWordId: nounWord.id,
    matchingMatchedPairs: {},
    matchingCarryoverWordIds: [],
    matchingAttemptedWordIds: [],
    matchingFirstTryCorrectWordIds: [],
  }, nounWord.id, words => words);

  assert.equal(result.firstAttempt, true);
  assert.equal(result.firstAttemptCorrect, true);
  assert.equal(result.nextState.doneCount, 1);
  assert.equal(result.nextState.matchingMatchedPairs[nounWord.id], nounWord.id);
  assert.equal(result.nextState.matchingSelectedWordId, null);
  assert.deepEqual(result.nextState.matchingFirstTryCorrectWordIds, [nounWord.id]);
});

test('attemptMatchingPair: wrong first attempt marks carryover and keeps word unmatched', () => {
  const result = attemptMatchingPair({
    matchingPairsMode: true,
    doneCount: 0,
    round: 1,
    roundSize: 2,
    pool: [],
    redo: [],
    remaining: [nounWord, verbWord],
    matchingRoundWords: [nounWord, verbWord],
    matchingInfoWords: [verbWord, nounWord],
    matchingSelectedWordId: nounWord.id,
    matchingMatchedPairs: {},
    matchingCarryoverWordIds: [],
    matchingAttemptedWordIds: [],
    matchingFirstTryCorrectWordIds: [],
  }, verbWord.id, words => words);

  assert.equal(result.firstAttempt, true);
  assert.equal(result.firstAttemptCorrect, false);
  assert.equal(result.nextState.doneCount, 0);
  assert.deepEqual(result.nextState.matchingCarryoverWordIds, [nounWord.id]);
  assert.equal(result.nextState.matchingMatchedPairs[nounWord.id], undefined);
  assert.equal(result.nextState.matchingSelectedWordId, nounWord.id);
});

test('attemptMatchingPair: later correct match after miss stays carried-over and finishes round', () => {
  const result = attemptMatchingPair({
    matchingPairsMode: true,
    doneCount: 0,
    round: 1,
    roundSize: 2,
    pool: [otherWord],
    redo: [],
    remaining: [nounWord, verbWord],
    matchingRoundWords: [nounWord, verbWord],
    matchingInfoWords: [verbWord, nounWord],
    matchingSelectedWordId: nounWord.id,
    matchingMatchedPairs: { [verbWord.id]: verbWord.id },
    matchingCarryoverWordIds: [nounWord.id],
    matchingAttemptedWordIds: [nounWord.id, verbWord.id],
    matchingFirstTryCorrectWordIds: [verbWord.id],
  }, nounWord.id, words => [...words].reverse());

  assert.equal(result.firstAttempt, false);
  assert.equal(result.nextState.round, 2);
  assert.deepEqual(result.nextState.redo, []);
  assert.deepEqual(result.nextState.matchingCarryoverWordIds, []);
  assert.deepEqual(result.nextState.remaining, [nounWord, otherWord]);
  assert.deepEqual(result.nextState.matchingRoundWords, [nounWord, otherWord]);
  assert.deepEqual(result.nextState.matchingInfoWords, [otherWord, nounWord]);
  assert.equal(result.nextState.doneCount, 0);
});

test('isMatchingRoundComplete: requires every round word to be matched', () => {
  assert.equal(isMatchingRoundComplete({
    matchingRoundWords: [nounWord, verbWord],
    matchingMatchedPairs: { [nounWord.id]: nounWord.id },
  }), false);
  assert.equal(isMatchingRoundComplete({
    matchingRoundWords: [nounWord, verbWord],
    matchingMatchedPairs: { [nounWord.id]: nounWord.id, [verbWord.id]: verbWord.id },
  }), true);
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
    matchingRoundWords: [],
    matchingInfoWords: [],
    matchingRedoWordIds: [],
    matchingSelectedWordId: null,
    matchingMatchedPairs: {},
    matchingCarryoverWordIds: [],
    matchingAttemptedWordIds: [],
    matchingFirstTryCorrectWordIds: [],
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

test('getAnswerFeedbackState: records answer result without advancing', () => {
  const state = {
    currentWord: nounWord,
    remaining: [nounWord, otherWord],
    redo: [],
    pool: [katakanaWord],
    round: 2,
    doneCount: 4,
    sidebarItems: createSidebarItems([nounWord, otherWord]),
  };

  assert.deepEqual(getAnswerFeedbackState(state, true), {
    ...state,
    doneCount: 5,
    lastAnswered: { word: nounWord, knew: true },
    sidebarItems: [
      { word: nounWord, status: 'known' },
      { word: otherWord, status: 'unseen' },
    ],
    awaitingAdvance: true,
    pendingAnswerCorrect: true,
  });
});

test('advanceAfterRevealState: advances using pending answer result', () => {
  const state = {
    currentWord: nounWord,
    remaining: [nounWord, otherWord],
    redo: [],
    pool: [katakanaWord],
    round: 2,
    doneCount: 5,
    sidebarItems: [
      { word: nounWord, status: 'known' },
      { word: otherWord, status: 'unseen' },
    ],
    lastAnswered: { word: nounWord, knew: true },
    awaitingAdvance: true,
    pendingAnswerCorrect: true,
  };

  assert.deepEqual(advanceAfterRevealState(state), {
    ...state,
    redo: [],
    remaining: [otherWord],
    currentWord: otherWord,
    round: 2,
    pool: [katakanaWord],
    awaitingAdvance: false,
    pendingAnswerCorrect: null,
  });
});

test('advanceAfterRevealState: returns null when not awaiting advance', () => {
  assert.equal(advanceAfterRevealState({
    currentWord: nounWord,
    awaitingAdvance: false,
    pendingAnswerCorrect: true,
  }), null);
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
    matchingPairsMode: true,
    matchingRoundWords: [nounWord, otherWord],
    matchingInfoWords: [otherWord, nounWord],
    matchingRedoWordIds: [nounWord.id],
    matchingSelectedWordId: nounWord.id,
    matchingMatchedPairs: { [otherWord.id]: otherWord.id },
    matchingCarryoverWordIds: [nounWord.id],
    matchingAttemptedWordIds: [nounWord.id, otherWord.id],
    matchingFirstTryCorrectWordIds: [otherWord.id],
    skipAnswerReveal: false,
    awaitingAdvance: true,
    pendingAnswerCorrect: true,
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
    matchingPairsMode: true,
    matchingRoundWords: [nounWord, otherWord],
    matchingInfoWords: [otherWord, nounWord],
    matchingRedoWordIds: [nounWord.id],
    matchingSelectedWordId: nounWord.id,
    matchingMatchedPairs: { [otherWord.id]: otherWord.id },
    matchingCarryoverWordIds: [nounWord.id],
    matchingAttemptedWordIds: [nounWord.id, otherWord.id],
    matchingFirstTryCorrectWordIds: [otherWord.id],
    skipAnswerReveal: false,
    awaitingAdvance: true,
    pendingAnswerCorrect: true,
  });
});

test('serializeSessionState: preserves partial state and filter insertion order', () => {
  const serialized = serializeSessionState({
    poolSize: 0,
    requestedRoundSize: 5,
    roundSize: 5,
    round: 3,
    doneCount: 1,
    activeFilters: new Set(['other', 'verbs']),
    pool: [],
    redo: [],
    remaining: [],
    sidebarItems: [],
    lastAnswered: null,
    matchingPairsMode: false,
    matchingRoundWords: [],
    matchingInfoWords: [],
    matchingRedoWordIds: [],
    matchingSelectedWordId: null,
    matchingMatchedPairs: {},
    matchingCarryoverWordIds: [],
    matchingAttemptedWordIds: [],
    matchingFirstTryCorrectWordIds: [],
    skipAnswerReveal: true,
    awaitingAdvance: false,
    pendingAnswerCorrect: null,
  });

  assert.deepEqual(serialized.activeFilters, ['other', 'verbs']);
  assert.equal(serialized.lastAnswered, null);
  assert.equal(serialized.pendingAnswerCorrect, null);
});
