import { renderReading } from './lexicon-utils.js';

const placeholder = '<span class="detail-placeholder">- - -</span>';

// ── Word-type filters ──────────────────────────────────────────────────────
const FILTER_KEYS = ['katakana', 'verbs', 'nouns', 'other'];
const activeFilters = new Set(FILTER_KEYS);

function matchesFilter(w, f) {
  const isKatakana = /^[\u30A0-\u30FF]+$/.test(w.word);
  const isVerb = w.type === 'ichidan-verb' || w.type === 'godan-verb';
  const isNoun = w.type === 'noun';
  if (f === 'katakana') return isKatakana;
  if (f === 'verbs')    return isVerb;
  if (f === 'nouns')    return isNoun;
  if (f === 'other')    return !isKatakana && !isVerb && !isNoun;
  return false;
}

function getFilteredWords() {
  return words.filter(w => FILTER_KEYS.some(f => activeFilters.has(f) && matchesFilter(w, f)));
}

function updateFilterHint() {
  const hint = document.getElementById('filter-hint');
  const btn  = document.getElementById('restart-start-btn');
  if (activeFilters.size === 0) {
    hint.textContent = 'Select at least one word type';
    hint.classList.add('error');
    btn.disabled = true;
  } else {
    const count = getFilteredWords().length;
    hint.textContent = activeFilters.size === FILTER_KEYS.length
      ? 'All ' + count + ' words'
      : count + ' of ' + words.length + ' words';
    hint.classList.remove('error');
    btn.disabled = false;
  }
}

const DEFAULT_ROUND_SIZE = 10;

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

// Kanji reference data keyed by ID (populated by init)
let kanjiMap = {};

// Session state (populated by init / restartDrill)
let words = [];
let sessionId = null;
let poolSize = 0;
let maxPoolSize = 0;
let roundSize = DEFAULT_ROUND_SIZE;
let pool = [];
let round = 1;
let redo = [];
let doneCount = 0;
let drillStartedAt = Date.now();
let remaining = [];
let currentWord = null;

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

// ── API helpers ────────────────────────────────────────────────────────────

async function createSession() {
  const resp = await fetch('/api/drill/sessions', { method: 'POST' });
  const data = await resp.json();
  return data.id;
}

function postAnswer(wordId, correct) {
  if (!sessionId) return;
  fetch('/api/drill/sessions/' + sessionId + '/answers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ wordId, correct }),
  });
}

// ── Init ───────────────────────────────────────────────────────────────────

async function init() {
  const [wordsResp, kanjiResp] = await Promise.all([
    fetch('/api/words'),
    fetch('/api/kanji'),
  ]);
  const allWords = await wordsResp.json();
  const kanjiList = await kanjiResp.json();
  kanjiMap = {};
  kanjiList.forEach(k => { kanjiMap[k.id] = k; });

  // Only drill active words (drill_count < target)
  words = allWords.filter(w => w.correct < w.target);

  sessionId = await createSession();

  poolSize = words.length;
  maxPoolSize = words.length;
  pool = shuffle([...words]);
  remaining = buildRound();
  currentWord = remaining[0];

  initSidebar();
  showWord();
  updateStats();
}

// ── Drill logic ────────────────────────────────────────────────────────────

function reveal(knew) {
  if (!currentWord) return;
  const answered = currentWord;

  postAnswer(answered.id, knew);

  // Update drill state
  remaining.shift();
  if (knew) {
    doneCount++;
  } else {
    redo.push(answered);
  }

  addToSidebar(answered, knew);
  updateStats();

  // Show answered word in last-word card
  document.getElementById('last-word-card').style.display = '';
  const lastWordEl = document.getElementById('last-word-jp');
  lastWordEl.textContent = answered.word;
  lastWordEl.className = 'tooltip-word ' + (knew ? 'knew' : 'missed');
  document.getElementById('last-reading').innerHTML = renderReading(answered.reading, answered.word, answered.kanjiData);
  document.getElementById('last-pos').textContent = answered.type;
  document.getElementById('last-meaning').textContent = answered.meaning;
  document.getElementById('last-example-jp').textContent = answered.exampleJp;
  document.getElementById('last-example-en').textContent = answered.exampleEn;
  renderKanjiInfo(document.getElementById('last-kanji-info'), answered);

  // Advance
  if (remaining.length === 0) {
    if (redo.length > 0 || pool.length > 0) {
      startNextRound();
      return;
    } else {
      document.getElementById('prompt-word-jp').textContent = 'Done!';
      document.getElementById('prompt-example-jp').textContent = 'All words cleared.';
      document.getElementById('action-prompt').style.display = 'none';
      return;
    }
  }

  currentWord = remaining[0];
  showWord();
}

function initSidebar() {
  const list = document.getElementById('sidebar-list');
  remaining.forEach(word => {
    const li = document.createElement('li');
    li.className = 'sidebar-item unseen';
    li.textContent = word.word;
    li.dataset.word = JSON.stringify(word);
    li.dataset.id = word.word;
    list.appendChild(li);
  });
}

function addToSidebar(word, knew) {
  const list = document.getElementById('sidebar-list');
  const existing = list.querySelector('[data-id="' + word.word + '"]');

  if (existing) {
    // Update in place — sorting only happens at round boundaries
    existing.className = 'sidebar-item ' + (knew ? 'known flash-known' : 'missed flash-missed');
    existing.dataset.word = JSON.stringify(word);
    existing.addEventListener('animationend', () => existing.classList.remove('flash-known', 'flash-missed'), { once: true });
    return;
  }

  // Fallback: append new entry (not expected mid-round)
  const li = document.createElement('li');
  li.className = 'sidebar-item ' + (knew ? 'known flash-known' : 'missed flash-missed');
  li.textContent = word.word;
  li.dataset.word = JSON.stringify(word);
  li.dataset.id = word.word;
  li.addEventListener('animationend', () => li.classList.remove('flash-known', 'flash-missed'));
  list.appendChild(li);
}

