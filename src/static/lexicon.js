import { openEditModal, closeAddResultModal, state as addEditState } from './lexicon-add-edit.js';
import { timeAgo, getSortedWords as _getSortedWords, renderReading, getFirstImageFile, esc } from './lexicon-utils.js';
import { playTts, playWordAudio, playSentenceAudio, checkVoicevoxAvailable } from './common.js';
import { bindBackdropClose, setModalOpen } from './modal-utils.js';
import { computeVisibleRange, mergeWordPage, removeWordAtIndex } from './lexicon-virtual.js';

const LEXICON_AUDIO_OPTIONS = { preferSynthesis: true, fallbackToBrowserTts: true };
const VIRTUAL_PAGE_SIZE = 80;
const VIRTUAL_OVERSCAN = 6;

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
  wordTableBody: document.getElementById('word-table-body'),
  wordTableScroll: document.getElementById('word-table-scroll'),
  wordTableTopSpacer: document.getElementById('word-table-top-spacer'),
  wordTableBottomSpacer: document.getElementById('word-table-bottom-spacer'),
};
els.addModalCloseBtn = els.addModalBackdrop.querySelector('.modal-close');
els.addModalCancelBtn = els.addModalBackdrop.querySelector('.btn-cancel');
els.deleteModalCloseBtn = els.deleteModalBackdrop.querySelector('.modal-close');
els.deleteModalCancelBtn = els.deleteModalBackdrop.querySelector('.btn-cancel');

export const state = {
  activeWords: 0,
  deleteRowEl: null,
  imageSources: null,
  loadedPages: new Set(),
  pendingPages: new Map(),
  providers: null,
  requestVersion: 0,
  totalWords: 0,
  voicevoxAvailable: false,
  wordListCache: new Map(),
  wordSlots: [],
  words: [],
};

let activeDropGroup = null;
let virtualItemHeight = 96;

function readVirtualItemHeight() {
  const cssValue = getComputedStyle(document.documentElement)
    .getPropertyValue('--lexicon-item-height')
    .trim();
  const parsed = Number.parseFloat(cssValue);
  virtualItemHeight = Number.isFinite(parsed) && parsed > 0 ? parsed : 96;
}

function currentSort() {
  const activeBtn = els.sortBtns.find(btn => btn.classList.contains('btn-sort--active'));
  return {
    sort: activeBtn?.dataset.sort || 'added',
    dir: activeBtn?.dataset.dir || 'desc',
  };
}

function syncCachedWords() {
  state.words = state.wordSlots.filter(Boolean);
}

function updateWordCount() {
  const total = state.totalWords || 0;
  const active = state.activeWords || 0;
  els.wordCount.textContent = total + ' words (' + active + ' active)';
}

function clearDropTarget() {
  if (!activeDropGroup?.isConnected) {
    activeDropGroup = null;
    return;
  }
  activeDropGroup.classList.remove('word-group--drop-target');
  activeDropGroup = null;
}

function setDropTarget(group) {
  if (activeDropGroup === group) return;
  clearDropTarget();
  activeDropGroup = group;
  activeDropGroup?.classList.add('word-group--drop-target');
}

function clearFailedDropState(group) {
  group?.classList.remove('word-group--upload-failed');
}

function getWordRowContext(group) {
  const wordId = parseInt(group?.dataset.wordId || '', 10);
  const word = group?._word || null;
  return { group, wordId, word };
}

async function uploadWordImage(group, file, actionDescription = 'set image') {
  const { wordId, word } = getWordRowContext(group);
  if (!group || !wordId || !word || group.classList.contains('word-group--uploading')) return false;

  clearFailedDropState(group);
  group.classList.add('word-group--uploading');

  try {
    const formData = new FormData();
    formData.append('image', file);
    const res = await fetch('/api/words/' + wordId + '/upload-image', {
      method: 'POST',
      body: formData,
    });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    const data = await res.json();
    updateWordImagePath(wordId, data.image_path);
  } catch (err) {
    if (group.isConnected) group.classList.remove('word-group--uploading');
    console.error('Failed to ' + actionDescription + ' for "' + word.word + '":', err);
    return false;
  }
  if (group.isConnected) group.classList.remove('word-group--uploading');
  return true;
}

