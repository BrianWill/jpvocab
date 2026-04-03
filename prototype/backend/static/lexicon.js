import { openEditModal, closeAddResultModal, _addPhase, _pendingGenerates } from './lexicon-add-edit.js';
import { timeAgo, getSortedWords as _getSortedWords, renderReading } from './lexicon-utils.js';
import { playTts, playWordAudio, playSentenceAudio } from './common.js';

let words = [];
export let defaultDrillTarget = 8; // updated from /api/providers at init
export let _providers = null;
export let _imageSources = null;

function updateWordCount() {
  const active = words.filter(w => w.correct < w.target).length;
  document.getElementById('word-count').textContent =
    words.length + ' words (' + active + ' active)';
}

export const typeLabels = {
  'godan-verb':   'Godan verb — Group 1 (五段動詞)',
  'ichidan-verb': 'Ichidan verb — Group 2 (一段動詞)',
  'noun':         'Noun (名詞)',
  'i-adjective':  'い-adjective (い形容詞)',
  'na-adjective': 'な-adjective (な形容詞)',
  'adverb':       'Adverb (副詞)',
  'other':        'Other',
};

function fullDateTime(dateStr) {
  return new Date(dateStr).toLocaleString(undefined, {
    year: 'numeric', month: 'long', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

function renderRow(w, trMain, trEx) {
  const imgCell = w.imagePath
    ? '<td class="cell-img" rowspan="2"><img src="/static/' + w.imagePath + '" alt=""></td>'
    : '<td class="cell-img" rowspan="2"></td>';
  trMain.innerHTML =
    imgCell +
    '<td><div class="cell-word" data-tooltip="Word">' + w.word +
      '<button class="btn-edit" data-tooltip="Edit word">✎</button>' +
      '<button class="btn-delete" data-tooltip="Delete word">✕</button>' +
    '</div></td>' +
    '<td class="cell-reading" data-tooltip="Reading (Pronunciation)">' + renderReading(w.reading, w.word, w.kanjiData) + '</td>' +
    '<td><span class="type-badge" data-tooltip="' + (typeLabels[w.type] || w.type) + '">' + w.type + '</span></td>' +
    '<td class="cell-meaning"><div class="cell-meaning-inner" data-tooltip="Meaning: ' + w.meaning + '">' + w.meaning + '</div></td>' +
    '<td class="cell-correct" data-tooltip="Times answered correctly">' + w.correct + '</td>' +
    '<td class="cell-incorrect" data-tooltip="Times answered incorrectly">' + w.incorrect + '</td>' +
    '<td class="cell-target" data-tooltip="Remaining drills to target">' +
      '<div class="target-stepper">' +
        '<button class="btn-target-adj">−</button>' +
        '<span>' + w.target + '</span>' +
        '<button class="btn-target-adj">+</button>' +
      '</div>' +
    '</td>' +
    '<td></td>';
  trMain._word = w;
  trMain._trEx  = trEx;

  trMain.querySelector('.cell-word').addEventListener('click', () => playWordAudio(w));
  trMain.querySelector('.btn-edit').addEventListener('click', openEditModal);
  trMain.querySelector('.btn-delete').addEventListener('click', openDeleteModal);
  const [adjMinus, adjPlus] = trMain.querySelectorAll('.btn-target-adj');
  adjMinus.addEventListener('mousedown', e => adjustTargetInline(e, -4));
  adjPlus.addEventListener('mousedown', e => adjustTargetInline(e, 4));

  trEx.innerHTML =
    '<td colspan="2" class="cell-date">' +
      '<span class="cell-date-added" data-tooltip="Date added: ' + fullDateTime(w.createdAt) + '">added ' + timeAgo(w.createdAt) + '</span>' +
      '<span class="cell-date-sep"> · </span>' +
      (w.lastDrilled
        ? '<span class="cell-date-drilled" data-tooltip="Last drilled: ' + fullDateTime(w.lastDrilled) + '">drilled ' + timeAgo(w.lastDrilled) + '</span>'
        : '<span class="cell-date-drilled cell-date-never">never drilled</span>') +
    '</td>' +
    '<td colspan="5" class="cell-ex">' +
      (w.exampleJp?.trim() ? '<span class="cell-ex-flag">🇯🇵</span> <span class="cell-ex-jp" data-tooltip="Example sentence">' + w.exampleJp + '</span>' : '') +
      (w.exampleEn?.trim() ? '<span class="cell-ex-sep">🏴󠁧󠁢󠁥󠁮󠁧󠁿</span><span class="cell-ex-en" data-tooltip="Example sentence">' + w.exampleEn + '</span>' : '') +
    '</td>' +
    '<td></td>';

  const elJp = trEx.querySelector('.cell-ex-jp');
  if (elJp) elJp.addEventListener('click', () => playSentenceAudio(w));
  const elEn = trEx.querySelector('.cell-ex-en');
  if (elEn) elEn.addEventListener('click', () => playTts(w.exampleEn, 'en-US'));
}

export function getSortedWords(key, dir) {
  return _getSortedWords(words, key, dir);
}

const wordTable = document.getElementById('word-table');

export function renderTable(sortedWords) {
  wordTable.querySelectorAll('tbody').forEach(b => b.remove());
  sortedWords.forEach(w => {
    const group = document.createElement('tbody');
    group.className = 'word-group';
    const trMain = document.createElement('tr');
    trMain.className = 'row-main';
    const trEx = document.createElement('tr');
    trEx.className = 'row-example';
    renderRow(w, trMain, trEx);
    group.appendChild(trMain);
    group.appendChild(trEx);
    wordTable.appendChild(group);
  });
}

export async function reloadWords() {
  words = await fetch('/api/words').then(r => r.json());
  updateWordCount();
}

export function updateWordImagePath(wordId, imagePath) {
  const word = words.find(w => w.id === wordId);
  if (!word) return;
  word.imagePath = imagePath;
  const activeBtn = document.querySelector('.btn-sort--active');
  renderTable(getSortedWords(activeBtn.dataset.sort, activeBtn.dataset.dir || 'desc'));
}

export function updateWordAudioFlags(wordId, hasWordAudio, hasSentenceAudio) {
  const word = words.find(w => w.id === wordId);
  if (!word) return;
  word.hasWordAudio = hasWordAudio;
  word.hasSentenceAudio = hasSentenceAudio;
}

async function init() {
  const [wordsData, providers] = await Promise.all([
    fetch('/api/words').then(r => r.json()),
    fetch('/api/providers').then(r => r.json()),
  ]);
  words = wordsData;
  if (providers.default_drill_target) defaultDrillTarget = providers.default_drill_target;
  updateWordCount();
  renderTable(getSortedWords('added', 'desc'));
  _providers = providers.ai;
  _imageSources = providers.image_sources;
}

init();

function onBackdropClick(event, closeFn) {
  if (event.target === event.currentTarget) closeFn();
}

// --- Delete modal ---
let _deleteTrMain = null;

function openDeleteModal(event) {
  event.stopPropagation();
  _deleteTrMain = event.target.closest('tr');
  const w = _deleteTrMain._word;
  document.getElementById('delete-modal-label').textContent = w.word;
  document.getElementById('delete-error').classList.add('hidden');
  document.getElementById('btn-delete-confirm').disabled = false;
  document.getElementById('btn-delete-confirm').textContent = 'Delete';
  document.getElementById('delete-modal-backdrop').classList.remove('hidden');
}

function closeDeleteModal() {
  document.getElementById('delete-modal-backdrop').classList.add('hidden');
}


async function confirmDelete() {
  const w   = _deleteTrMain._word;
  const btn = document.getElementById('btn-delete-confirm');
  const errEl = document.getElementById('delete-error');
  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span>';
  errEl.classList.add('hidden');

  try {
    const res = await fetch('/api/words/' + w.id, { method: 'DELETE' });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    words.splice(words.indexOf(w), 1);
    _deleteTrMain.closest('tbody').remove();
    updateWordCount();
    closeDeleteModal();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('hidden');
    btn.disabled = false;
    btn.textContent = 'Delete';
  }
}

function adjustTargetInline(event, delta) {
  event.stopPropagation();
  const trMain = event.target.closest('tr');
  const w = trMain._word;
  const newTarget = Math.max(w.correct, w.target + delta);
  if (newTarget === w.target) return;
  w.target = newTarget;
  renderRow(w, trMain, trMain._trEx);
  updateWordCount();
  fetch('/api/words/' + w.id + '/target', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target: newTarget }),
  });
}

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    closeAddModal();
    closeDeleteModal();
    if (_addPhase !== 'loading' && _pendingGenerates === 0) closeAddResultModal();
  }
});

