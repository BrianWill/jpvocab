import { populateWordTooltip, positionAnchoredWordTooltip, renderWordTooltipKanji } from './common.js';
import { renderReading } from './lexicon-utils.js';

const FILTER_KEYS = ['katakana', 'verbs', 'nouns', 'other'];
const DEFAULT_ROUND_SIZE = 10;
const STEP_INTERVAL = 230;

const els = {
  actionPrompt: document.getElementById('action-prompt'),
  headerBegan: document.getElementById('header-began'),
  lastExampleEn: document.getElementById('last-example-en'),
  lastExampleJp: document.getElementById('last-example-jp'),
  lastKanjiInfo: document.getElementById('last-kanji-info'),
  lastMeaning: document.getElementById('last-meaning'),
  lastPos: document.getElementById('last-pos'),
  lastReading: document.getElementById('last-reading'),
  lastWordCard: document.getElementById('last-word-card'),
  lastWordImage: document.getElementById('last-word-image'),
  lastWordJp: document.getElementById('last-word-jp'),
  progressBar: document.querySelector('.progress-bar'),
  promptExampleJp: document.getElementById('prompt-example-jp'),
  promptWordJp: document.getElementById('prompt-word-jp'),
  restartBackdrop: document.getElementById('restart-modal-backdrop'),
  restartRoundSize: document.getElementById('restart-round-size'),
  restartStartBtn: document.getElementById('restart-start-btn'),
  restartTotalWords: document.getElementById('restart-total-words'),
  sidebar: document.querySelector('.sidebar'),
  sidebarList: document.getElementById('sidebar-list'),
  sidebarTitle: document.getElementById('sidebar-title'),
  statToGo: document.getElementById('stat-togo'),
  tip: document.getElementById('tooltip'),
  filterHint: document.getElementById('filter-hint'),
  headerRestartBtn: document.querySelector('.btn-header'),
  knowBtn: document.querySelector('.btn-yes'),
  dontKnowBtn: document.querySelector('.btn-no'),
};

els.restartFilterButtons = Array.from(
  els.restartBackdrop.querySelectorAll('.filter-chip[data-filter]')
);
els.restartCloseBtn = els.restartBackdrop.querySelector('.modal-close');
els.restartCancelBtn = els.restartBackdrop.querySelector('.btn-cancel');

const state = {
  activeFilters: new Set(FILTER_KEYS),
  currentWord: null,
  doneCount: 0,
  drillStartedAt: Date.now(),
  isSubmittingAnswer: false,
  kanjiMap: {},
  lastAnswered: null,
  maxPoolSize: 0,
  pool: [],
  poolSize: 0,
  redo: [],
  remaining: [],
  round: 1,
  roundSize: DEFAULT_ROUND_SIZE,
  sessionId: null,
  settingsMaxWords: null,
  sidebarItems: [],
  stepTimer: null,
  words: [],
};

function matchesFilter(w, f) {
  const isKatakana = /^[\u30A0-\u30FF]+$/.test(w.word);
  const isVerb = w.type === 'ichidan-verb' || w.type === 'godan-verb';
  const isNoun = w.type === 'noun';
  if (f === 'katakana') return isKatakana;
  if (f === 'verbs') return isVerb;
  if (f === 'nouns') return isNoun;
  if (f === 'other') return !isKatakana && !isVerb && !isNoun;
  return false;
}

function getFilteredWords() {
  return state.words.filter(w => FILTER_KEYS.some(f => state.activeFilters.has(f) && matchesFilter(w, f)));
}

function syncRestartFilterButtons() {
  els.restartFilterButtons.forEach(btn => {
    btn.classList.toggle('active', state.activeFilters.has(btn.dataset.filter));
  });
}

function updateFilterHint() {
  if (state.activeFilters.size === 0) {
    els.filterHint.textContent = 'Select at least one word type';
    els.filterHint.classList.add('error');
    els.restartStartBtn.disabled = true;
    return;
  }

  const count = getFilteredWords().length;
  els.filterHint.textContent = state.activeFilters.size === FILTER_KEYS.length
    ? 'All ' + count + ' words'
    : count + ' of ' + state.words.length + ' words';
  els.filterHint.classList.remove('error');
  els.restartStartBtn.disabled = false;
}

