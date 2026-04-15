export const DEFAULT_ROUND_SIZE = 10;

export async function loadDrillData() {
  const [wordsResp, settingsResp, currentSessionResp] = await Promise.all([
    fetch('/api/words'),
    fetch('/api/settings/drill'),
    fetch('/api/drill/sessions/current'),
  ]);

  const allWords = await wordsResp.json();
  const settings = await settingsResp.json();
  const currentSessionData = await currentSessionResp.json();

  return {
    allWords,
    currentSession: currentSessionData.session,
    settings,
  };
}

export async function createSession(sessionState) {
  const resp = await fetch('/api/drill/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ state: sessionState }),
  });
  const data = await resp.json();
  return data.id;
}

export async function updateSessionState(sessionId, sessionState) {
  if (!sessionId) return;
  const resp = await fetch('/api/drill/sessions/' + sessionId + '/state', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ state: sessionState }),
  });
  if (!resp.ok) throw new Error('failed to save drill session');
}

export async function postAnswer(sessionId, wordId, correct, sessionState) {
  if (!sessionId) return;
  const resp = await fetch('/api/drill/sessions/' + sessionId + '/answers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ wordId, correct, state: sessionState }),
  });
  if (!resp.ok) throw new Error('failed to save drill answer');
}

function includesWordId(ids, wordId) {
  return Array.isArray(ids) && ids.includes(wordId);
}

export function createEmptyMatchingState() {
  return {
    matchingRoundWords: [],
    matchingInfoWords: [],
    matchingRedoWordIds: [],
    matchingSelectedWordId: null,
    matchingMatchedPairs: {},
    matchingCarryoverWordIds: [],
    matchingAttemptedWordIds: [],
    matchingFirstTryCorrectWordIds: [],
  };
}

function normalizeMatchingState(sessionState) {
  const roundWords = Array.isArray(sessionState.matchingRoundWords)
    ? sessionState.matchingRoundWords
    : (sessionState.matchingPairsMode && Array.isArray(sessionState.remaining) ? sessionState.remaining : []);

  const infoWords = Array.isArray(sessionState.matchingInfoWords) && sessionState.matchingInfoWords.length > 0
    ? sessionState.matchingInfoWords
    : roundWords;

  return {
    matchingRoundWords: roundWords,
    matchingInfoWords: infoWords,
    matchingRedoWordIds: Array.isArray(sessionState.matchingRedoWordIds)
      ? sessionState.matchingRedoWordIds
      : [],
    matchingSelectedWordId: typeof sessionState.matchingSelectedWordId === 'number'
      ? sessionState.matchingSelectedWordId
      : null,
    matchingMatchedPairs: sessionState.matchingMatchedPairs && typeof sessionState.matchingMatchedPairs === 'object'
      ? sessionState.matchingMatchedPairs
      : {},
    matchingCarryoverWordIds: Array.isArray(sessionState.matchingCarryoverWordIds)
      ? sessionState.matchingCarryoverWordIds
      : [],
    matchingAttemptedWordIds: Array.isArray(sessionState.matchingAttemptedWordIds)
      ? sessionState.matchingAttemptedWordIds
      : [],
    matchingFirstTryCorrectWordIds: Array.isArray(sessionState.matchingFirstTryCorrectWordIds)
      ? sessionState.matchingFirstTryCorrectWordIds
      : [],
  };
}

export function createDrillState(filterKeys) {
  return {
    activeFilters: new Set(filterKeys),
    awaitingAdvance: false,
    currentWord: null,
    doneCount: 0,
    drillStartedAt: Date.now(),
    lastAnswered: null,
    lastAutoPlayedId: null,
    matchingAttemptedWordIds: [],
    matchingCarryoverWordIds: [],
    matchingFirstTryCorrectWordIds: [],
    matchingInfoWords: [],
    matchingMatchedPairs: {},
    matchingPairsMode: false,
    matchingRedoWordIds: [],
    matchingRoundWords: [],
    matchingSelectedWordId: null,
    pendingAnswerCorrect: null,
    pool: [],
    poolSize: 0,
    redo: [],
    remaining: [],
    round: 1,
    requestedRoundSize: DEFAULT_ROUND_SIZE,
    roundSize: DEFAULT_ROUND_SIZE,
    sidebarFlash: null,
    sessionId: null,
    settingsMaxWords: null,
    skipAnswerReveal: false,
    sidebarItems: [],
    words: [],
    ...createEmptyMatchingState(),
  };
}

export function matchesFilter(word, filterKey) {
  const isKatakana = /^[\u30A0-\u30FF]+$/.test(word.word);
  const isVerb = word.type === 'ichidan-verb' || word.type === 'godan-verb';
  const isNoun = word.type === 'noun';
  if (filterKey === 'katakana') return isKatakana;
  if (filterKey === 'verbs') return isVerb;
  if (filterKey === 'nouns') return isNoun;
  if (filterKey === 'other') return !isKatakana && !isVerb && !isNoun;
  return false;
}

