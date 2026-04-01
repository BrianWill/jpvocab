import { populateWordTooltip, positionAnchoredWordTooltip, renderWordTooltipKanji } from './common.js';
import { renderReading } from './lexicon-utils.js';

const FILTER_KEYS = ['katakana', 'verbs', 'nouns', 'other'];
const activeFilters = new Set(FILTER_KEYS);
const DEFAULT_ROUND_SIZE = 10;
const STEP_INTERVAL = 230;

let kanjiMap = {};
let words = [];
let sessionId = null;
let poolSize = 0;
let maxPoolSize = 0;
let settingsMaxWords = null;
let roundSize = DEFAULT_ROUND_SIZE;
let pool = [];
let round = 1;
let redo = [];
let doneCount = 0;
let drillStartedAt = Date.now();
let remaining = [];
let currentWord = null;
let sidebarItems = [];
let lastAnswered = null;
let isSubmittingAnswer = false;
let stepTimer = null;

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
  return words.filter(w => FILTER_KEYS.some(f => activeFilters.has(f) && matchesFilter(w, f)));
}

function updateFilterHint() {
  const hint = document.getElementById('filter-hint');
  const btn = document.getElementById('restart-start-btn');
  if (activeFilters.size === 0) {
    hint.textContent = 'Select at least one word type';
    hint.classList.add('error');
    btn.disabled = true;
    return;
  }

  const count = getFilteredWords().length;
  hint.textContent = activeFilters.size === FILTER_KEYS.length
    ? 'All ' + count + ' words'
    : count + ' of ' + words.length + ' words';
  hint.classList.remove('error');
  btn.disabled = false;
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

function buildRound() {
  const slots = Math.max(0, roundSize - redo.length);
  const picked = pool.splice(0, slots);
  return [...redo, ...picked];
}

function updateStats() {
  document.getElementById('stat-togo').textContent = (poolSize - doneCount) + ' to go of ' + poolSize;
  document.getElementById('sidebar-title').textContent = 'Round ' + round;
  document.getElementById('header-began').textContent = 'began ' + timeAgo(drillStartedAt);

  const pct = poolSize > 0 ? (doneCount / poolSize) * 100 : 0;
  document.querySelector('.progress-bar').style.width = pct + '%';
}

function showWord() {
  if (!currentWord) return;
  document.getElementById('prompt-word-jp').textContent = currentWord.word;
  document.getElementById('prompt-example-jp').textContent = currentWord.exampleJp;

  const list = document.getElementById('sidebar-list');
  list.querySelectorAll('.sidebar-item.current').forEach(el => el.classList.remove('current'));
  const item = list.querySelector('[data-id="' + currentWord.word + '"]');
  if (item) item.classList.add('current');
}

function renderSidebar() {
  const list = document.getElementById('sidebar-list');
  list.innerHTML = '';
  sidebarItems.forEach(itemData => {
    const li = document.createElement('li');
    li.className = 'sidebar-item ' + itemData.status;
    li.textContent = itemData.word.word;
    li.dataset.word = JSON.stringify(itemData.word);
    li.dataset.id = itemData.word.word;
    li.addEventListener('animationend', () => li.classList.remove('flash-known', 'flash-missed'), { once: true });
    list.appendChild(li);
  });
}

function renderLastAnswered() {
  const card = document.getElementById('last-word-card');
  if (!lastAnswered) {
    card.style.display = 'none';
    return;
  }

  const answered = lastAnswered.word;
  card.style.display = '';
  const lastWordEl = document.getElementById('last-word-jp');
  lastWordEl.textContent = answered.word;
  lastWordEl.className = 'tooltip-word ' + (lastAnswered.knew ? 'knew' : 'missed');
  document.getElementById('last-reading').innerHTML = renderReading(answered.reading, answered.word, answered.kanjiData);
  document.getElementById('last-pos').textContent = answered.type;
  document.getElementById('last-meaning').textContent = answered.meaning;
  document.getElementById('last-example-jp').textContent = answered.exampleJp;
  document.getElementById('last-example-en').textContent = answered.exampleEn;
  renderWordTooltipKanji(document.getElementById('last-kanji-info'), answered, kanjiMap);
  const imgEl = document.getElementById('last-word-image');
  if (answered.imagePath) {
    imgEl.src = '/static/' + answered.imagePath;
    imgEl.style.display = '';
  } else {
    imgEl.style.display = 'none';
  }
}

function renderCompleteState() {
  document.getElementById('prompt-word-jp').textContent = 'Done!';
  document.getElementById('prompt-example-jp').textContent = 'All words cleared.';
  document.getElementById('action-prompt').style.display = 'none';
}

function renderInProgressState() {
  document.getElementById('action-prompt').style.display = '';
}

function addToSidebar(word, knew) {
  const existing = sidebarItems.find(item => item.word.word === word.word);
  const status = knew ? 'known flash-known' : 'missed flash-missed';
  if (existing) {
    existing.word = word;
    existing.status = status;
    return;
  }
  sidebarItems.push({ word, status });
}

function startNextRound() {
  round++;
  const redoSet = new Set(redo.map(w => w.word));
  remaining = buildRound();
  redo = [];
  currentWord = remaining[0] || null;

  const redoWords = remaining.filter(w => redoSet.has(w.word));
  const newWords = remaining.filter(w => !redoSet.has(w.word));
  sidebarItems = [...redoWords, ...newWords].map(word => ({
    word,
    status: redoSet.has(word.word) ? 'unseen-redo' : 'unseen',
  }));
}

function getSessionState() {
  return {
    poolSize,
    maxPoolSize,
    settingsMaxWords,
    roundSize,
    round,
    doneCount,
    activeFilters: [...activeFilters],
    pool,
    redo,
    remaining,
    sidebarItems: sidebarItems.map(item => ({
      word: item.word,
      status: item.status.replace(/\sflash-(known|missed)\b/g, ''),
    })),
    lastAnswered,
    completed: !currentWord && remaining.length === 0 && redo.length === 0 && pool.length === 0,
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

async function postAnswer(wordId, correct, state) {
  if (!sessionId) return;
  const resp = await fetch('/api/drill/sessions/' + sessionId + '/answers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ wordId, correct, state }),
  });
  if (!resp.ok) throw new Error('failed to save drill answer');
}

async function reveal(knew) {
  if (!currentWord || isSubmittingAnswer) return;
  isSubmittingAnswer = true;

  const answered = currentWord;
  remaining.shift();
  if (knew) {
    doneCount++;
  } else {
    redo.push(answered);
  }

  addToSidebar(answered, knew);
  lastAnswered = { word: answered, knew };

  if (remaining.length === 0) {
    if (redo.length > 0 || pool.length > 0) {
      startNextRound();
      renderInProgressState();
    } else {
      currentWord = null;
      renderCompleteState();
    }
  } else {
    currentWord = remaining[0];
    renderInProgressState();
  }

  renderSidebar();
  renderLastAnswered();
  updateStats();
  showWord();

  try {
    await postAnswer(answered.id, knew, getSessionState());
  } finally {
    isSubmittingAnswer = false;
  }
}

function positionSidebarTooltip(item, tip) {
  const sidebar = document.querySelector('.sidebar');
  const itemRect = item.getBoundingClientRect();
  const sidebarRect = sidebar.getBoundingClientRect();
  positionAnchoredWordTooltip(tip, {
    anchorRect: itemRect,
    left: sidebarRect.right - 14,
  });
}

function restoreSession(session) {
  sessionId = session.id;
  const startedAt = Date.parse(session.startedAt);
  drillStartedAt = Number.isNaN(startedAt) ? Date.now() : startedAt;

  const state = session.state || {};
  poolSize = state.poolSize || 0;
  maxPoolSize = state.maxPoolSize || 0;
  settingsMaxWords = state.settingsMaxWords > 0 ? state.settingsMaxWords : settingsMaxWords;
  roundSize = state.roundSize || DEFAULT_ROUND_SIZE;
  round = state.round || 1;
  doneCount = state.doneCount || 0;
  pool = Array.isArray(state.pool) ? state.pool : [];
  redo = Array.isArray(state.redo) ? state.redo : [];
  remaining = Array.isArray(state.remaining) ? state.remaining : [];
  currentWord = remaining[0] || null;
  sidebarItems = Array.isArray(state.sidebarItems) ? state.sidebarItems : [];
  lastAnswered = state.lastAnswered || null;

  if (Array.isArray(state.activeFilters) && state.activeFilters.length > 0) {
    activeFilters.clear();
    state.activeFilters.forEach(f => activeFilters.add(f));
  }
  document.querySelectorAll('#restart-modal-backdrop .filter-chip[data-filter]').forEach(btn => {
    btn.classList.toggle('active', activeFilters.has(btn.dataset.filter));
  });

  if (state.completed) renderCompleteState();
  else renderInProgressState();

  renderSidebar();
  renderLastAnswered();
  updateStats();
  showWord();
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

  kanjiMap = {};
  kanjiList.forEach(k => { kanjiMap[k.id] = k; });

  words = allWords.filter(w => w.correct < w.target);

  settingsMaxWords = settings.maxWords;
  if (settings.roundSize > 0) roundSize = settings.roundSize;
  if (Array.isArray(settings.wordTypes) && settings.wordTypes.length > 0) {
    activeFilters.clear();
    settings.wordTypes.forEach(f => activeFilters.add(f));
  }
  document.querySelectorAll('#restart-modal-backdrop .filter-chip[data-filter]').forEach(btn => {
    btn.classList.toggle('active', activeFilters.has(btn.dataset.filter));
  });

  if (currentSession) {
    const state = currentSession.state || {};
    const hasRestorableState = (state.poolSize || 0) > 0 || (Array.isArray(state.remaining) && state.remaining.length > 0);
    if (hasRestorableState) {
      restoreSession(currentSession);
      return;
    }
  }

  const filtered = getFilteredWords();
  const source = filtered.length > 0 ? filtered : words;
  maxPoolSize = Math.min(settings.maxWords, source.length);
  poolSize = maxPoolSize;
  pool = shuffle([...source]).slice(0, poolSize);
  remaining = buildRound();
  currentWord = remaining[0] || null;
  sidebarItems = remaining.map(word => ({ word, status: 'unseen' }));
  lastAnswered = null;

  sessionId = await createSession(getSessionState());
  renderInProgressState();
  renderSidebar();
  renderLastAnswered();
  showWord();
  updateStats();
}

function startStep(fn, ...args) {
  fn(...args);
  stepTimer = setInterval(() => fn(...args), STEP_INTERVAL);
}

function stopStep() {
  clearInterval(stepTimer);
  stepTimer = null;
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
  document.getElementById('restart-total-words').value = settingsMaxWords;
  document.getElementById('restart-round-size').value = roundSize;
  updateFilterHint();
  document.getElementById('restart-modal-backdrop').classList.remove('hidden');
}

function closeRestartModal() {
  document.getElementById('restart-modal-backdrop').classList.add('hidden');
}

function handleRestartBackdropClick(e) {
  if (e.target === document.getElementById('restart-modal-backdrop')) closeRestartModal();
}

function restartDrill(totalWords, newRoundSize, sourceWords) {
  poolSize = totalWords;
  roundSize = newRoundSize;
  pool = shuffle([...(sourceWords || words)]).slice(0, poolSize);
  round = 1;
  redo = [];
  doneCount = 0;
  drillStartedAt = Date.now();
  remaining = buildRound();
  currentWord = remaining[0] || null;
  sidebarItems = remaining.map(word => ({ word, status: 'unseen' }));
  lastAnswered = null;

  renderInProgressState();
  renderSidebar();
  renderLastAnswered();
  updateStats();
  showWord();
}

async function confirmRestart() {
  const filtered = getFilteredWords();
  maxPoolSize = Math.max(1, parseInt(document.getElementById('restart-total-words').value, 10) || filtered.length);
  const total = Math.min(maxPoolSize, filtered.length);
  const rSize = Math.max(1, Math.min(total, parseInt(document.getElementById('restart-round-size').value, 10) || roundSize));
  closeRestartModal();
  restartDrill(total, rSize, filtered);
  sessionId = await createSession(getSessionState());
}

const tip = document.getElementById('tooltip');
document.getElementById('sidebar-list').addEventListener('mouseover', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item || !item.dataset.word) return;
  const data = JSON.parse(item.dataset.word);
  populateWordTooltip(tip, data, kanjiMap, renderReading);
  positionSidebarTooltip(item, tip);
});
document.getElementById('sidebar-list').addEventListener('mouseout', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item) return;
  if (!item.contains(e.relatedTarget)) tip.classList.remove('visible');
});