// Sort button active state, direction toggle, and sorting
const sortBtns = document.querySelectorAll('.btn-sort');
sortBtns.forEach(btn => {
  btn.addEventListener('click', e => {
    e.stopPropagation();
    const wasActive = btn.classList.contains('btn-sort--active');
    sortBtns.forEach(b => {
      b.classList.remove('btn-sort--active');
      if (b !== btn && 'dir' in b.dataset && b.dataset.dir === 'asc') {
        b.dataset.dir = 'desc';
        b.textContent = b.textContent.replace('↑', '↓');
      }
    });
    btn.classList.add('btn-sort--active');
    if (wasActive && 'dir' in btn.dataset) {
      const desc = btn.dataset.dir === 'desc';
      btn.dataset.dir = desc ? 'asc' : 'desc';
      btn.textContent = btn.textContent.replace(desc ? '↓' : '↑', desc ? '↑' : '↓');
    }
    renderTable(getSortedWords(btn.dataset.sort, btn.dataset.dir));
  });
});

// --- Tooltip ---
const lexTooltip = document.createElement('div');
lexTooltip.className = 'lex-tooltip';
document.body.appendChild(lexTooltip);

let _activeTooltipEl = null;

document.addEventListener('mouseover', e => {
  const el = e.target.closest('[data-tooltip]');
  _activeTooltipEl = el ?? null;
  if (!el) { lexTooltip.classList.remove('visible'); return; }
  lexTooltip.textContent = el.dataset.tooltip;
  lexTooltip.classList.add('visible');
});

