import {
  attachNumberStepper,
  checkVoicevoxAvailable,
  DRILL_FILTER_KEYS,
  getVoicevoxSettings,
  playTts,
  populateWordTooltip,
  playWordAudio,
  playSentenceAudio,
} from './common.js';
import { getSynthAudio } from './synth-cache.js';
import {
  advanceAfterRevealState,
  attemptMatchingPair,
  buildMatchingRoundState,
  buildRoundState,
  createSession,
  createDrillState,
  createEmptyMatchingState,
  DEFAULT_ROUND_SIZE,
  getAnswerFeedbackState,
  getFilteredWords,
  getNextRevealState,
  hasRestorableSessionState,
  loadDrillData,
  postAnswer,
  restoreMatchingSessionState,
  restoreStandardSessionState,
  selectMatchingWord,
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
state.hoveredMatchingWordId = null;
state.isPointerInMatchingWordList = false;
state.matchingTooltipPointer = null;

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

function getMatchingRoundWord(wordId) {
  return state.matchingRoundWords.find(word => word.id === wordId) || null;
}

function getSelectedMatchingWord() {
  if (typeof state.matchingSelectedWordId !== 'number') return null;
  return getMatchingRoundWord(state.matchingSelectedWordId);
}

function getHoveredMatchingWord() {
  if (typeof state.hoveredMatchingWordId !== 'number') return null;
  return getMatchingRoundWord(state.hoveredMatchingWordId);
}

function positionMatchingWordTooltip(clientX, clientY) {
  const pad = 8;
  const w = els.tip.offsetWidth;
  const h = els.tip.offsetHeight;
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  let left = clientX + 14;
  if (left + w > vw - pad) left = vw - w - pad;
  let top = clientY + 18;
  top = Math.max(pad, Math.min(top, vh - h - pad));
  els.tip.style.left = left + 'px';
  els.tip.style.top = top + 'px';
}

function showMatchingWordTooltip(event, word) {
  if (!event || !word) {
    els.tip.classList.remove('visible');
    return;
  }

  state.matchingTooltipPointer = {
    clientX: event.clientX,
    clientY: event.clientY,
  };

  els.tip.querySelector('[data-word-tooltip="word"]').textContent = word.word || '';
  els.tip.querySelector('[data-word-tooltip="reading"]').innerHTML =
    renderReading(word.reading, word.word, word.kanjiData, word.pitchAccent);
  els.tip.querySelector('[data-word-tooltip="pos"]').textContent = word.type || '';
  els.tip.querySelector('[data-word-tooltip="meaning"]').textContent = '';
  els.tip.querySelector('[data-word-tooltip="example"]').textContent = word.exampleJp;
  els.tip.querySelector('[data-word-tooltip="example-en"]').textContent = '';
  populateWordTooltipKanjiOnly(word);
  const imgEl = els.tip.querySelector('[data-word-tooltip="image"]');
  imgEl.removeAttribute('src');
  imgEl.style.display = 'none';
  els.tip.style.left = '-9999px';
  els.tip.style.top = '-9999px';
  els.tip.classList.add('visible');
  positionMatchingWordTooltip(event.clientX, event.clientY);
}

function showHoveredMatchingWordTooltip() {
  const word = getHoveredMatchingWord();
  if (!word || !state.matchingTooltipPointer) {
    els.tip.classList.remove('visible');
    return;
  }

  showMatchingWordTooltip(state.matchingTooltipPointer, word);
}

function showSelectedMatchingWordTooltip() {
  if (!state.matchingPairsMode) {
    els.tip.classList.remove('visible');
    return;
  }

  const word = getSelectedMatchingWord();
  if (!word) {
    els.tip.classList.remove('visible');
    return;
  }

  if (!state.matchingTooltipPointer) {
    els.tip.classList.remove('visible');
    return;
  }

  els.tip.querySelector('[data-word-tooltip="word"]').textContent = word.word || '';
  els.tip.querySelector('[data-word-tooltip="reading"]').innerHTML =
    renderReading(word.reading, word.word, word.kanjiData, word.pitchAccent);
  els.tip.querySelector('[data-word-tooltip="pos"]').textContent = word.type || '';
  els.tip.querySelector('[data-word-tooltip="meaning"]').textContent = '';
  els.tip.querySelector('[data-word-tooltip="example"]').textContent = word.exampleJp;
  els.tip.querySelector('[data-word-tooltip="example-en"]').textContent = '';
  populateWordTooltipKanjiOnly(word);
  const imgEl = els.tip.querySelector('[data-word-tooltip="image"]');
  imgEl.removeAttribute('src');
  imgEl.style.display = 'none';
  els.tip.style.left = '-9999px';
  els.tip.style.top = '-9999px';
  els.tip.classList.add('visible');
  positionMatchingWordTooltip(state.matchingTooltipPointer.clientX, state.matchingTooltipPointer.clientY);
}

function renderDrillUI() {
  renderDrill(els, state);
  if (state.matchingPairsMode && state.isPointerInMatchingWordList) {
    showHoveredMatchingWordTooltip();
    return;
  }
  showSelectedMatchingWordTooltip();
}

function populateWordTooltipKanjiOnly(word) {
  const kanjiEl = els.tip.querySelector('[data-word-tooltip="kanji"]');
  kanjiEl.innerHTML = '';
  (word.kanjiData || []).forEach(entry => {
    const div = document.createElement('div');
    div.className = 'kanji-entry';
    div.innerHTML =
      '<div class="kanji-char">' + (entry.character || '') + '</div>' +
      '<div class="kanji-detail">' +
        '<div class="kanji-readings">' + (entry.reading || '') + '</div>' +
        '<div class="kanji-meanings">' + ((entry.meanings || []).join(', ')) + '</div>' +
      '</div>';
    kanjiEl.appendChild(div);
  });
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
  state.skipAnswerReveal = sessionState.skipAnswerReveal === true;
  state.sidebarFlash = null;
  state.matchingPairsMode = sessionState.matchingPairsMode === true;

  Object.assign(
    state,
    state.matchingPairsMode
      ? restoreMatchingSessionState(sessionState)
      : restoreStandardSessionState(sessionState)
  );

  if (Array.isArray(sessionState.activeFilters) && sessionState.activeFilters.length > 0) {
    state.activeFilters.clear();
    sessionState.activeFilters.forEach(filterKey => state.activeFilters.add(filterKey));
  }

  syncRestartFilterButtons(els, state.activeFilters);
  renderDrillUI();
  if (state.matchingPairsMode) {
    state.prefetchController?.abort();
  } else if (state.awaitingAdvance) {
    state.prefetchController?.abort();
  } else {
    prefetchRoundAudio(state.remaining);
  }
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
    if (hasRestorableSessionState(sessionState)) {
      restoreSession(currentSession);
      return;
    }
  }

  const filtered = getCurrentFilteredWords();
  const source = filtered.length > 0 ? filtered : state.words;
  state.poolSize = Math.min(settings.maxWords, source.length);
  state.pool = shuffle(source).slice(0, state.poolSize);
  state.lastAutoPlayedId = null;
  Object.assign(state, state.matchingPairsMode
    ? buildMatchingRoundState(state, shuffle)
    : {
      ...buildRoundState(state),
      ...createEmptyMatchingState(),
    });
  state.lastAnswered = null;
  state.sidebarFlash = null;

  state.sessionId = await createSession(serializeSessionState(state));
  renderDrillUI();
  if (state.matchingPairsMode) state.prefetchController?.abort();
  else prefetchRoundAudio(state.remaining);
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
  renderDrillUI();
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
  if (state.matchingPairsMode) return;
  if (!state.awaitingAdvance) return;

  const nextState = advanceAfterRevealState(state);
  if (!nextState) return;

  Object.assign(state, nextState);
  state.sidebarFlash = null;
  renderDrillUI();
  prefetchRoundAudio(state.remaining);

  const sessionId = state.sessionId;
  const sessionSnapshot = serializeSessionState(state);
  state.answerQueue = state.answerQueue
    .then(() => updateSessionState(sessionId, sessionSnapshot))
    .catch(err => console.error('Failed to save drill session', err));
}