init();

setInterval(() => {
  document.getElementById('header-began').textContent = 'began ' + timeAgo(drillStartedAt);
}, 30_000);

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    closeRestartModal();
    return;
  }
  const prompt = document.getElementById('action-prompt');
  if (prompt.style.display === 'none') return;
  if (e.key === 'd' || e.key === 'D') reveal(true);
  if (e.key === 'a' || e.key === 'A') reveal(false);
});

document.querySelectorAll('#restart-modal-backdrop .filter-chip').forEach(btn => {
  btn.addEventListener('click', () => {
    const f = btn.dataset.filter;
    if (activeFilters.has(f)) activeFilters.delete(f);
    else activeFilters.add(f);
    btn.classList.toggle('active');
    updateFilterHint();
  });
});

document.querySelector('.btn-header').addEventListener('click', openRestartModal);
document.querySelector('.btn-no').addEventListener('click', () => reveal(false));
document.querySelector('.btn-yes').addEventListener('click', () => reveal(true));

const restartBackdrop = document.getElementById('restart-modal-backdrop');
restartBackdrop.addEventListener('click', handleRestartBackdropClick);
restartBackdrop.querySelector('.modal-close').addEventListener('click', closeRestartModal);
restartBackdrop.querySelector('.btn-cancel').addEventListener('click', closeRestartModal);
document.getElementById('restart-start-btn').addEventListener('click', confirmRestart);

