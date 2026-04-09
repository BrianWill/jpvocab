import { openEditModal, closeAddResultModal, state as addEditState } from './lexicon-add-edit.js';
import { timeAgo, getSortedWords as _getSortedWords, renderReading } from './lexicon-utils.js';
import { playTts, playWordAudio, playSentenceAudio, checkVoicevoxAvailable, checkFfmpegAvailable, refreshTooltip } from './common.js';

const LEXICON_AUDIO_OPTIONS = { preferSynthesis: true, fallbackToBrowserTts: true };

const els = {
  addModalBackdrop: document.getElementById('add-modal-backdrop'),
  addModalSidebar: document.getElementById('add-modal-sidebar'),
  addModalStatus: document.getElementById('add-modal-status'),
  addWordsInput: document.getElementById('add-words-input'),
  deleteConfirmBtn: document.getElementById('btn-delete-confirm'),
  deleteError: document.getElementById('delete-error'),
  deleteModalBackdrop: document.getElementById('delete-modal-backdrop'),
  deleteModalLabel: document.getElementById('delete-modal-label'),
  headerAddBtn: document.querySelector('.btn-header'),
  sortBtns: Array.from(document.querySelectorAll('.btn-sort')),
  wordCount: document.getElementById('word-count'),
  wordTable: document.getElementById('word-table'),
};
els.addModalCloseBtn = els.addModalBackdrop.querySelector('.modal-close');
els.addModalCancelBtn = els.addModalBackdrop.querySelector('.btn-cancel');
els.deleteModalCloseBtn = els.deleteModalBackdrop.querySelector('.modal-close');
els.deleteModalCancelBtn = els.deleteModalBackdrop.querySelector('.btn-cancel');

export const state = {
  defaultDrillTarget: 8, // updated from /api/providers at init
  deleteTrMain: null,
  ffmpegAvailable: false,
  imageSources: null,
  providers: null,
  voicevoxAvailable: false,
  wordListCache: new Map(),
  words: [],
};

function updateWordCount() {
  const active = state.words.filter(w => w.correct < w.target).length;
  els.wordCount.textContent = state.words.length + ' words (' + active + ' active)';
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
    '<td class="cell-reading" data-tooltip="Reading (Pronunciation)">' + renderReading(w.reading, w.word, w.kanjiData, w.pitchAccent) + '</td>' +
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

  trMain.querySelector('.cell-word').addEventListener('click', () => playWordAudio(w, 1, LEXICON_AUDIO_OPTIONS));
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
  if (elJp) elJp.addEventListener('click', () => playSentenceAudio(w, 1, LEXICON_AUDIO_OPTIONS));
  const elEn = trEx.querySelector('.cell-ex-en');
  if (elEn) elEn.addEventListener('click', () => playTts(w.exampleEn, 'en-US'));
}

export function getSortedWords(key, dir) {
  return _getSortedWords(state.words, key, dir);
}

export function renderTable(sortedWords) {
  els.wordTable.querySelectorAll('tbody').forEach(b => b.remove());
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
    els.wordTable.appendChild(group);
  });
}

export async function reloadWords() {
  state.words = await fetch('/api/words').then(r => r.json());
  updateWordCount();
}

export function updateWordImagePath(wordId, imagePath) {
  const word = state.words.find(w => w.id === wordId);
  if (!word) return;
  word.imagePath = imagePath;
  const activeBtn = els.sortBtns.find(b => b.classList.contains('btn-sort--active'));
  renderTable(getSortedWords(activeBtn.dataset.sort, activeBtn.dataset.dir || 'desc'));
}

async function init() {
  const [wordsData, providers] = await Promise.all([
    fetch('/api/words').then(r => r.json()),
    fetch('/api/providers').then(r => r.json()),
  ]);
  state.words = wordsData;
  if (providers.default_drill_target) state.defaultDrillTarget = providers.default_drill_target;
  updateWordCount();
  renderTable(getSortedWords('added', 'desc'));
  state.providers = providers.ai;
  state.imageSources = providers.image_sources;
  [state.voicevoxAvailable, state.ffmpegAvailable] = await Promise.all([
    checkVoicevoxAvailable(),
    checkFfmpegAvailable(),
  ]);
}

init();

function onBackdropClick(event, closeFn) {
  if (event.target === event.currentTarget) closeFn();
}

// --- Delete modal ---

function openDeleteModal(event) {
  event.stopPropagation();
  state.deleteTrMain = event.target.closest('tr');
  const w = state.deleteTrMain._word;
  els.deleteModalLabel.textContent = w.word;
  els.deleteError.classList.add('hidden');
  els.deleteConfirmBtn.disabled = false;
  els.deleteConfirmBtn.textContent = 'Delete';
  els.deleteModalBackdrop.classList.remove('hidden');
}

function closeDeleteModal() {
  els.deleteModalBackdrop.classList.add('hidden');
}

