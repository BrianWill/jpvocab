import {
  attachNumberStepper,
  checkVoicevoxAvailable,
  DRILL_FILTER_KEYS,
  getVoicevoxSettings,
  playTts,
  playWordAudio,
  playSentenceAudio,
  populateWordTooltip,
} from './common.js';
import { getSynthAudio } from './synth-cache.js';
import {
  advanceAfterRevealState,
  buildRoundState,
  createSession,
  createDrillState,
  DEFAULT_ROUND_SIZE,
  getAnswerFeedbackState,
  getFilteredWords,
  getNextRevealState,
  loadDrillData,
  postAnswer,
  serializeSessionState,
  updateSessionState,
} from './drill-state.js';
import {
  createDrillElements,
  positionSidebarTooltip,
  renderDrill,
  syncRestartFilterButtons,
  updateFilterHint,
} from './drill-view.js';
import { renderReading } from './lexicon-utils.js';

const els = createDrillElements();
const state = createDrillState(DRILL_FILTER_KEYS);
const DRILL_AUDIO_OPTIONS = { preferSynthesis: true, fallbackToBrowserTts: true };
state.prefetchController = null;
state.answerQueue = Promise.resolve();

async function prefetchRoundAudio(remaining) {
  if (state.prefetchController) state.prefetchController.abort();
  state.prefetchController = new AbortController();
  const { signal } = state.prefetchController;
  const available = await checkVoicevoxAvailable();
  if (signal.aborted) return;
  const vv = getVoicevoxSettings();
  // Skip remaining[0] (the current word) — auto-play fetches it on demand.
  // Prefetch sequentially so prefetch requests don't crowd out the current word's synthesis.
  for (const word of remaining.slice(1, 10)) {
    if (signal.aborted) return;
    await getSynthAudio(word.word, vv, signal).catch(() => {});
    if (word.exampleJp) {
      if (signal.aborted) return;
      await getSynthAudio(word.exampleJp, vv, signal).catch(() => {});
    }
  }
}

function playDrillWordAudio(word, rate = 1) {
  return playWordAudio(word, rate, DRILL_AUDIO_OPTIONS);
}

function playDrillSentenceAudio(word, rate = 1) {
  return playSentenceAudio(word, rate, DRILL_AUDIO_OPTIONS);
}

function shuffle(items) {
  const shuffled = [...items];
  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
  }
  return shuffled;
}

function getCurrentFilteredWords() {
  return getFilteredWords(state.words, state.activeFilters, DRILL_FILTER_KEYS);
}

function refreshFilterHint() {
  updateFilterHint(
    els,
    state.activeFilters,
    getCurrentFilteredWords().length,
    state.words.length,
    DRILL_FILTER_KEYS.length
  );
}

function restoreSession(session) {
  state.sessionId = session.id;
  const startedAt = Date.parse(session.startedAt);
  state.drillStartedAt = Number.isNaN(startedAt) ? Date.now() : startedAt;

  const sessionState = session.state || {};
  state.poolSize = sessionState.poolSize || 0;
  state.requestedRoundSize = sessionState.requestedRoundSize || state.requestedRoundSize || DEFAULT_ROUND_SIZE;
  state.roundSize = sessionState.roundSize || DEFAULT_ROUND_SIZE;
  state.round = sessionState.round || 1;
  state.doneCount = sessionState.doneCount || 0;
  state.pool = Array.isArray(sessionState.pool) ? sessionState.pool : [];
  state.redo = Array.isArray(sessionState.redo) ? sessionState.redo : [];
  state.remaining = Array.isArray(sessionState.remaining) ? sessionState.remaining : [];
  state.currentWord = state.remaining[0] || null;
  state.skipAnswerReveal = sessionState.skipAnswerReveal === true;
  if (typeof sessionState.matchingPairsMode === 'boolean') {
    state.matchingPairsMode = sessionState.matchingPairsMode;
  }
  state.awaitingAdvance = sessionState.awaitingAdvance === true;
  state.pendingAnswerCorrect = typeof sessionState.pendingAnswerCorrect === 'boolean'
    ? sessionState.pendingAnswerCorrect
    : null;
  state.sidebarFlash = null;
  state.sidebarItems = Array.isArray(sessionState.sidebarItems) ? sessionState.sidebarItems : [];
  state.lastAnswered = sessionState.lastAnswered || null;

  if (Array.isArray(sessionState.activeFilters) && sessionState.activeFilters.length > 0) {
    state.activeFilters.clear();
    sessionState.activeFilters.forEach(filterKey => state.activeFilters.add(filterKey));
  }

  syncRestartFilterButtons(els, state.activeFilters);
  renderDrill(els, state);
  prefetchRoundAudio(state.remaining);
}