const totalInput = document.getElementById('restart-total-words');
const [totalMinus, totalPlus] = totalInput.closest('.num-stepper').querySelectorAll('.num-btn');
totalMinus.addEventListener('mousedown', () => startStep(adjustRestart, 'restart-total-words', -5));
totalMinus.addEventListener('mouseup', stopStep);
totalMinus.addEventListener('mouseleave', stopStep);
totalPlus.addEventListener('mousedown', () => startStep(adjustRestart, 'restart-total-words', 5));
totalPlus.addEventListener('mouseup', stopStep);
totalPlus.addEventListener('mouseleave', stopStep);
totalInput.addEventListener('input', () => capRestartInput(totalInput));

const roundInput = document.getElementById('restart-round-size');
const [roundMinus, roundPlus] = roundInput.closest('.num-stepper').querySelectorAll('.num-btn');
roundMinus.addEventListener('mousedown', () => startStep(adjustRestart, 'restart-round-size', -5));
roundMinus.addEventListener('mouseup', stopStep);
roundMinus.addEventListener('mouseleave', stopStep);
roundPlus.addEventListener('mousedown', () => startStep(adjustRestart, 'restart-round-size', 5));
roundPlus.addEventListener('mouseup', stopStep);
roundPlus.addEventListener('mouseleave', stopStep);
roundInput.addEventListener('input', () => capRestartInput(roundInput));