export function getFilteredWords(words, activeFilters, filterKeys) {
  return words.filter(word => filterKeys.some(filterKey => activeFilters.has(filterKey) && matchesFilter(word, filterKey)));
}

export function createSidebarItems(words, redoSet = new Set()) {
  return words.map(word => ({
    word,
    status: redoSet.has(word.word) ? 'unseen-redo' : 'unseen',
  }));
}

export function applySidebarAnswer(sidebarItems, word, knew) {
  const status = knew ? 'known' : 'missed';
  let found = false;
  const nextItems = sidebarItems.map(item => {
    if (item.word.word !== word.word) return item;
    found = true;
    return { ...item, word, status };
  });
  if (!found) nextItems.push({ word, status });
  return nextItems;
}

export function isMatchingRoundComplete(sessionState) {
  const { matchingRoundWords, matchingMatchedPairs } = normalizeMatchingState(sessionState);
  return matchingRoundWords.length > 0 &&
    matchingRoundWords.every(word => typeof matchingMatchedPairs[word.id] === 'number');
}

export function isSessionComplete(sessionState) {
  if (sessionState.matchingPairsMode) {
    const { matchingRoundWords } = normalizeMatchingState(sessionState);
    return matchingRoundWords.length === 0 &&
      sessionState.redo.length === 0 &&
      sessionState.pool.length === 0;
  }

  return !sessionState.currentWord &&
    sessionState.remaining.length === 0 &&
    sessionState.redo.length === 0 &&
    sessionState.pool.length === 0;
}

export function buildRoundState(sessionState) {
  const slots = Math.max(0, sessionState.roundSize - sessionState.redo.length);
  const pool = [...sessionState.pool];
  const picked = pool.splice(0, slots);
  const remaining = [...sessionState.redo, ...picked];
  const redoSet = new Set(sessionState.redo.map(word => word.word));

  return {
    pool,
    redo: [],
    remaining,
    currentWord: remaining[0] || null,
    sidebarItems: createSidebarItems(remaining, redoSet),
    ...createEmptyMatchingState(),
  };
}

export function buildMatchingRoundState(sessionState, shuffleWords = words => words) {
  const slots = Math.max(0, sessionState.roundSize - sessionState.redo.length);
  const pool = [...sessionState.pool];
  const picked = pool.splice(0, slots);
  const remaining = [...sessionState.redo, ...picked];
  const shuffledRedo = shuffleWords(sessionState.redo);
  const shuffledFresh = shuffleWords(picked);

  return {
    pool,
    redo: [],
    remaining,
    currentWord: null,
    sidebarItems: [],
    matchingRoundWords: [...shuffledRedo, ...shuffledFresh],
    matchingInfoWords: shuffleWords(remaining),
    matchingRedoWordIds: sessionState.redo.map(word => word.id),
    matchingSelectedWordId: null,
    matchingMatchedPairs: {},
    matchingCarryoverWordIds: [],
    matchingAttemptedWordIds: [],
    matchingFirstTryCorrectWordIds: [],
    awaitingAdvance: false,
    pendingAnswerCorrect: null,
  };
}

export function restoreStandardSessionState(sessionState) {
  const remaining = Array.isArray(sessionState.remaining) ? sessionState.remaining : [];
  const redo = Array.isArray(sessionState.redo) ? sessionState.redo : [];
  const sidebarItems = Array.isArray(sessionState.sidebarItems) ? sessionState.sidebarItems : [];
  const awaitingAdvance = sessionState.awaitingAdvance === true;
  const pendingAnswerCorrect = awaitingAdvance && typeof sessionState.pendingAnswerCorrect === 'boolean'
    ? sessionState.pendingAnswerCorrect
    : null;

  return {
    remaining,
    redo,
    currentWord: remaining[0] || null,
    lastAnswered: sessionState.lastAnswered || null,
    sidebarItems,
    awaitingAdvance,
    pendingAnswerCorrect,
    ...createEmptyMatchingState(),
  };
}

export function restoreMatchingSessionState(sessionState) {
  const matchingState = normalizeMatchingState({
    ...sessionState,
    matchingPairsMode: true,
  });

  return {
    remaining: Array.isArray(sessionState.remaining) ? sessionState.remaining : [],
    redo: Array.isArray(sessionState.redo) ? sessionState.redo : [],
    currentWord: null,
    lastAnswered: sessionState.lastAnswered || null,
    sidebarItems: [],
    awaitingAdvance: false,
    pendingAnswerCorrect: null,
    ...matchingState,
  };
}