function queueSessionStateSave() {
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
  Object.assign(state, state.matchingPairsMode
    ? buildMatchingRoundState(state, shuffle)
    : {
      ...buildRoundState(state),
      ...createEmptyMatchingState(),
    });
  state.lastAnswered = null;
  state.sidebarFlash = null;

  renderDrillUI();
  if (state.matchingPairsMode) state.prefetchController?.abort();
  else prefetchRoundAudio(state.remaining);
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
  if (state.matchingPairsMode) return;
  const word = JSON.parse(item.dataset.word);
  playDrillWordAudio(word);
});

els.sidebarList.addEventListener('mouseover', event => {
  const item = event.target.closest('.sidebar-item');
  if (!item || !item.dataset.word) return;
  if (state.matchingPairsMode) return;
  const word = JSON.parse(item.dataset.word);
  populateWordTooltip(els.tip, word, renderReading);
  positionSidebarTooltip(els, item, els.tip);
});

els.sidebarList.addEventListener('mouseout', event => {
  const item = event.target.closest('.sidebar-item');
  if (!item) return;
  if (state.matchingPairsMode) return;
  if (!item.contains(event.relatedTarget)) els.tip.classList.remove('visible');
});

els.matchingWordList.addEventListener('click', event => {
  const button = event.target.closest('[data-word-id]');
  if (!button || !state.matchingPairsMode) return;
  const wordId = parseInt(button.dataset.wordId, 10);
  if (Number.isNaN(wordId)) return;
  const word = getMatchingRoundWord(wordId);
  const nextState = selectMatchingWord(state, wordId);
  if (nextState === state) return;
  Object.assign(state, nextState);
  renderDrillUI();
  queueSessionStateSave();
  if (word) playDrillWordAudio(word).catch(() => {});
});