export function refreshTooltip(el) {
  if (_activeTooltipEl === el) lexTooltip.textContent = el.dataset.tooltip;
}
document.addEventListener('mousemove', e => {
  if (!lexTooltip.classList.contains('visible')) return;
  const x = e.clientX + 14;
  lexTooltip.style.left = (x + lexTooltip.offsetWidth > window.innerWidth)
    ? (e.clientX - lexTooltip.offsetWidth) + 'px'
    : x + 'px';
  lexTooltip.style.top = (e.clientY + 18) + 'px';
});

// --- Static element event listeners ---
document.querySelector('.btn-header').addEventListener('click', openAddModal);

const addModalBackdrop = document.getElementById('add-modal-backdrop');
addModalBackdrop.addEventListener('click', e => onBackdropClick(e, closeAddModal));
addModalBackdrop.querySelector('.modal-close').addEventListener('click', closeAddModal);
addModalBackdrop.querySelector('.btn-cancel').addEventListener('click', closeAddModal);

const deleteModalBackdrop = document.getElementById('delete-modal-backdrop');
deleteModalBackdrop.addEventListener('click', e => onBackdropClick(e, closeDeleteModal));
deleteModalBackdrop.querySelector('.modal-close').addEventListener('click', closeDeleteModal);
deleteModalBackdrop.querySelector('.btn-cancel').addEventListener('click', closeDeleteModal);
document.getElementById('btn-delete-confirm').addEventListener('click', confirmDelete);