async function init() {
  const { allWords, currentSession, settings } = await loadDrillData();

  state.words = allWords.filter(word => word.correct < word.target);
  state.settingsMaxWords = settings.maxWords;
  state.skipAnswerReveal = settings.skipAnswerReveal === true;
  state.matchingPairsMode = settings.matchingPairsMode === true;
  els.restartSkipAnswerReveal.checked = state.skipAnswerReveal;
  els.restartMatchingPairsMode.checked = state.matchingPairsMode;
  if (settings.roundSize > 0) {
    state.roundSize = settings.roundSize;
    state.requestedRoundSize = settings.roundSize;
  }
  if (Array.isArray(settings.wordTypes) && settings.wordTypes.length > 0) {
    state.activeFilters.clear();
    settings.wordTypes.forEach(filterKey => state.activeFilters.add(filterKey));
  }

  syncRestartFilterButtons(els, state.activeFilters);

  if (currentSession) {
    const sessionState = currentSession.state || {};
    const hasRestorableState = (sessionState.poolSize || 0) > 0 ||
      (Array.isArray(sessionState.remaining) && sessionState.remaining.length > 0);
    if (hasRestorableState) {
      restoreSession(currentSession);
      return;
    }
  }

  const filtered = getCurrentFilteredWords();
  const source = filtered.length > 0 ? filtered : state.words;
  state.poolSize = Math.min(settings.maxWords, source.length);
  state.pool = shuffle(source).slice(0, state.poolSize);
  state.lastAutoPlayedId = null;
  Object.assign(state, buildRoundState(state));
  state.lastAnswered = null;
  state.sidebarFlash = null;

  state.sessionId = await createSession(serializeSessionState(state));
  renderDrill(els, state);
  prefetchRoundAudio(state.remaining);
}

function reveal(knew) {
  if (!state.currentWord || state.awaitingAdvance) return;

  const answered = state.currentWord;
  if (state.skipAnswerReveal) {
    Object.assign(state, getNextRevealState(state, knew));
  } else {
    Object.assign(state, getAnswerFeedbackState(state, knew));
    playDrillWordAudio(answered).catch(() => {});
  }
  state.sidebarFlash = { word: answered.word, knew };
  renderDrill(els, state);
  if (!state.awaitingAdvance) prefetchRoundAudio(state.remaining);

  // Capture state snapshot now; queue the network call so answers are sent in
  // order without blocking the UI between answers.
  const sessionId = state.sessionId;
  const sessionSnapshot = serializeSessionState(state);
  state.answerQueue = state.answerQueue
    .then(() => postAnswer(sessionId, answered.id, knew, sessionSnapshot))
    .catch(err => console.error('Failed to save drill answer', err));
}

function advanceAfterReveal() {
  if (!state.awaitingAdvance) return;

  const nextState = advanceAfterRevealState(state);
  if (!nextState) return;

  Object.assign(state, nextState);
  state.sidebarFlash = null;
  renderDrill(els, state);
  prefetchRoundAudio(state.remaining);

  const sessionId = state.sessionId;
  const sessionSnapshot = serializeSessionState(state);
  state.answerQueue = state.answerQueue
    .then(() => updateSessionState(sessionId, sessionSnapshot))
    .catch(err => console.error('Failed to save drill session', err));
}

function openRestartModal() {
  els.restartTotalWords.value = state.settingsMaxWords;
  els.restartRoundSize.value = state.requestedRoundSize;
  els.restartSkipAnswerReveal.checked = state.skipAnswerReveal;
  els.restartMatchingPairsMode.checked = state.matchingPairsMode;
  refreshFilterHint();
  els.restartBackdrop.classList.remove('hidden');
}

function closeRestartModal() {
  els.restartBackdrop.classList.add('hidden');
}

function restartDrill(totalWords, roundSize, sourceWords) {
  state.poolSize = totalWords;
  state.roundSize = roundSize;
  state.pool = shuffle(sourceWords || state.words).slice(0, state.poolSize);
  state.round = 1;
  state.redo = [];
  state.doneCount = 0;
  state.drillStartedAt = Date.now();
  state.awaitingAdvance = false;
  state.lastAutoPlayedId = null;
  state.pendingAnswerCorrect = null;
  Object.assign(state, buildRoundState(state));
  state.lastAnswered = null;
  state.sidebarFlash = null;

  renderDrill(els, state);
  prefetchRoundAudio(state.remaining);
}