function shuffle(arr) {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

function timeAgo(date) {
  const sec = Math.floor((Date.now() - date) / 1000);
  const min = Math.floor(sec / 60);
  if (min < 1) return 'just now';
  if (min < 60) return min + ' minute' + (min === 1 ? '' : 's') + ' ago';
  const hr = Math.floor(min / 60);
  if (hr < 24) return hr + ' hour' + (hr === 1 ? '' : 's') + ' ago';
  const day = Math.floor(hr / 24);
  return day + ' day' + (day === 1 ? '' : 's') + ' ago';
}

function createSidebarItems(words, redoSet = new Set()) {
  return words.map(word => ({
    word,
    status: redoSet.has(word.word) ? 'unseen-redo' : 'unseen',
  }));
}

function applySidebarAnswer(sidebarItems, word, knew) {
  const status = knew ? 'known flash-known' : 'missed flash-missed';
  let found = false;
  const nextItems = sidebarItems.map(item => {
    const baseStatus = item.status.replace(/\sflash-(known|missed)\b/g, '');
    if (item.word.word !== word.word) {
      return baseStatus === item.status ? item : { ...item, status: baseStatus };
    }
    found = true;
    return { ...item, word, status };
  });
  if (!found) nextItems.push({ word, status });
  return nextItems;
}

function isSessionComplete(sessionState) {
  return !sessionState.currentWord &&
    sessionState.remaining.length === 0 &&
    sessionState.redo.length === 0 &&
    sessionState.pool.length === 0;
}

function buildRoundState(sessionState) {
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

function getNextRevealState(sessionState, knew) {
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

function updateStats() {
  els.statToGo.textContent = (state.poolSize - state.doneCount) + ' to go of ' + state.poolSize;
  els.sidebarTitle.textContent = 'Round ' + state.round;
  els.headerBegan.textContent = 'began ' + timeAgo(state.drillStartedAt);

  const pct = state.poolSize > 0 ? (state.doneCount / state.poolSize) * 100 : 0;
  els.progressBar.style.width = pct + '%';
}

function renderPrompt() {
  els.sidebarList.querySelectorAll('.sidebar-item.current').forEach(el => el.classList.remove('current'));
  if (isSessionComplete(state)) {
    if (state.poolSize === 0) {
      els.promptWordJp.textContent = 'No words to drill';
      els.promptExampleJp.textContent = 'There are no active words available with current drill settings.';
    } else {
      els.promptWordJp.textContent = 'Done!';
      els.promptExampleJp.textContent = 'All words cleared.';
    }
    els.actionPrompt.style.display = 'none';
    return;
  }

  els.actionPrompt.style.display = '';
  if (!state.currentWord) return;

  els.promptWordJp.textContent = state.currentWord.word;
  els.promptExampleJp.textContent = state.currentWord.exampleJp;
  const item = els.sidebarList.querySelector('[data-id="' + state.currentWord.word + '"]');
  if (item) item.classList.add('current');
}

function renderSidebar() {
  els.sidebarList.innerHTML = '';
  state.sidebarItems.forEach(itemData => {
    const li = document.createElement('li');
    li.className = 'sidebar-item ' + itemData.status;
    li.textContent = itemData.word.word;
    li.dataset.word = JSON.stringify(itemData.word);
    li.dataset.id = itemData.word.word;
    li.addEventListener('animationend', () => li.classList.remove('flash-known', 'flash-missed'), { once: true });
    els.sidebarList.appendChild(li);
  });
}

function renderLastAnswered() {
  if (!state.lastAnswered) {
    els.lastWordCard.style.display = 'none';
    return;
  }

  const answered = state.lastAnswered.word;
  els.lastWordCard.style.display = '';
  els.lastWordJp.textContent = answered.word;
  els.lastWordJp.className = 'tooltip-word ' + (state.lastAnswered.knew ? 'knew' : 'missed');
  els.lastReading.innerHTML = renderReading(answered.reading, answered.word, answered.kanjiData);
  els.lastPos.textContent = answered.type;
  els.lastMeaning.textContent = answered.meaning;
  els.lastExampleJp.textContent = answered.exampleJp;
  els.lastExampleEn.textContent = answered.exampleEn;
  renderWordTooltipKanji(els.lastKanjiInfo, answered, state.kanjiMap);
  if (answered.imagePath) {
    els.lastWordImage.src = '/static/' + answered.imagePath;
    els.lastWordImage.style.display = '';
  } else {
    els.lastWordImage.style.display = 'none';
  }
}

function renderDrill() {
  renderSidebar();
  renderLastAnswered();
  updateStats();
  renderPrompt();
}

function getSessionState() {
  return {
    poolSize: state.poolSize,
    maxPoolSize: state.maxPoolSize,
    settingsMaxWords: state.settingsMaxWords,
    roundSize: state.roundSize,
    round: state.round,
    doneCount: state.doneCount,
    activeFilters: [...state.activeFilters],
    pool: state.pool,
    redo: state.redo,
    remaining: state.remaining,
    sidebarItems: state.sidebarItems.map(item => ({
      word: item.word,
      status: item.status.replace(/\sflash-(known|missed)\b/g, ''),
    })),
    lastAnswered: state.lastAnswered,
    completed: isSessionComplete(state),
  };
}

async function createSession(state) {
  const resp = await fetch('/api/drill/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ state }),
  });
  const data = await resp.json();
  return data.id;
}

async function getCurrentSession() {
  const resp = await fetch('/api/drill/sessions/current');
  const data = await resp.json();
  return data.session;
}

async function postAnswer(wordId, correct, sessionState) {
  if (!state.sessionId) return;
  const resp = await fetch('/api/drill/sessions/' + state.sessionId + '/answers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ wordId, correct, state: sessionState }),
  });
  if (!resp.ok) throw new Error('failed to save drill answer');
}

async function reveal(knew) {
  if (!state.currentWord || state.isSubmittingAnswer) return;
  state.isSubmittingAnswer = true;

  const answered = state.currentWord;
  Object.assign(state, getNextRevealState(state, knew));
  renderDrill();

  try {
    await postAnswer(answered.id, knew, getSessionState());
  } finally {
    state.isSubmittingAnswer = false;
  }
}

function positionSidebarTooltip(item, tip) {
  const itemRect = item.getBoundingClientRect();
  const sidebarRect = els.sidebar.getBoundingClientRect();
  positionAnchoredWordTooltip(tip, {
    anchorRect: itemRect,
    left: sidebarRect.right - 14,
  });
}

function restoreSession(session) {
  state.sessionId = session.id;
  const startedAt = Date.parse(session.startedAt);
  state.drillStartedAt = Number.isNaN(startedAt) ? Date.now() : startedAt;

  const sessionState = session.state || {};
  state.poolSize = sessionState.poolSize || 0;
  state.maxPoolSize = sessionState.maxPoolSize || 0;
  state.settingsMaxWords = sessionState.settingsMaxWords > 0 ? sessionState.settingsMaxWords : state.settingsMaxWords;
  state.roundSize = sessionState.roundSize || DEFAULT_ROUND_SIZE;
  state.round = sessionState.round || 1;
  state.doneCount = sessionState.doneCount || 0;
  state.pool = Array.isArray(sessionState.pool) ? sessionState.pool : [];
  state.redo = Array.isArray(sessionState.redo) ? sessionState.redo : [];
  state.remaining = Array.isArray(sessionState.remaining) ? sessionState.remaining : [];
  state.currentWord = state.remaining[0] || null;
  state.sidebarItems = Array.isArray(sessionState.sidebarItems) ? sessionState.sidebarItems : [];
  state.lastAnswered = sessionState.lastAnswered || null;

  if (Array.isArray(sessionState.activeFilters) && sessionState.activeFilters.length > 0) {
    state.activeFilters.clear();
    sessionState.activeFilters.forEach(f => state.activeFilters.add(f));
  }
  syncRestartFilterButtons();
  renderDrill();
}

async function init() {
  const [wordsResp, kanjiResp, settingsResp, currentSession] = await Promise.all([
    fetch('/api/words'),
    fetch('/api/kanji'),
    fetch('/api/settings/drill'),
    getCurrentSession(),
  ]);
  const allWords = await wordsResp.json();
  const kanjiList = await kanjiResp.json();
  const settings = await settingsResp.json();

  state.kanjiMap = {};
  kanjiList.forEach(k => { state.kanjiMap[k.id] = k; });

  state.words = allWords.filter(w => w.correct < w.target);

  state.settingsMaxWords = settings.maxWords;
  if (settings.roundSize > 0) state.roundSize = settings.roundSize;
  if (Array.isArray(settings.wordTypes) && settings.wordTypes.length > 0) {
    state.activeFilters.clear();
    settings.wordTypes.forEach(f => state.activeFilters.add(f));
  }
  syncRestartFilterButtons();

  if (currentSession) {
    const sessionState = currentSession.state || {};
    const hasRestorableState = (sessionState.poolSize || 0) > 0 || (Array.isArray(sessionState.remaining) && sessionState.remaining.length > 0);
    if (hasRestorableState) {
      restoreSession(currentSession);
      return;
    }
  }

  const filtered = getFilteredWords();
  const source = filtered.length > 0 ? filtered : state.words;
  state.maxPoolSize = Math.min(settings.maxWords, source.length);
  state.poolSize = state.maxPoolSize;
  state.pool = shuffle([...source]).slice(0, state.poolSize);
  Object.assign(state, buildRoundState(state));
  state.lastAnswered = null;

  state.sessionId = await createSession(getSessionState());
  renderDrill();
}

function startStep(fn, ...args) {
  fn(...args);
  state.stepTimer = setInterval(() => fn(...args), STEP_INTERVAL);
}

function stopStep() {
  clearInterval(state.stepTimer);
  state.stepTimer = null;
}

function adjustRestart(id, delta) {
  const input = document.getElementById(id);
  const val = parseInt(input.value, 10) || 5;
  input.value = delta > 0
    ? Math.min(995, Math.floor(val / 5) * 5 + 5)
    : Math.max(5, Math.ceil(val / 5) * 5 - 5);
}

function capRestartInput(input) {
  if (input.value.length > 3) input.value = input.value.slice(0, 3);
  if (input.value === '0') input.value = '1';
}

function openRestartModal() {
  els.restartTotalWords.value = state.settingsMaxWords;
  els.restartRoundSize.value = state.roundSize;
  updateFilterHint();
  els.restartBackdrop.classList.remove('hidden');
}

function closeRestartModal() {
  els.restartBackdrop.classList.add('hidden');
}

function handleRestartBackdropClick(e) {
  if (e.target === els.restartBackdrop) closeRestartModal();
}

function restartDrill(totalWords, newRoundSize, sourceWords) {
  state.poolSize = totalWords;
  state.roundSize = newRoundSize;
  state.pool = shuffle([...(sourceWords || state.words)]).slice(0, state.poolSize);
  state.round = 1;
  state.redo = [];
  state.doneCount = 0;
  state.drillStartedAt = Date.now();
  Object.assign(state, buildRoundState(state));
  state.lastAnswered = null;

  renderDrill();
}

async function confirmRestart() {
  const filtered = getFilteredWords();
  state.maxPoolSize = Math.max(1, parseInt(els.restartTotalWords.value, 10) || filtered.length);
  const total = Math.min(state.maxPoolSize, filtered.length);
  const rSize = Math.max(1, Math.min(total, parseInt(els.restartRoundSize.value, 10) || state.roundSize));
  closeRestartModal();
  restartDrill(total, rSize, filtered);
  state.sessionId = await createSession(getSessionState());
}

els.sidebarList.addEventListener('mouseover', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item || !item.dataset.word) return;
  const data = JSON.parse(item.dataset.word);
  populateWordTooltip(els.tip, data, state.kanjiMap, renderReading);
  positionSidebarTooltip(item, els.tip);
});
els.sidebarList.addEventListener('mouseout', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item) return;
  if (!item.contains(e.relatedTarget)) els.tip.classList.remove('visible');
});