function renderKanjiInfo(container, word) {
  container.innerHTML = '';
  if (!word.kanjiData || word.kanjiData.length === 0) return;
  word.kanjiData.forEach(entry => {
    const k = kanjiMap[entry.id];
    if (!k) return;
    const isOn = /[\u30A0-\u30FF]/.test(entry.reading);
    const div = document.createElement('div');
    div.className = 'kanji-entry';
    div.innerHTML =
      '<div class="kanji-char">' + k.character + '</div>' +
      '<div class="kanji-detail">' +
        '<div class="kanji-readings"><span class="kanji-' + (isOn ? 'on' : 'kun') + '">' + entry.reading + '</span></div>' +
        '<div class="kanji-meanings">' + k.meanings.join(', ') + '</div>' +
      '</div>';
    container.appendChild(div);
  });
}

// Tooltip hover logic
const tip = document.getElementById('tooltip');
document.getElementById('sidebar-list').addEventListener('mouseover', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item || !item.dataset.word) return;
  const data = JSON.parse(item.dataset.word);
  document.getElementById('tip-word').textContent = data.word;
  document.getElementById('tip-reading').innerHTML = renderReading(data.reading, data.word, data.kanjiData);
  document.getElementById('tip-pos').textContent = data.type;
  document.getElementById('tip-meaning').textContent = data.meaning;
  document.getElementById('tip-example').textContent = data.exampleJp || '';
  document.getElementById('tip-example-en').textContent = data.exampleEn || '';
  renderKanjiInfo(document.getElementById('tip-kanji-info'), data);

  const rect = item.getBoundingClientRect();
  const sidebar = document.querySelector('.sidebar');
  tip.style.left = sidebar.getBoundingClientRect().right + 'px';
  tip.style.top = rect.top + 'px';
  tip.style.transform = '';
  tip.classList.add('visible');
});
document.getElementById('sidebar-list').addEventListener('mouseout', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item) return;
  if (!item.contains(e.relatedTarget)) {
    tip.classList.remove('visible');
  }
});


function startNextRound() {
  round++;
  const redoSet = new Set(redo.map(w => w.word));
  remaining = buildRound(); // uses current redo + new picks from pool
  redo = [];
  currentWord = remaining[0];
  updateStats();

  const list = document.getElementById('sidebar-list');
  list.innerHTML = '';

  // Redo words first (red + blurred), then new words (gray + blurred)
  const redoWords = remaining.filter(w => redoSet.has(w.word));
  const newWords = remaining.filter(w => !redoSet.has(w.word));
  [...redoWords, ...newWords].forEach(word => {
    const isRedo = redoSet.has(word.word);
    const li = document.createElement('li');
    li.className = 'sidebar-item ' + (isRedo ? 'unseen-redo' : 'unseen');
    li.textContent = word.word;
    li.dataset.word = JSON.stringify(word);
    li.dataset.id = word.word;
    list.appendChild(li);
  });

  showWord();
}

const STEP_INTERVAL = 230;
let _stepTimer = null;
function startStep(fn, ...args) { fn(...args); _stepTimer = setInterval(() => fn(...args), STEP_INTERVAL); }
function stopStep() { clearInterval(_stepTimer); _stepTimer = null; }

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
  document.getElementById('restart-total-words').value = maxPoolSize;
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
async function confirmRestart() {
  const filtered = getFilteredWords();
  maxPoolSize = Math.max(1, parseInt(document.getElementById('restart-total-words').value, 10) || filtered.length);
  const total = Math.min(maxPoolSize, filtered.length);
  const rSize = Math.max(1, Math.min(total, parseInt(document.getElementById('restart-round-size').value, 10) || roundSize));
  closeRestartModal();
  sessionId = await createSession();
  restartDrill(total, rSize, filtered);
}

function restartDrill(totalWords, newRoundSize, sourceWords) {
  sourceWords = sourceWords || words;
  poolSize = totalWords;
  roundSize = newRoundSize;
  pool = shuffle([...sourceWords]).slice(0, poolSize);
  round = 1;
  redo = [];
  doneCount = 0;
  drillStartedAt = Date.now();
  remaining = buildRound();
  currentWord = remaining[0];

  document.getElementById('sidebar-list').innerHTML = '';
  document.getElementById('action-prompt').style.display = '';
  document.getElementById('last-word-card').style.display = 'none';
  initSidebar();
  updateStats();
  showWord();
}

// Initialize
init();

setInterval(() => {
  document.getElementById('header-began').textContent = 'began ' + timeAgo(drillStartedAt);
}, 30_000);

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') { closeRestartModal(); return; }
  const prompt = document.getElementById('action-prompt');
  if (prompt.style.display === 'none') return;
  if (e.key === 'd' || e.key === 'D') reveal(true);
  if (e.key === 'a' || e.key === 'A') reveal(false);
});

document.querySelectorAll('.filter-chip').forEach(btn => {
  btn.addEventListener('click', () => {
    const f = btn.dataset.filter;
    if (activeFilters.has(f)) activeFilters.delete(f);
    else activeFilters.add(f);
    btn.classList.toggle('active');
    updateFilterHint();
  });
});

// --- Static element event listeners ---
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
