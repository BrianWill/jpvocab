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

export async function postAnswer(sessionId, wordId, correct, sessionState) {
  if (!sessionId) return;
  const resp = await fetch('/api/drill/sessions/' + sessionId + '/answers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ wordId, correct, state: sessionState }),
  });
  if (!resp.ok) throw new Error('failed to save drill answer');
}

export function createDrillState(filterKeys) {
  return {
    activeFilters: new Set(filterKeys),
    currentWord: null,
    doneCount: 0,
    drillStartedAt: Date.now(),
    lastAnswered: null,
    lastAutoPlayedId: null,
    pool: [],
    poolSize: 0,
    redo: [],
    remaining: [],
    round: 1,
    roundSize: DEFAULT_ROUND_SIZE,
    sidebarFlash: null,
    sessionId: null,
    settingsMaxWords: null,
    sidebarItems: [],
    words: [],
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

export function isSessionComplete(sessionState) {
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

export function serializeSessionState(state) {
  return {
    poolSize: state.poolSize,
    roundSize: state.roundSize,
    round: state.round,
    doneCount: state.doneCount,
    activeFilters: [...state.activeFilters],
    pool: state.pool,
    redo: state.redo,
    remaining: state.remaining,
    sidebarItems: state.sidebarItems,
    lastAnswered: state.lastAnswered,
  };
}