els.matchingWordList.addEventListener('mouseover', event => {
  const button = event.target.closest('[data-word-id]');
  if (!button || !state.matchingPairsMode) return;
  state.isPointerInMatchingWordList = true;
  const wordId = parseInt(button.dataset.wordId, 10);
  if (Number.isNaN(wordId)) return;
  state.hoveredMatchingWordId = wordId;
  showMatchingWordTooltip(event, getMatchingRoundWord(wordId));
});

els.matchingWordList.addEventListener('mousemove', event => {
  if (!state.matchingPairsMode) return;
  state.isPointerInMatchingWordList = true;
  state.matchingTooltipPointer = {
    clientX: event.clientX,
    clientY: event.clientY,
  };
  const button = event.target.closest('[data-word-id]');
  if (button) {
    const wordId = parseInt(button.dataset.wordId, 10);
    if (!Number.isNaN(wordId)) state.hoveredMatchingWordId = wordId;
  }
  if (!els.tip.classList.contains('visible')) return;
  if (state.hoveredMatchingWordId !== null) {
    showHoveredMatchingWordTooltip();
  } else {
    positionMatchingWordTooltip(event.clientX, event.clientY);
  }
});

els.matchingWordList.addEventListener('mouseout', event => {
  const button = event.target.closest('[data-word-id]');
  if (!button || !state.matchingPairsMode) return;
  if (!button.contains(event.relatedTarget)) showHoveredMatchingWordTooltip();
});

els.matchingWordList.addEventListener('mouseleave', () => {
  if (!state.matchingPairsMode) return;
  state.isPointerInMatchingWordList = false;
  state.hoveredMatchingWordId = null;
  showSelectedMatchingWordTooltip();
});

document.addEventListener('mousemove', event => {
  if (!state.matchingPairsMode) return;
  state.matchingTooltipPointer = {
    clientX: event.clientX,
    clientY: event.clientY,
  };
  if (event.target.closest?.('#matching-word-list')) return;
  if (!getSelectedMatchingWord()) {
    els.tip.classList.remove('visible');
    return;
  }
  showSelectedMatchingWordTooltip();
});

els.matchingInfoList.addEventListener('click', event => {
  const card = event.target.closest('[data-info-id]');
  if (!card || !state.matchingPairsMode) return;
  const infoWordId = parseInt(card.dataset.infoId, 10);
  if (Number.isNaN(infoWordId)) return;

  const result = attemptMatchingPair(state, infoWordId, shuffle);
  if (!result) return;

  Object.assign(state, result.nextState);
  renderDrillUI();

  const sessionId = state.sessionId;
  const sessionSnapshot = serializeSessionState(state);
  if (result.firstAttempt) {
    state.answerQueue = state.answerQueue
      .then(() => postAnswer(sessionId, result.answeredWord.id, result.firstAttemptCorrect, sessionSnapshot))
      .catch(err => console.error('Failed to save drill answer', err));
    return;
  }
  queueSessionStateSave();
});

document.addEventListener('keydown', event => {
  if (event.key === 'Escape') {
    closeRestartModal();
    return;
  }
  if (state.matchingPairsMode) {
    if (event.key === 'w' || event.key === 'W') {
      const word = getSelectedMatchingWord();
      if (word) playDrillWordAudio(word, 0.8);
    }
    if (event.key === 's' || event.key === 'S') {
      const word = getSelectedMatchingWord();
      if (word?.exampleJp) playDrillSentenceAudio(word, 0.8);
    }
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
  if (state.matchingPairsMode) return;
  if (state.currentWord) playDrillWordAudio(state.currentWord);
});
els.promptExampleJp.addEventListener('click', () => {
  if (state.matchingPairsMode) return;
  if (state.currentWord) playDrillSentenceAudio(state.currentWord);
});
els.lastWordJp.addEventListener('click', () => {
  if (state.matchingPairsMode) return;
  if (state.lastAnswered) playDrillWordAudio(state.lastAnswered.word);
});
els.lastExampleJp.addEventListener('click', () => {
  if (state.matchingPairsMode) return;
  if (state.lastAnswered) playDrillSentenceAudio(state.lastAnswered.word);
});
els.lastExampleEn.addEventListener('click', () => {
  if (state.matchingPairsMode) return;
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
  renderDrillUI();
}, 30_000);


init();