async function confirmRestart() {
  const filtered = getCurrentFilteredWords();
  const maxPoolSize = Math.max(1, parseInt(els.restartTotalWords.value, 10) || filtered.length);
  const total = Math.min(maxPoolSize, filtered.length);
  const requestedRoundSize = Math.max(1, parseInt(els.restartRoundSize.value, 10) || state.requestedRoundSize);
  const nextRoundSize = Math.max(1, Math.min(total, requestedRoundSize));
  state.requestedRoundSize = requestedRoundSize;
  state.skipAnswerReveal = els.restartSkipAnswerReveal.checked;
  state.matchingPairsMode = els.restartMatchingPairsMode.checked;
  closeRestartModal();
  restartDrill(total, nextRoundSize, filtered);
  state.sessionId = await createSession(serializeSessionState(state));
}

els.sidebarList.addEventListener('click', event => {
  const item = event.target.closest('.sidebar-item');
  if (!item?.dataset.word) return;
  const word = JSON.parse(item.dataset.word);
  playDrillWordAudio(word);
});

els.sidebarList.addEventListener('mouseover', event => {
  const item = event.target.closest('.sidebar-item');
  if (!item || !item.dataset.word) return;
  const word = JSON.parse(item.dataset.word);
  populateWordTooltip(els.tip, word, renderReading);
  positionSidebarTooltip(els, item, els.tip);
});

els.sidebarList.addEventListener('mouseout', event => {
  const item = event.target.closest('.sidebar-item');
  if (!item) return;
  if (!item.contains(event.relatedTarget)) els.tip.classList.remove('visible');
});

document.addEventListener('keydown', event => {
  if (event.key === 'Escape') {
    closeRestartModal();
    return;
  }
  if (event.key === 'w' || event.key === 'W') {
    if (state.currentWord) playDrillWordAudio(state.currentWord, 0.8);
    return;
  }
  if (event.key === 's' || event.key === 'S') {
    if (state.currentWord) playDrillSentenceAudio(state.currentWord, 0.8);
    return;
  }
  if (els.actionPrompt.style.display === 'none') return;
  if (state.awaitingAdvance) {
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === 'Shift' || event.key === 'CapsLock' || event.key === 'Tab') return;
    advanceAfterReveal();
    return;
  }
  if (event.key === 'd' || event.key === 'D') reveal(true);
  if (event.key === 'a' || event.key === 'A') reveal(false);
});

els.restartFilterButtons.forEach(btn => {
  btn.addEventListener('click', () => {
    const filterKey = btn.dataset.filter;
    if (state.activeFilters.has(filterKey)) state.activeFilters.delete(filterKey);
    else state.activeFilters.add(filterKey);
    btn.classList.toggle('active');
    refreshFilterHint();
  });
});

els.promptWordJp.addEventListener('click', () => {
  if (state.currentWord) playDrillWordAudio(state.currentWord);
});
els.promptExampleJp.addEventListener('click', () => {
  if (state.currentWord) playDrillSentenceAudio(state.currentWord);
});
els.lastWordJp.addEventListener('click', () => {
  if (state.lastAnswered) playDrillWordAudio(state.lastAnswered.word);
});
els.lastExampleJp.addEventListener('click', () => {
  if (state.lastAnswered) playDrillSentenceAudio(state.lastAnswered.word);
});
els.lastExampleEn.addEventListener('click', () => {
  if (state.lastAnswered?.word.exampleEn) playTts(state.lastAnswered.word.exampleEn, 'en-US');
});

els.headerRestartBtn.addEventListener('click', openRestartModal);
els.dontKnowBtn.addEventListener('click', () => reveal(false));
els.knowBtn.addEventListener('click', () => reveal(true));
els.nextBtn.addEventListener('click', advanceAfterReveal);

els.restartBackdrop.addEventListener('click', event => {
  if (event.target === els.restartBackdrop) closeRestartModal();
});
els.restartCloseBtn.addEventListener('click', closeRestartModal);
els.restartCancelBtn.addEventListener('click', closeRestartModal);
els.restartStartBtn.addEventListener('click', confirmRestart);

attachNumberStepper(els.restartTotalWords);
attachNumberStepper(els.restartRoundSize);

// Keeps the "began X minutes ago" timestamp in renderStats fresh.
setInterval(() => {
  renderDrill(els, state);
}, 30_000);


init();