export function hasRestorableSessionState(sessionState) {
  if (sessionState.matchingPairsMode) {
    const { matchingRoundWords } = restoreMatchingSessionState(sessionState);
    return (sessionState.poolSize || 0) > 0 ||
      matchingRoundWords.length > 0 ||
      (Array.isArray(sessionState.redo) && sessionState.redo.length > 0) ||
      (Array.isArray(sessionState.pool) && sessionState.pool.length > 0);
  }

  const restored = restoreStandardSessionState(sessionState);
  return (sessionState.poolSize || 0) > 0 ||
    restored.currentWord !== null ||
    restored.remaining.length > 0 ||
    restored.redo.length > 0 ||
    restored.awaitingAdvance ||
    restored.lastAnswered !== null ||
    (Array.isArray(sessionState.pool) && sessionState.pool.length > 0);
}

export function selectMatchingWord(sessionState, wordId) {
  const { matchingRoundWords, matchingMatchedPairs } = normalizeMatchingState(sessionState);
  const selectedWord = matchingRoundWords.find(word => word.id === wordId);
  if (!selectedWord || typeof matchingMatchedPairs[wordId] === 'number') {
    return sessionState;
  }
  return {
    ...sessionState,
    matchingSelectedWordId: wordId,
  };
}

export function attemptMatchingPair(sessionState, infoWordId, shuffleWords = words => words) {
  const matchingState = normalizeMatchingState(sessionState);
  const selectedWordId = matchingState.matchingSelectedWordId;
  if (typeof selectedWordId !== 'number') return null;
  if (typeof matchingState.matchingMatchedPairs[selectedWordId] === 'number') return null;
  if (Object.values(matchingState.matchingMatchedPairs).includes(infoWordId)) return null;

  const selectedWord = matchingState.matchingRoundWords.find(word => word.id === selectedWordId);
  const infoWord = matchingState.matchingInfoWords.find(word => word.id === infoWordId);
  if (!selectedWord || !infoWord) return null;

  const isCorrect = selectedWordId === infoWordId;
  const firstAttempt = !includesWordId(matchingState.matchingAttemptedWordIds, selectedWordId);
  const matchingAttemptedWordIds = firstAttempt
    ? [...matchingState.matchingAttemptedWordIds, selectedWordId]
    : [...matchingState.matchingAttemptedWordIds];
  const matchingCarryoverWordIds = (!isCorrect && firstAttempt)
    ? [...matchingState.matchingCarryoverWordIds, selectedWordId]
    : [...matchingState.matchingCarryoverWordIds];
  const matchingFirstTryCorrectWordIds = (isCorrect && firstAttempt)
    ? [...matchingState.matchingFirstTryCorrectWordIds, selectedWordId]
    : [...matchingState.matchingFirstTryCorrectWordIds];

  let nextState = {
    ...sessionState,
    matchingAttemptedWordIds,
    matchingCarryoverWordIds,
    matchingFirstTryCorrectWordIds,
    lastAnswered: { word: selectedWord, knew: isCorrect && firstAttempt },
  };

  if (!isCorrect) {
    nextState = {
      ...nextState,
      matchingSelectedWordId: selectedWordId,
    };
    return {
      nextState,
      answeredWord: selectedWord,
      firstAttempt,
      firstAttemptCorrect: false,
    };
  }

  const matchingMatchedPairs = {
    ...matchingState.matchingMatchedPairs,
    [selectedWordId]: infoWordId,
  };
  const nextDoneCount = sessionState.doneCount + (firstAttempt ? 1 : 0);
  nextState = {
    ...nextState,
    doneCount: nextDoneCount,
    matchingMatchedPairs,
    matchingSelectedWordId: null,
  };

  const completedRound = matchingState.matchingRoundWords.every(word =>
    typeof (word.id === selectedWordId ? matchingMatchedPairs[selectedWordId] : matchingMatchedPairs[word.id]) === 'number'
  );

  if (!completedRound) {
    return {
      nextState,
      answeredWord: selectedWord,
      firstAttempt,
      firstAttemptCorrect: firstAttempt,
    };
  }

  const redo = matchingState.matchingRoundWords.filter(word =>
    includesWordId(matchingCarryoverWordIds, word.id)
  );

  if (redo.length > 0 || sessionState.pool.length > 0) {
    return {
      nextState: {
        ...nextState,
        round: sessionState.round + 1,
        ...buildMatchingRoundState({
          ...sessionState,
          pool: sessionState.pool,
          redo,
          roundSize: sessionState.roundSize,
        }, shuffleWords),
      },
      answeredWord: selectedWord,
      firstAttempt,
      firstAttemptCorrect: firstAttempt,
    };
  }

  return {
    nextState: {
      ...nextState,
      currentWord: null,
      remaining: [],
      redo: [],
      matchingRoundWords: [],
      matchingInfoWords: [],
      matchingSelectedWordId: null,
      matchingMatchedPairs: {},
      matchingCarryoverWordIds: [],
      matchingAttemptedWordIds: [],
      matchingFirstTryCorrectWordIds: [],
      matchingRedoWordIds: [],
      sidebarItems: [],
    },
    answeredWord: selectedWord,
    firstAttempt,
    firstAttemptCorrect: firstAttempt,
  };
}