function isExternalFileDrag(event) {
  return Array.from(event.dataTransfer?.types || []).includes('Files');
}

function onWordTableDragOver(event) {
  if (!isExternalFileDrag(event)) return;
  const group = event.target.closest('.word-group');
  if (!group) {
    clearDropTarget();
    return;
  }
  event.preventDefault();
  event.dataTransfer.dropEffect = 'copy';
  clearFailedDropState(group);
  setDropTarget(group);
}

function onWordTableDragLeave(event) {
  if (!activeDropGroup) return;
  const nextTarget = event.relatedTarget;
  if (nextTarget && activeDropGroup.contains(nextTarget)) return;
  if (event.target === activeDropGroup || !activeDropGroup.contains(event.target)) clearDropTarget();
}

async function onWordTableDrop(event) {
  if (!isExternalFileDrag(event)) return;
  event.preventDefault();
  const group = event.target.closest('.word-group');
  clearDropTarget();
  if (!group) return;

  const file = getFirstImageFile(event.dataTransfer?.files);
  if (!file) {
    console.error('Dropped item is not an image file.');
    return;
  }
  await uploadWordImage(group, file, 'set image');
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

function renderWordGroupHTML(w) {
  const imgCell = w.imagePath
    ? '<div class="lex-cell lex-cell--img"><div class="cell-img"><img src="/static/' + w.imagePath + '" alt=""></div></div>'
    : '<div class="lex-cell lex-cell--img"><div class="cell-img"></div></div>';
  const exampleHtml = (w.exampleJp?.trim() || w.exampleEn?.trim())
    ? '<span class="cell-meaning-example">' +
        (w.exampleJp?.trim() ? '<span class="cell-ex-flag">🇯🇵</span> <span class="cell-ex-jp" data-tooltip="Example sentence">' + esc(w.exampleJp) + '</span>' : '') +
        (w.exampleEn?.trim() ? '<span class="cell-ex-sep"></span><span class="cell-ex-flag cell-ex-flag--en">🇬🇧</span> <span class="cell-ex-en" data-tooltip="Example sentence">' + esc(w.exampleEn) + '</span>' : '') +
      '</span>'
    : '';

  return (
    imgCell +
      '<div class="lex-cell lex-cell--word"><div class="cell-word">' +
        '<span class="cell-word-label">' + esc(w.word) + '</span>' +
        '<button class="btn-edit" data-word-id="' + w.id + '" data-tooltip="Edit word">✎</button>' +
        '<button class="btn-delete" data-tooltip="Delete word">✕</button>' +
      '</div></div>' +
      '<div class="lex-cell lex-cell--reading cell-reading" data-tooltip="Reading (Pronunciation)">' + renderReading(w.reading, w.word, w.kanjiData, w.pitchAccent) + '</div>' +
      '<div class="lex-cell lex-cell--type"><span class="type-badge" data-tooltip="Part of Speech">' + esc(w.type) + '</span></div>' +
      '<div class="lex-cell lex-cell--meaning cell-meaning"><div class="cell-meaning-inner" data-tooltip="Meaning">' + esc(w.meaning) + '</div></div>' +
      '<div class="lex-cell lex-cell--correct cell-correct" data-tooltip="Times answered correctly">' + w.correct + '</div>' +
      '<div class="lex-cell lex-cell--incorrect cell-incorrect" data-tooltip="Times answered incorrectly">' + w.incorrect + '</div>' +
      '<div class="lex-cell lex-cell--target cell-target" data-tooltip="Remaining drills to target">' +
        '<div class="target-stepper">' +
          '<button class="btn-target-adj" data-delta="-4">−</button>' +
          '<span>' + w.target + '</span>' +
          '<button class="btn-target-adj" data-delta="4">+</button>' +
        '</div>' +
      '</div>' +
      '<div class="lex-cell lex-cell--date cell-date">' +
        '<span class="cell-date-added" data-tooltip="Date added: ' + fullDateTime(w.createdAt) + '">added ' + timeAgo(w.createdAt) + '</span>' +
        '<span class="cell-date-sep"> · </span>' +
        (w.lastDrilled
          ? '<span class="cell-date-drilled" data-tooltip="Last drilled: ' + fullDateTime(w.lastDrilled) + '">drilled ' + timeAgo(w.lastDrilled) + '</span>'
          : '<span class="cell-date-drilled cell-date-never">never drilled</span>') +
      '</div>' +
      '<div class="lex-cell lex-cell--example cell-ex">' +
        exampleHtml +
      '</div>'
  );
}

function renderPlaceholderGroup() {
  const group = document.createElement('div');
  group.className = 'word-group word-group--placeholder';
  group.innerHTML =
    '<div class="lex-cell lex-cell--placeholder">' +
      '<div class="placeholder-grid">' +
        '<div class="lex-cell lex-cell--img"><div class="cell-img"></div></div>' +
        '<div class="lex-cell lex-cell--word"><span class="row-skeleton row-skeleton--word"></span></div>' +
        '<div class="lex-cell lex-cell--reading"><span class="row-skeleton row-skeleton--reading"></span></div>' +
        '<div class="lex-cell lex-cell--type"><span class="row-skeleton row-skeleton--type"></span></div>' +
        '<div class="lex-cell lex-cell--meaning"><span class="row-skeleton row-skeleton--meaning"></span></div>' +
        '<div class="lex-cell lex-cell--correct"><span class="row-skeleton"></span></div>' +
        '<div class="lex-cell lex-cell--incorrect"><span class="row-skeleton"></span></div>' +
        '<div class="lex-cell lex-cell--target"><span class="row-skeleton"></span></div>' +
        '<div class="lex-cell lex-cell--date"><span class="row-skeleton row-skeleton--meta"></span></div>' +
        '<div class="lex-cell lex-cell--example"><span class="row-skeleton row-skeleton--example"></span></div>' +
      '</div>' +
    '</div>';
  return group;
}

function positionWordGroup(group, index) {
  group.dataset.index = index;
  group.style.transform = 'translateY(' + (index * virtualItemHeight) + 'px)';
  return group;
}

function buildWordGroup(w, index) {
  const group = document.createElement('div');
  group.className = 'word-group';
  group.dataset.wordId = w.id;
  group._word = w;
  group.innerHTML = renderWordGroupHTML(w);
  return positionWordGroup(group, index);
}

function visibleRange() {
  return computeVisibleRange({
    scrollTop: els.wordTableScroll.scrollTop,
    viewportHeight: els.wordTableScroll.clientHeight || 1,
    itemHeight: virtualItemHeight,
    totalItems: state.totalWords,
    overscan: VIRTUAL_OVERSCAN,
  });
}

function ensureVisiblePagesLoaded() {
  if (state.totalWords === 0) return;
  const { start, end } = visibleRange();
  const firstPage = Math.floor(start / VIRTUAL_PAGE_SIZE) * VIRTUAL_PAGE_SIZE;
  const lastPage = Math.floor(Math.max(0, end - 1) / VIRTUAL_PAGE_SIZE) * VIRTUAL_PAGE_SIZE;
  for (let offset = firstPage; offset <= lastPage; offset += VIRTUAL_PAGE_SIZE) {
    fetchWordPage(offset);
  }
}

async function fetchWordPage(offset) {
  if (state.loadedPages.has(offset) || state.pendingPages.has(offset)) return state.pendingPages.get(offset);
  const version = state.requestVersion;
  const { sort, dir } = currentSort();
  const url = new URL('/api/words', window.location.origin);
  url.searchParams.set('sort', sort);
  url.searchParams.set('dir', dir);
  url.searchParams.set('offset', String(offset));
  url.searchParams.set('limit', String(VIRTUAL_PAGE_SIZE));

  const pending = fetch(url)
    .then(r => {
      if (!r.ok) throw new Error('Failed to load words');
      return r.json();
    })
    .then(page => {
      if (version !== state.requestVersion) return;
      state.totalWords = page.total || 0;
      state.activeWords = page.activeTotal || 0;
      state.wordSlots = mergeWordPage(state.wordSlots, offset, page.items || [], state.totalWords);
      syncCachedWords();
      state.loadedPages.add(offset);
      updateWordCount();
      renderTable();
      ensureVisiblePagesLoaded();
    })
    .finally(() => {
      state.pendingPages.delete(offset);
    });

  state.pendingPages.set(offset, pending);
  return pending;
}

function findWordIndex(wordId) {
  return state.wordSlots.findIndex(word => word && word.id === wordId);
}

function mutateCachedWord(wordId, mutateFn) {
  let changedWord = null;
  state.wordSlots = state.wordSlots.map(word => {
    if (!word || word.id !== wordId) return word;
    changedWord = mutateFn({ ...word });
    return changedWord;
  });
  syncCachedWords();
  return changedWord;
}

async function resetAndReload({ preserveScroll = false } = {}) {
  const priorScrollTop = preserveScroll ? els.wordTableScroll.scrollTop : 0;
  const priorTotalWords = state.totalWords;
  const priorActiveWords = state.activeWords;
  state.requestVersion++;
  state.loadedPages = new Set();
  state.pendingPages = new Map();
  state.totalWords = preserveScroll ? priorTotalWords : 0;
  state.activeWords = preserveScroll ? priorActiveWords : 0;
  state.wordSlots = [];
  syncCachedWords();
  updateWordCount();
  els.wordTableScroll.scrollTop = preserveScroll ? priorScrollTop : 0;
  renderTable();
  await fetchWordPage(0);
  ensureVisiblePagesLoaded();
}

export function getSortedWords() {
  return _getSortedWords(state.words, currentSort().sort, currentSort().dir);
}

export function renderTable() {
  const { start, end } = visibleRange();
  els.wordTableTopSpacer.style.height = '0px';
  els.wordTableBottomSpacer.style.height = '0px';
  els.wordTableBody.style.height = (state.totalWords * virtualItemHeight) + 'px';
  els.wordTableBody.innerHTML = '';
  const frag = document.createDocumentFragment();
  for (let i = start; i < end; i++) {
    const word = state.wordSlots[i];
    frag.appendChild(word ? buildWordGroup(word, i) : positionWordGroup(renderPlaceholderGroup(), i));
  }
  els.wordTableBody.appendChild(frag);
}

export async function reloadWords() {
  await resetAndReload({ preserveScroll: true });
}

export function updateWordImagePath(wordId, imagePath) {
  const changedWord = mutateCachedWord(wordId, word => {
    word.imagePath = imagePath;
    return word;
  });
  if (!changedWord) return;
  renderTable();
}

async function init() {
  readVirtualItemHeight();
  const providers = await fetch('/api/providers').then(r => r.json());
  state.providers = providers.ai;
  state.imageSources = providers.image_sources;
  state.voicevoxAvailable = await checkVoicevoxAvailable();
  await resetAndReload();
}

init();

els.wordTableScroll.addEventListener('scroll', () => {
  renderTable();
  ensureVisiblePagesLoaded();
});
if ('ResizeObserver' in window) {
  const wordTableResizeObserver = new ResizeObserver(() => {
    readVirtualItemHeight();
    renderTable();
    ensureVisiblePagesLoaded();
  });
  wordTableResizeObserver.observe(els.wordTableScroll);
}
els.wordTableBody.addEventListener('dragover', onWordTableDragOver);
els.wordTableBody.addEventListener('dragleave', onWordTableDragLeave);
els.wordTableBody.addEventListener('drop', onWordTableDrop);

function openDeleteModal(event) {
  event.stopPropagation();
  state.deleteRowEl = event.target.closest('.word-group');
  const w = state.deleteRowEl?._word;
  if (!w) return;
  els.deleteModalLabel.textContent = w.word;
  els.deleteError.classList.add('hidden');
  els.deleteConfirmBtn.disabled = false;
  els.deleteConfirmBtn.textContent = 'Delete';
  setModalOpen(els.deleteModalBackdrop, true);
}

function closeDeleteModal() {
  setModalOpen(els.deleteModalBackdrop, false);
}

async function confirmDelete() {
  const w = state.deleteRowEl?._word;
  if (!w) return;
  els.deleteConfirmBtn.disabled = true;
  els.deleteConfirmBtn.innerHTML = '<span class="spinner"></span>';
  els.deleteError.classList.add('hidden');

  try {
    const res = await fetch('/api/words/' + w.id, { method: 'DELETE' });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    const idx = findWordIndex(w.id);
    if (idx >= 0) {
      state.wordSlots = removeWordAtIndex(state.wordSlots, idx);
      state.totalWords = Math.max(0, state.totalWords - 1);
      if (w.correct < w.target) state.activeWords = Math.max(0, state.activeWords - 1);
      syncCachedWords();
      updateWordCount();
      renderTable();
    }
    closeDeleteModal();
    await resetAndReload({ preserveScroll: true });
  } catch (err) {
    els.deleteError.textContent = err.message;
    els.deleteError.classList.remove('hidden');
    els.deleteConfirmBtn.disabled = false;
    els.deleteConfirmBtn.textContent = 'Delete';
  }
}

function adjustTargetInline(event) {
  event.stopPropagation();
  const group = event.target.closest('.word-group');
  const w = group?._word;
  if (!w) return;
  const delta = parseInt(event.target.dataset.delta || '', 10);
  const newTarget = Math.max(w.correct, w.target + delta);
  if (newTarget === w.target) return;

  const wasActive = w.correct < w.target;
  const willBeActive = w.correct < newTarget;
  mutateCachedWord(w.id, word => {
    word.target = newTarget;
    return word;
  });
  w.target = newTarget;
  if (wasActive !== willBeActive) state.activeWords += willBeActive ? 1 : -1;
  updateWordCount();
  renderTable();

  fetch('/api/words/' + w.id + '/target', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target: newTarget }),
  });

  if (currentSort().sort === 'target') {
    resetAndReload({ preserveScroll: true });
  }
}

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    closeAddModal();
    closeDeleteModal();
    if (addEditState.addPhase !== 'loading' && addEditState.pendingGenerates === 0) closeAddResultModal();
  }
});