init();

setInterval(() => {
  els.headerBegan.textContent = 'began ' + timeAgo(state.drillStartedAt);
}, 30_000);

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    closeRestartModal();
    return;
  }
  if (els.actionPrompt.style.display === 'none') return;
  if (e.key === 'd' || e.key === 'D') reveal(true);
  if (e.key === 'a' || e.key === 'A') reveal(false);
});

els.restartFilterButtons.forEach(btn => {
  btn.addEventListener('click', () => {
    const f = btn.dataset.filter;
    if (state.activeFilters.has(f)) state.activeFilters.delete(f);
    else state.activeFilters.add(f);
    btn.classList.toggle('active');
    updateFilterHint();
  });
});

els.headerRestartBtn.addEventListener('click', openRestartModal);
els.dontKnowBtn.addEventListener('click', () => reveal(false));
els.knowBtn.addEventListener('click', () => reveal(true));

els.restartBackdrop.addEventListener('click', handleRestartBackdropClick);
els.restartCloseBtn.addEventListener('click', closeRestartModal);
els.restartCancelBtn.addEventListener('click', closeRestartModal);
els.restartStartBtn.addEventListener('click', confirmRestart);

const totalInput = els.restartTotalWords;
const [totalMinus, totalPlus] = totalInput.closest('.num-stepper').querySelectorAll('.num-btn');
totalMinus.addEventListener('mousedown', () => startStep(adjustRestart, 'restart-total-words', -5));
totalMinus.addEventListener('mouseup', stopStep);
totalMinus.addEventListener('mouseleave', stopStep);
totalPlus.addEventListener('mousedown', () => startStep(adjustRestart, 'restart-total-words', 5));
totalPlus.addEventListener('mouseup', stopStep);
totalPlus.addEventListener('mouseleave', stopStep);
totalInput.addEventListener('input', () => capRestartInput(totalInput));

const roundInput = els.restartRoundSize;
const [roundMinus, roundPlus] = roundInput.closest('.num-stepper').querySelectorAll('.num-btn');
roundMinus.addEventListener('mousedown', () => startStep(adjustRestart, 'restart-round-size', -5));
roundMinus.addEventListener('mouseup', stopStep);
roundMinus.addEventListener('mouseleave', stopStep);
roundPlus.addEventListener('mousedown', () => startStep(adjustRestart, 'restart-round-size', 5));
roundPlus.addEventListener('mouseup', stopStep);
roundPlus.addEventListener('mouseleave', stopStep);
roundInput.addEventListener('input', () => capRestartInput(roundInput));