// --- Word list sidebar ---
// Per-list cache: slug → { remaining, total, inLexicon, initialAvailable }
const _wordListCache = new Map();

function setAddModalStatus(msg) {
  document.getElementById('add-modal-status').textContent = msg;
}

function listItemTooltip(slug, total, inLexicon) {
  const c = _wordListCache.get(slug);
  const lexCount = c ? c.inLexicon : inLexicon;
  const available = total - lexCount;
  const added = c ? (c.initialAvailable - c.remaining.length) : 0;
  const remaining = c ? c.remaining.length : available;
  let s = total + ' total · ' + lexCount + ' in lexicon · ' + added + ' added · ' + remaining + ' remaining';
  return s;
}

async function initWordListSidebar() {
  const sidebar = document.getElementById('add-modal-sidebar');
  try {
    const res = await fetch('/api/wordlists');
    if (!res.ok) return;
    const lists = await res.json();

    const title = document.createElement('div');
    title.className = 'add-modal-sidebar-title';
    title.textContent = 'Word lists';
    sidebar.appendChild(title);

    for (const list of lists) {
      const item = document.createElement('div');
      item.className = 'add-modal-sidebar-item';
      item.dataset.tooltip = listItemTooltip(list.slug, list.total, list.in_lexicon);

      const btn = document.createElement('button');
      btn.className = 'add-modal-sidebar-add-btn';
      btn.textContent = '+';
      btn.addEventListener('click', () => addWordFromList(list.slug, list.name, list.total, list.in_lexicon, btn, item));

      const label = document.createElement('span');
      label.className = 'add-modal-sidebar-name';
      label.textContent = list.name;

      item.appendChild(btn);
      item.appendChild(label);
      sidebar.appendChild(item);
    }
  } catch (_) { /* fail silently — sidebar stays empty */ }
}

async function addWordFromList(slug, name, total, inLexicon, btn, item) {
  btn.disabled = true;
  try {
    // Fetch and cache the available words on first click.
    if (!_wordListCache.has(slug)) {
      const res = await fetch('/api/wordlists/' + encodeURIComponent(slug) + '/words');
      if (!res.ok) { setAddModalStatus('Failed to load the ' + name + ' list.'); return; }
      const data = await res.json();
      const words = data.words ?? [];
      _wordListCache.set(slug, {
        remaining: words,
        total: data.total ?? total,
        inLexicon: data.in_lexicon ?? inLexicon,
        initialAvailable: words.length,
      });
    }

    const c = _wordListCache.get(slug);
    if (c.remaining.length === 0) {
      setAddModalStatus('No more words to add from the ' + name + ' list.');
      return;
    }

    const idx = Math.floor(Math.random() * c.remaining.length);
    const word = c.remaining.splice(idx, 1)[0];

    const displayText = word + ' (from the ' + name + ' word list)';
    const textarea = document.getElementById('add-words-input');
    textarea.value = textarea.value ? displayText + '\n' + textarea.value : displayText;
    textarea.scrollTop = 0;
    setAddModalStatus('One random word added from the ' + name + ' list.');
    item.dataset.tooltip = listItemTooltip(slug, c.total, c.inLexicon);
    refreshTooltip(item);
  } catch (_) {
    setAddModalStatus('Failed to load the ' + name + ' list.');
  } finally {
    btn.disabled = false;
  }
}

initWordListSidebar();

// --- Add words modal ---
function openAddModal() {
  document.getElementById('add-words-input').value = '';
  document.getElementById('add-modal-status').textContent = '';
  document.getElementById('add-modal-backdrop').classList.remove('hidden');
  document.getElementById('add-words-input').focus();
}

export function closeAddModal() {
  document.getElementById('add-modal-backdrop').classList.add('hidden');
}