els.sortBtns.forEach(btn => {
  btn.addEventListener('click', async e => {
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
    await resetAndReload();
  });
});

els.headerAddBtn.addEventListener('click', openAddModal);

bindBackdropClose(els.addModalBackdrop, closeAddModal);
els.addModalCloseBtn.addEventListener('click', closeAddModal);
els.addModalCancelBtn.addEventListener('click', closeAddModal);

bindBackdropClose(els.deleteModalBackdrop, closeDeleteModal);
els.deleteModalCloseBtn.addEventListener('click', closeDeleteModal);
els.deleteModalCancelBtn.addEventListener('click', closeDeleteModal);
els.deleteConfirmBtn.addEventListener('click', confirmDelete);

els.wordTableBody.addEventListener('click', event => {
  const group = event.target.closest('.word-group');
  const word = group?._word;
  if (!word) return;

  if (event.target.closest('.btn-edit')) {
    openEditModal(event);
    return;
  }
  if (event.target.closest('.btn-delete')) {
    openDeleteModal(event);
    return;
  }
  if (event.target.closest('.cell-word-label') || event.target.closest('.cell-word')) {
    playWordAudio(word, 1, LEXICON_AUDIO_OPTIONS);
    return;
  }
  if (event.target.closest('.cell-ex-jp')) {
    playSentenceAudio(word, 1, LEXICON_AUDIO_OPTIONS);
    return;
  }
  if (event.target.closest('.cell-ex-en')) {
    playTts(word.exampleEn, 'en-US');
  }
});