async function confirmDelete() {
  const w = state.deleteTrMain._word;
  els.deleteConfirmBtn.disabled = true;
  els.deleteConfirmBtn.innerHTML = '<span class="spinner"></span>';
  els.deleteError.classList.add('hidden');

  try {
    const res = await fetch('/api/words/' + w.id, { method: 'DELETE' });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    state.words.splice(state.words.indexOf(w), 1);
    state.deleteTrMain.closest('tbody').remove();
    updateWordCount();
    closeDeleteModal();
  } catch (err) {
    els.deleteError.textContent = err.message;
    els.deleteError.classList.remove('hidden');
    els.deleteConfirmBtn.disabled = false;
    els.deleteConfirmBtn.textContent = 'Delete';
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
    if (addEditState.addPhase !== 'loading' && addEditState.pendingGenerates === 0) closeAddResultModal();
  }
});

// Sort buttons: active state, direction toggle, and sorting
els.sortBtns.forEach(btn => {
  btn.addEventListener('click', e => {
    e.stopPropagation();
    const wasActive = btn.classList.contains('btn-sort--active');
    els.sortBtns.forEach(b => {
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

// --- Static element event listeners ---
els.headerAddBtn.addEventListener('click', openAddModal);

els.addModalBackdrop.addEventListener('click', e => onBackdropClick(e, closeAddModal));
els.addModalCloseBtn.addEventListener('click', closeAddModal);
els.addModalCancelBtn.addEventListener('click', closeAddModal);

els.deleteModalBackdrop.addEventListener('click', e => onBackdropClick(e, closeDeleteModal));
els.deleteModalCloseBtn.addEventListener('click', closeDeleteModal);
els.deleteModalCancelBtn.addEventListener('click', closeDeleteModal);
els.deleteConfirmBtn.addEventListener('click', confirmDelete);

// --- Word list sidebar ---

function setAddModalStatus(msg) {
  els.addModalStatus.textContent = msg;
}

function listItemTooltip(slug, total, inLexicon) {
  const c = state.wordListCache.get(slug);
  const lexCount = c ? c.inLexicon : inLexicon;
  const available = total - lexCount;
  const added = c ? (c.initialAvailable - c.remaining.length) : 0;
  const remaining = c ? c.remaining.length : available;
  return total + ' total · ' + lexCount + ' in lexicon · ' + added + ' added · ' + remaining + ' remaining';
}

async function initWordListSidebar() {
  try {
    const res = await fetch('/api/wordlists');
    if (!res.ok) return;
    const lists = await res.json();

    const title = document.createElement('div');
    title.className = 'add-modal-sidebar-title';
    title.textContent = 'Word lists';
    els.addModalSidebar.appendChild(title);

    for (const list of lists) {
      const item = document.createElement('div');
      item.className = 'add-modal-sidebar-item';
      item.dataset.tooltip = listItemTooltip(list.slug, list.total, list.tracked);

      const btn = document.createElement('button');
      btn.className = 'add-modal-sidebar-add-btn';
      btn.textContent = '+';
      btn.addEventListener('click', () => addWordFromList(list.slug, list.name, list.total, list.tracked, btn, item));

      const label = document.createElement('span');
      label.className = 'add-modal-sidebar-name';
      label.textContent = list.name;

      item.appendChild(btn);
      item.appendChild(label);
      els.addModalSidebar.appendChild(item);
    }
  } catch (_) { /* fail silently — sidebar stays empty */ }
}

async function addWordFromList(slug, name, total, inLexicon, btn, item) {
  btn.disabled = true;
  try {
    // Fetch and cache the available words on first click.
    if (!state.wordListCache.has(slug)) {
      const res = await fetch('/api/wordlists/' + encodeURIComponent(slug) + '/words');
      if (!res.ok) { setAddModalStatus('Failed to load the ' + name + ' list.'); return; }
      const data = await res.json();
      const words = data.words ?? [];
      state.wordListCache.set(slug, {
        remaining: words,
        total: data.total ?? total,
        inLexicon: data.tracked ?? inLexicon,
        initialAvailable: words.length,
      });
    }

    const c = state.wordListCache.get(slug);
    if (c.remaining.length === 0) {
      setAddModalStatus('No more words to add from the ' + name + ' list.');
      return;
    }

    const idx = Math.floor(Math.random() * c.remaining.length);
    const word = c.remaining.splice(idx, 1)[0];

    const displayText = word + ' (from the ' + name + ' word list)';
    els.addWordsInput.value = els.addWordsInput.value ? displayText + '\n' + els.addWordsInput.value : displayText;
    els.addWordsInput.scrollTop = 0;
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
  els.addWordsInput.value = '';
  els.addModalStatus.textContent = '';
  els.addModalBackdrop.classList.remove('hidden');
  els.addWordsInput.focus();
}

export function closeAddModal() {
  els.addModalBackdrop.classList.add('hidden');
}