export function getNextRevealState(sessionState, knew) {
  if (!sessionState.currentWord) return null;

  const answered = sessionState.currentWord;
  const remaining = sessionState.remaining.slice(1);
  const redo = knew ? [...sessionState.redo] : [...sessionState.redo, answered];
  const nextState = {
    doneCount: sessionState.doneCount + (knew ? 1 : 0),
    lastAnswered: { word: answered, knew },
    sidebarItems: applySidebarAnswer(sessionState.sidebarItems, answered, knew),
    redo,
    remaining,
  };

  if (remaining.length > 0) {
    return {
      ...nextState,
      currentWord: remaining[0],
      round: sessionState.round,
      pool: sessionState.pool,
    };
  }

  if (redo.length > 0 || sessionState.pool.length > 0) {
    return {
      ...nextState,
      round: sessionState.round + 1,
      ...buildRoundState({
        ...sessionState,
        doneCount: nextState.doneCount,
        lastAnswered: nextState.lastAnswered,
        pool: sessionState.pool,
        redo,
      }),
    };
  }

  return {
    ...nextState,
    currentWord: null,
    round: sessionState.round,
    pool: sessionState.pool,
  };
}

export function getAnswerFeedbackState(sessionState, knew) {
  if (!sessionState.currentWord) return null;

  const answered = sessionState.currentWord;
  return {
    ...sessionState,
    doneCount: sessionState.doneCount + (knew ? 1 : 0),
    lastAnswered: { word: answered, knew },
    sidebarItems: applySidebarAnswer(sessionState.sidebarItems, answered, knew),
    awaitingAdvance: true,
    pendingAnswerCorrect: knew,
  };
}

export function advanceAfterRevealState(sessionState) {
  if (!sessionState.currentWord || !sessionState.awaitingAdvance || typeof sessionState.pendingAnswerCorrect !== 'boolean') {
    return null;
  }

  const knew = sessionState.pendingAnswerCorrect;
  const answered = sessionState.currentWord;
  const remaining = sessionState.remaining.slice(1);
  const redo = knew ? [...sessionState.redo] : [...sessionState.redo, answered];
  const nextState = {
    ...sessionState,
    lastAnswered: { word: answered, knew },
    sidebarItems: applySidebarAnswer(sessionState.sidebarItems, answered, knew),
    redo,
    remaining,
    awaitingAdvance: false,
    pendingAnswerCorrect: null,
  };

  if (remaining.length > 0) {
    return {
      ...nextState,
      currentWord: remaining[0],
      round: sessionState.round,
      pool: sessionState.pool,
    };
  }

  if (redo.length > 0 || sessionState.pool.length > 0) {
    return {
      ...nextState,
      round: sessionState.round + 1,
      ...buildRoundState({
        ...sessionState,
        doneCount: nextState.doneCount,
        lastAnswered: nextState.lastAnswered,
        pool: sessionState.pool,
        redo,
      }),
      awaitingAdvance: false,
      pendingAnswerCorrect: null,
    };
  }

  return {
    ...nextState,
    currentWord: null,
    round: sessionState.round,
    pool: sessionState.pool,
  };
}

export function serializeSessionState(state) {
  return {
    poolSize: state.poolSize,
    requestedRoundSize: state.requestedRoundSize,
    roundSize: state.roundSize,
    round: state.round,
    doneCount: state.doneCount,
    activeFilters: [...state.activeFilters],
    pool: state.pool,
    redo: state.redo,
    remaining: state.remaining,
    sidebarItems: state.sidebarItems,
    lastAnswered: state.lastAnswered,
    matchingPairsMode: state.matchingPairsMode,
    matchingRoundWords: state.matchingRoundWords,
    matchingInfoWords: state.matchingInfoWords,
    matchingRedoWordIds: state.matchingRedoWordIds,
    matchingSelectedWordId: state.matchingSelectedWordId,
    matchingMatchedPairs: state.matchingMatchedPairs,
    matchingCarryoverWordIds: state.matchingCarryoverWordIds,
    matchingAttemptedWordIds: state.matchingAttemptedWordIds,
    matchingFirstTryCorrectWordIds: state.matchingFirstTryCorrectWordIds,
    skipAnswerReveal: state.skipAnswerReveal,
    awaitingAdvance: state.awaitingAdvance,
    pendingAnswerCorrect: state.pendingAnswerCorrect,
  };
}
