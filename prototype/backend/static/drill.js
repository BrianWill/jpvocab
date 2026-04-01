import {
  attachNumberStepper,
  DRILL_FILTER_KEYS,
  populateWordTooltip,
} from './common.js';
import {
  buildRoundState,
  createSession,
  createDrillState,
  DEFAULT_ROUND_SIZE,
  getFilteredWords,
  getNextRevealState,
  loadDrillData,
  postAnswer,
  serializeSessionState,
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
  state.roundSize = sessionState.roundSize || DEFAULT_ROUND_SIZE;
  state.round = sessionState.round || 1;
  state.doneCount = sessionState.doneCount || 0;
  state.pool = Array.isArray(sessionState.pool) ? sessionState.pool : [];
  state.redo = Array.isArray(sessionState.redo) ? sessionState.redo : [];
  state.remaining = Array.isArray(sessionState.remaining) ? sessionState.remaining : [];
  state.currentWord = state.remaining[0] || null;
  state.sidebarFlash = null;
  state.sidebarItems = Array.isArray(sessionState.sidebarItems) ? sessionState.sidebarItems : [];
  state.lastAnswered = sessionState.lastAnswered || null;

  if (Array.isArray(sessionState.activeFilters) && sessionState.activeFilters.length > 0) {
    state.activeFilters.clear();
    sessionState.activeFilters.forEach(filterKey => state.activeFilters.add(filterKey));
  }

  syncRestartFilterButtons(els, state.activeFilters);
  renderDrill(els, state);
}

async function init() {
  const { allWords, currentSession, kanjiList, settings } = await loadDrillData();

  state.kanjiMap = {};
  kanjiList.forEach(kanji => { state.kanjiMap[kanji.id] = kanji; });

  state.words = allWords.filter(word => word.correct < word.target);
  state.settingsMaxWords = settings.maxWords;
  if (settings.roundSize > 0) state.roundSize = settings.roundSize;
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
  Object.assign(state, buildRoundState(state));
  state.lastAnswered = null;
  state.sidebarFlash = null;

  state.sessionId = await createSession(serializeSessionState(state));
  renderDrill(els, state);
}

async function reveal(knew) {
  if (!state.currentWord || state.isSubmittingAnswer) return;
  state.isSubmittingAnswer = true;

  const answered = state.currentWord;
  Object.assign(state, getNextRevealState(state, knew));
  state.sidebarFlash = { word: answered.word, knew };
  renderDrill(els, state);

  try {
    await postAnswer(state.sessionId, answered.id, knew, serializeSessionState(state));
  } finally {
    state.isSubmittingAnswer = false;
  }
}

function openRestartModal() {
  els.restartTotalWords.value = state.settingsMaxWords;
  els.restartRoundSize.value = state.roundSize;
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
  Object.assign(state, buildRoundState(state));
  state.lastAnswered = null;
  state.sidebarFlash = null;

  renderDrill(els, state);
}

async function confirmRestart() {
  const filtered = getCurrentFilteredWords();
  const maxPoolSize = Math.max(1, parseInt(els.restartTotalWords.value, 10) || filtered.length);
  const total = Math.min(maxPoolSize, filtered.length);
  const nextRoundSize = Math.max(1, Math.min(total, parseInt(els.restartRoundSize.value, 10) || state.roundSize));
  closeRestartModal();
  restartDrill(total, nextRoundSize, filtered);
  state.sessionId = await createSession(serializeSessionState(state));
}

els.sidebarList.addEventListener('mouseover', event => {
  const item = event.target.closest('.sidebar-item');
  if (!item || !item.dataset.word) return;
  const word = JSON.parse(item.dataset.word);
  populateWordTooltip(els.tip, word, state.kanjiMap, renderReading);
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
  if (els.actionPrompt.style.display === 'none') return;
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

els.headerRestartBtn.addEventListener('click', openRestartModal);
els.dontKnowBtn.addEventListener('click', () => reveal(false));
els.knowBtn.addEventListener('click', () => reveal(true));

els.restartBackdrop.addEventListener('click', event => {
  if (event.target === els.restartBackdrop) closeRestartModal();
});
els.restartCloseBtn.addEventListener('click', closeRestartModal);
els.restartCancelBtn.addEventListener('click', closeRestartModal);
els.restartStartBtn.addEventListener('click', confirmRestart);

attachNumberStepper(els.restartTotalWords);
attachNumberStepper(els.restartRoundSize);

setInterval(() => {
  renderDrill(els, state);
}, 30_000);

init();