els.wordTableBody.addEventListener('mousedown', event => {
  if (event.target.closest('.btn-target-adj')) adjustTargetInline(event);
});

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
      label.addEventListener('click', () => btn.click());

      item.appendChild(btn);
      item.appendChild(label);
      els.addModalSidebar.appendChild(item);
    }
  } catch (_) { /* fail silently — sidebar stays empty */ }
}

async function addWordFromList(slug, name, total, inLexicon, btn, item) {
  btn.disabled = true;
  try {
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

    const displayText = word + '  ' + name;
    els.addWordsInput.value = els.addWordsInput.value ? displayText + '\n' + els.addWordsInput.value : displayText;
    els.addWordsInput.scrollTop = 0;
    setAddModalStatus('One random word added from the ' + name + ' list.');
    item.dataset.tooltip = listItemTooltip(slug, c.total, c.inLexicon);
  } catch (_) {
    setAddModalStatus('Failed to load the ' + name + ' list.');
  } finally {
    btn.disabled = false;
  }
}

initWordListSidebar();

function openAddModal() {
  els.addWordsInput.value = '';
  els.addModalStatus.textContent = '';
  setModalOpen(els.addModalBackdrop, true);
  els.addWordsInput.focus();
}

export function closeAddModal() {
  setModalOpen(els.addModalBackdrop, false);
}
