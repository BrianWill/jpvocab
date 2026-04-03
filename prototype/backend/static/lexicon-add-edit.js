import { defaultDrillTarget, typeLabels, _providers, _imageSources, _voicevoxAvailable, _ffmpegAvailable, reloadWords, renderTable, getSortedWords, closeAddModal, updateWordImagePath, updateWordAudioFlags } from './lexicon.js';
import { esc, isKanji, detailItemPosSelect, detailItemKanjiReadings, detailItemInput, detailItemExInput } from './lexicon-utils.js';
import { getVoicevoxSettings, playWordAudio, playSentenceAudio } from './common.js';

const imagePlaceholderSvg =
  '<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">' +
    '<rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="1.5"/>' +
    '<circle cx="8.5" cy="8.5" r="1.5" fill="currentColor"/>' +
    '<polyline points="3,21 8,14 12,18 16,13 21,18" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/>' +
  '</svg>';

function buildWordResultImage(imagePath, state, bust = '') {
  if (imagePath) {
    return '<div class="word-result-image"><img src="/static/' + esc(imagePath) + (bust ? '?v=' + bust : '') + '" alt=""></div>';
  }
  const classes = ['word-result-image', 'word-result-image--empty'];
  let overlay = '';
  if (state === 'loading') {
    classes.push('word-result-image--loading');
    overlay = '<span class="spinner word-result-image-spinner" aria-hidden="true"></span>';
  } else if (state === 'failed') {
    classes.push('word-result-image--failed');
  }
  return '<div class="' + classes.join(' ') + '">' + overlay + imagePlaceholderSvg + '</div>';
}

function setWordRowImage(row, imagePath, state = '', bust = '') {
  const imageEl = row.querySelector('.word-result-image');
  if (!imageEl) return;
  imageEl.outerHTML = buildWordResultImage(imagePath, state, bust);
}

// --- Add/edit modal ---
// Handles two scenarios:
//   1. After "Add words": words stream in via SSE, filling placeholder rows one
//      by one as the server processes them (saveAddModal → appendWordRow).
//   2. From the edit button (✎) on a lexicon row: opens with a single word,
//      bypassing the streaming machinery entirely (openEditModal).
// The word row helpers (appendWordRow, saveWordRowEdits, etc.) serve both cases.

// --- Edit modal (reuses add-result modal with a single word) ---
export function openEditModal(event) {
  event.stopPropagation();
  const trMain = event.target.closest('tr');
  const w = trMain._word;

  _addPhase = 'done';
  _isSingleEdit = true;
  _addedWords = [];
  _skippedCount = 0;
  _pendingGenerates = 0;
  _abortController = null;

  const resultBody = document.getElementById('add-result-modal-body');
  resultBody.innerHTML = '';

  appendWordRow({
    word: w.word,
    word_id: w.id,
    added: true,
    reading: w.reading,
    part_of_speech: w.type,
    meaning: w.meaning,
    example_jp: w.exampleJp,
    example_en: w.exampleEn,
    drill_count: w.correct,
    drill_incorrect: w.incorrect,
    drill_target: w.target,
    kanji_data: w.kanjiData,
    image_path: w.imagePath,
    has_word_audio: w.hasWordAudio,
    has_sentence_audio: w.hasSentenceAudio,
  });

  document.getElementById('add-result-modal-backdrop').classList.remove('hidden');
  initAddResultFooter();
  document.getElementById('btn-add-result-remove').style.display = 'none';
  renderStatus();
  resultBody.querySelector('.result-badge').style.display = 'none';
}

export let _addPhase = 'idle'; // 'loading' | 'done' | 'cancelled'
let _isSingleEdit = false;
let _addedWords = [];
let _skippedCount = 0;
export let _pendingGenerates = 0;
let _abortController = null;

document.getElementById('add-result-modal-backdrop').addEventListener('click', function (e) {
  if (e.target === this && _addPhase !== 'loading' && _pendingGenerates === 0) closeAddResultModal();
});
document.getElementById('add-result-modal-close').addEventListener('click', closeAddResultModal);

document.getElementById('add-result-modal-body').addEventListener('click', e => {
  if (!e.target.closest('.detail-ex-play')) return;
  const row = e.target.closest('.word-result-row');
  const jpInput = e.target.closest('.detail-ex-inputs')?.querySelector('.detail-input:not(.detail-input--en)');
  const text = jpInput?.textContent.trim();
  if (text) playSentenceAudio({
    word: row?._resolvedWord ?? '',
    hasSentenceAudio: row?._hasSentenceAudio ?? false,
    exampleJp: text,
  });
});

// --- Remove-confirm mini-modal ---
let _pendingRemoveAction = null;
function openRemoveConfirm(message, action) {
  _pendingRemoveAction = action;
  document.getElementById('remove-confirm-text').textContent = message;
  document.getElementById('remove-confirm-modal-backdrop').classList.remove('hidden');
}
function closeRemoveConfirm() {
  _pendingRemoveAction = null;
  document.getElementById('remove-confirm-modal-backdrop').classList.add('hidden');
}
document.getElementById('remove-confirm-modal-backdrop').addEventListener('click', e => {
  if (e.target === document.getElementById('remove-confirm-modal-backdrop')) closeRemoveConfirm();
});
document.getElementById('remove-confirm-cancel').addEventListener('click', closeRemoveConfirm);
document.getElementById('remove-confirm-ok').addEventListener('click', () => {
  const action = _pendingRemoveAction;
  closeRemoveConfirm();
  if (action) action();
});
// --- Generate-confirm mini-modal ---
function openGenerateConfirm() {
  const addedCount   = document.querySelectorAll('#add-result-modal-body .result-added .btn-generate:not(.btn-generate--busy):not([disabled])').length;
  const skippedCount = document.querySelectorAll('#add-result-modal-body .result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])').length;

  document.getElementById('generate-confirm-added-text').textContent   = addedCount   + ' newly added words';
  document.getElementById('generate-confirm-skipped-text').textContent = skippedCount + ' already existing words';
  document.getElementById('generate-confirm-added-checkbox').checked   = true;
  document.getElementById('generate-confirm-skipped-checkbox').checked = false;

  document.getElementById('generate-confirm-modal-backdrop').classList.remove('hidden');
}
function closeGenerateConfirm() {
  document.getElementById('generate-confirm-modal-backdrop').classList.add('hidden');
}
document.getElementById('generate-confirm-modal-backdrop').addEventListener('click', e => {
  if (e.target === document.getElementById('generate-confirm-modal-backdrop')) closeGenerateConfirm();
});
document.getElementById('generate-confirm-cancel').addEventListener('click', closeGenerateConfirm);
document.getElementById('generate-confirm-ok').addEventListener('click', () => {
  const includeAdded   = document.getElementById('generate-confirm-added-checkbox').checked;
  const includeSkipped = document.getElementById('generate-confirm-skipped-checkbox').checked;
  closeGenerateConfirm();
  generateAll(includeAdded, includeSkipped);
});

document.querySelector('#add-modal-backdrop .btn-save').addEventListener('click', saveAddModal);

// Prevent newlines in contenteditable fields; Enter blurs instead
document.getElementById('add-result-modal-body').addEventListener('keydown', function(e) {
  if (e.key !== 'Enter') return;
  if (e.isComposing) return; // let IME handle its own Enter (commit keystroke)
  if (!e.target.classList.contains('detail-input')) return;
  e.preventDefault();
  e.target.blur();
});

// Language enforcement for detail input fields
function _getFieldLanguageFilter(el) {
  if (el.closest('.detail-ex')) {
    const isEn = el.classList.contains('detail-input--en');
    return text => isEn
      ? text.replace(/[\u3040-\u30FF\u4E00-\u9FFF\u3400-\u4DBF\uFF01-\uFF9F]/g, '')
      : text.replace(/[a-zA-Z]/g, '');
  }
  if (el.closest('.detail-reading') || el.classList.contains('kanji-reading-input')) {
    return text => text.replace(/[a-zA-Z]/g, '');
  }
  return null;
}
function _getFieldLanguageErrorMsg(el) {
  if (el.closest('.detail-ex') && el.classList.contains('detail-input--en')) return 'English only — Japanese characters are not allowed here';
  return 'Japanese only — Latin letters are not allowed here';
}
let _fieldErrorTimer = null;
function _showFieldError(el, msg) {
  el.classList.remove('detail-input--flash-error');
  void el.offsetWidth; // force reflow to restart animation
  el.classList.add('detail-input--flash-error');
  el.addEventListener('animationend', () => el.classList.remove('detail-input--flash-error'), { once: true });

  const footer = document.getElementById('add-result-modal-footer');
  let errEl = footer.querySelector('.footer-field-error');
  if (!errEl) {
    errEl = document.createElement('span');
    errEl.className = 'footer-field-error';
    const closeBtn = document.getElementById('btn-add-result-close');
    footer.insertBefore(errEl, closeBtn);
  }
  errEl.textContent = msg;
  clearTimeout(_fieldErrorTimer);
  _fieldErrorTimer = setTimeout(() => errEl.remove(), 3000);
}
function _enforceFieldLanguage(el) {
  const filter = _getFieldLanguageFilter(el);
  if (!filter) return;
  const original = el.textContent;
  const filtered = filter(original);
  if (filtered === original) return;
  const sel = window.getSelection();
  const rawOffset = sel.rangeCount > 0 ? sel.getRangeAt(0).startOffset : 0;
  const removedBefore = rawOffset - filter(original.slice(0, rawOffset)).length;
  const newOffset = Math.max(0, rawOffset - removedBefore);
  el.textContent = filtered;
  if (el.firstChild) {
    const range = document.createRange();
    range.setStart(el.firstChild, Math.min(newOffset, filtered.length));
    range.collapse(true);
    sel.removeAllRanges();
    sel.addRange(range);
  }
  _showFieldError(el, _getFieldLanguageErrorMsg(el));
}
const _modalBody = document.getElementById('add-result-modal-body');
_modalBody.addEventListener('input', function(e) {
  if (e.isComposing) return;
  if (e.target.classList.contains('detail-input')) _enforceFieldLanguage(e.target);
});
_modalBody.addEventListener('compositionend', function(e) {
  if (e.target.classList.contains('detail-input')) _enforceFieldLanguage(e.target);
});
_modalBody.addEventListener('paste', function(e) {
  const el = e.target;
  if (!el.classList.contains('detail-input')) return;
  const filter = _getFieldLanguageFilter(el);
  if (!filter) return;
  e.preventDefault();
  const text = (e.clipboardData || window.clipboardData).getData('text/plain');
  document.execCommand('insertText', false, filter(text));
});

// Auto-save word info edits in the add-result modal
document.getElementById('add-result-modal-body').addEventListener('focusout', function(e) {
  if (!e.target.classList.contains('detail-input')) return;
  const row = e.target.closest('.word-result-row');
  if (row) saveWordRowEdits(row);
});
document.getElementById('add-result-modal-body').addEventListener('change', function(e) {
  if (!e.target.classList.contains('detail-pos-select')) return;
  const row = e.target.closest('.word-result-row');
  if (row) saveWordRowEdits(row);
});

export async function closeAddResultModal() {
  if (_addPhase === 'loading' || _pendingGenerates > 0) return;
  document.getElementById('add-result-modal-backdrop').classList.add('hidden');
  await reloadWords();
  const activeBtn = document.querySelector('.btn-sort--active');
  renderTable(getSortedWords(activeBtn.dataset.sort, activeBtn.dataset.dir || 'desc'));
}

async function saveAddModal() {
  function sortWordRows() {
    const body = document.getElementById('add-result-modal-body');
    const rows = Array.from(body.children);
    rows.sort((a, b) => {
      const aLexicon = a.dataset.reason === 'already in lexicon' ? 1 : 0;
      const bLexicon = b.dataset.reason === 'already in lexicon' ? 1 : 0;
      return aLexicon - bLexicon;
    });
    rows.forEach(r => body.appendChild(r));
  }
  function setModalStatus(type, text) {
    const el = document.getElementById('add-result-modal-status');
    const spinner = type === 'loading' ? '<span class="spinner"></span>' : '';
    el.className = 'modal-status modal-status-' + type;
    el.innerHTML = spinner + '<span>' + esc(text) + '</span>';
  }

  const rawText = document.getElementById('add-words-input').value.trim();
  if (!rawText) return;

  closeAddModal();

  _addPhase = 'loading';
  _isSingleEdit = false;
  _addedWords = [];
  _skippedCount = 0;
  _pendingGenerates = 0;
  _abortController = new AbortController();

  const resultBody = document.getElementById('add-result-modal-body');
  resultBody.innerHTML = '';
  document.getElementById('add-result-modal-backdrop').classList.remove('hidden');
  initAddResultFooter();
  document.getElementById('add-result-modal-status').style.display = '';
  renderStatus();

  const form = new FormData();
  form.append('words', rawText);
  form.append('autofill', 'off');

  try {
    const res = await fetch('/admin/words/batch', {
      method: 'POST', body: form, signal: _abortController.signal,
    });
    if (!res.ok) throw new Error(await res.text());

    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const lines = buf.split('\n');
      buf = lines.pop();
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue;
        const data = JSON.parse(line.slice(6));
        if (data.updated) { updateWordRowDetails(data); continue; }
        if (data.done) {
          _addPhase = 'done';
          clearAutofillSpinners();
          sortWordRows();
          renderStatus();
          await reloadWords();
          updateAddResultFooter();
          return;
        }
        if (data.added) _addedWords.push(data.word);
        else _skippedCount++;
        appendWordRow(data);
        renderStatus();
        updateAddResultFooter();
      }
    }
  } catch (err) {
    if (err.name === 'AbortError') {
      if (_addPhase === 'loading') {
        // Abort came from Cancel button — handle as cancellation.
        _addPhase = 'cancelled';
        clearAutofillSpinners();
        renderStatus();
        await reloadWords();
        updateAddResultFooter();
      }
      // else: abort was triggered by the Remove handler, which manages cleanup itself.
    } else {
      _addPhase = 'done';
      setModalStatus('done', 'Error: ' + err.message);
      await reloadWords();
      updateAddResultFooter();
    }
  }
}

function appendWordRow(data) {
  // Find the pre-inserted placeholder row for this word; fall back to appending a new one
  const body = document.getElementById('add-result-modal-body');
  let row = null;
  for (const el of body.children) {
    if (el._pendingWord === data.word) { row = el; break; }
  }
  if (!row) {
    row = document.createElement('div');
    body.appendChild(row);
  }
  row._pendingWord = null;
  row._resolvedWord = data.word;
  row._wordId = data.word_id || null;
  row.className = 'word-result-row ' + (data.added ? 'result-added' : 'result-skipped');
  row.dataset.reason = data.added ? 'added' : (data.reason || '');

  const badge = data.added
    ? '<span class="result-badge badge-added">added</span>'
    : '<span class="result-badge badge-skipped">' + esc(data.reason) + '</span>';

  const removeBtn =
    '<button class="btn-delete btn-word-remove" data-tooltip="Remove word"' +
      ' data-word="' + esc(data.word) + '">✕</button>';
  const hasProviders = _providers && (_providers.anthropic || _providers.openai || _providers.google || _providers.mistral || _providers.glm);
  const generateBtn = data.word_id
    ? '<button class="btn-generate"' +
        (hasProviders ? '' : ' disabled') +
        ' data-tooltip="Uses an AI API request to get the word\'s reading, part-of-speech, meaning, and an example sentence"' +
        '>generate</button>'
    : '';
  let inlineExtra;
  if (data.word_id) {
    const correct   = data.drill_count     ?? 0;
    const incorrect = data.drill_incorrect ?? 0;
    const target    = data.drill_target    ?? 0;
    inlineExtra =
      '<span class="word-result-drill">' +
        '<span class="word-result-actions">' + generateBtn + removeBtn + '</span>' +
        '<span class="drill-correct" data-tooltip="Times answered correctly">✓ ' + correct + '</span>' +
        '<span class="drill-incorrect" data-tooltip="Times answered incorrectly">✗ ' + incorrect + '</span>' +
        '<span class="target-stepper" data-tooltip="Remaining drills to target">' +
          '<span class="drill-target-label">🎯</span>' +
          '<span class="drill-target-val" data-target="' + target + '">' + target + '</span>' +
          '<button class="btn-target-adj">−</button>' +
          '<button class="btn-target-adj">+</button>' +
        '</span>' +
      '</span>';
  } else {
    inlineExtra = '<span class="word-result-drill">' + removeBtn + '</span>';
  }

  const details =
    '<div class="word-result-details">' +
      detailItemInput('reading', data.reading,        'detail-reading') +
      detailItemKanjiReadings(data.word, data.kanji_data) +
      detailItemPosSelect(data.part_of_speech, typeLabels) +
      detailItemInput('meaning', data.meaning,        'detail-meaning') +
      detailItemExInput(data.example_jp, data.example_en) +
    '</div>';

  const imageHtml = buildWordResultImage(data.image_path, '');

  row.innerHTML =
    '<div class="word-result-main"><span class="result-word">' + esc(data.word) + '</span>' + badge + inlineExtra + '</div>' +
    '<div class="word-result-body">' + details + imageHtml + '</div>';

  row._hasWordAudio = data.has_word_audio ?? false;
  row._hasSentenceAudio = data.has_sentence_audio ?? false;
  const resultWordEl = row.querySelector('.result-word');
  if (resultWordEl) resultWordEl.addEventListener('click', () =>
    playWordAudio({ word: data.word, hasWordAudio: row._hasWordAudio })
  );

  const removeBtnEl = row.querySelector('.btn-word-remove');
  if (removeBtnEl) removeBtnEl.addEventListener('mousedown', e => removeWordRow(e, removeBtnEl));

  if (data.word_id) {
    const genBtnEl = row.querySelector('.btn-generate');
    if (genBtnEl) genBtnEl.addEventListener('mousedown', e => {
      const t = getGenerateType();
      if (t === 'image') generateWordImage(e, data.word_id, data.word, genBtnEl);
      else if (t === 'audio') generateWordAudio(e, data.word_id, data.word, genBtnEl);
      else generateWordAutofill(e, data.word_id, data.word, genBtnEl);
    });

    const [adjMinusEl, adjPlusEl] = row.querySelectorAll('.btn-target-adj');
    if (adjMinusEl) adjMinusEl.addEventListener('mousedown', e => adjustWordTarget(e, data.word_id, -1, adjMinusEl));
    if (adjPlusEl) adjPlusEl.addEventListener('mousedown', e => adjustWordTarget(e, data.word_id, 1, adjPlusEl));
  }

  if (data.added && data.word_id && !data.image_path && data.suggested_image_url) {
    startSuggestedImageDownload(row, data.word_id, data.suggested_image_url);
  }
}

function updateWordRowDetails(data) {
  const body = document.getElementById('add-result-modal-body');
  let row = null;
  for (const el of body.children) {
    if (el._resolvedWord === data.word) { row = el; break; }
  }
  if (!row) return;
  const newDetails =
    '<div class="word-result-details">' +
      detailItemInput('reading', data.reading,        'detail-reading') +
      detailItemKanjiReadings(row._resolvedWord, data.kanji_data) +
      detailItemPosSelect(data.part_of_speech, typeLabels) +
      detailItemInput('meaning', data.meaning,        'detail-meaning') +
      detailItemExInput(data.example_jp, data.example_en) +
    '</div>';
  row.querySelector('.word-result-details').outerHTML = newDetails;
  const genBtn = row.querySelector('.btn-generate');
  if (genBtn && genBtn.classList.contains('btn-generate--busy') && !genBtn._generateAbort) {
    genBtn.classList.remove('btn-generate--busy');
    genBtn.innerHTML = 'generate';
    _pendingGenerates = Math.max(0, _pendingGenerates - 1);
    renderStatus();
  }
}

async function startSuggestedImageDownload(row, wordId, imageURL) {
  if (!row || !row.isConnected) return;
  setWordRowImage(row, '', 'loading');
  try {
    const res = await fetch('/api/words/' + wordId + '/download-image', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: imageURL }),
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    if (!row.isConnected) return;
    setWordRowImage(row, data.image_path, '', Date.now());
    updateWordImagePath(wordId, data.image_path);
  } catch (_) {
    if (!row.isConnected) return;
    setWordRowImage(row, '', 'failed');
  }
}

function getGenerateType() {
  return document.getElementById('add-result-generate-type')?.value ?? 'word-info';
}

function getImageSource() {
  return document.getElementById('add-result-image-source-select')?.value ?? 'wikimedia';
}

function audioReadyTooltip() {
  if (!_voicevoxAvailable) return 'VoiceVox is not running';
  if (!_ffmpegAvailable) return 'ffmpeg is not installed (required for audio generation)';
  return 'Generates audio via the local VoiceVox engine';
}

function updateGenerateBtnStates() {
  const type = getGenerateType();
  const hasProviders = _providers && (_providers.anthropic || _providers.openai || _providers.google || _providers.mistral || _providers.glm);
  const audioReady = _voicevoxAvailable && _ffmpegAvailable;
  const disabled = type === 'audio' ? !audioReady : !hasProviders;
  const tooltip = type === 'audio'
    ? audioReadyTooltip()
    : type === 'image'
      ? 'Uses an AI API request to find and download an image for this word'
      : 'Uses an AI API request to get the word\'s reading, part-of-speech, meaning, and an example sentence';
  document.querySelectorAll('#add-result-modal-body .btn-generate:not(.btn-generate--busy)').forEach(btn => {
    btn.disabled = disabled;
    btn.dataset.tooltip = tooltip;
  });
}

async function generateWordAutofill(event, wordId, word, btn) {
  event.stopPropagation();
  if (btn._generateAbort) {
    btn._generateAbort.abort();
    return; // ongoing call's finally handles cleanup
  }
  if (btn.classList.contains('btn-generate--busy')) return; // batch autofill in progress
  const abort = new AbortController();
  btn._generateAbort = abort;
  btn.classList.add('btn-generate--busy', 'btn-generate--cancellable');
  btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">generating\u2026</span><span class="btn-gen-cancel">cancel generation</span>';
  _pendingGenerates++;
  renderStatus();
  const aiModel = document.getElementById('add-result-model-select').value;
  try {
    const res = await fetch('/api/words/' + wordId + '/autofill', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ word, ai_model: aiModel }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    data.word = word;
    updateWordRowDetails(data);
  } finally {
    if (btn._generateAbort === abort) {
      btn._generateAbort = null;
      if (btn.classList.contains('btn-generate--busy')) {
        btn.classList.remove('btn-generate--busy', 'btn-generate--cancellable');
        btn.innerHTML = 'generate';
        _pendingGenerates = Math.max(0, _pendingGenerates - 1);
        renderStatus();
      }
    }
  }
}

async function generateWordImage(event, wordId, word, btn) {
  event.stopPropagation();
  if (btn._generateAbort) {
    btn._generateAbort.abort();
    return;
  }
  if (btn.classList.contains('btn-generate--busy')) return;
  const row = btn.closest('.word-result-row');
  const abort = new AbortController();
  btn._generateAbort = abort;
  btn.classList.add('btn-generate--busy', 'btn-generate--cancellable');
  btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">finding image\u2026</span><span class="btn-gen-cancel">cancel</span>';
  _pendingGenerates++;
  renderStatus();
  const aiModel = document.getElementById('add-result-model-select').value;
  const meaning = (row?.querySelector('.detail-meaning .detail-input')?.textContent ?? '').trim();
  const prevImageHtml = row?.querySelector('.word-result-image')?.outerHTML ?? null;
  setWordRowImage(row, '', 'loading');
  try {
    const res = await fetch('/api/words/' + wordId + '/find-image', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ word, meaning, ai_model: aiModel, image_source: getImageSource() }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    if (row?.isConnected) {
      setWordRowImage(row, data.image_path, '', Date.now());
      updateWordImagePath(wordId, data.image_path);
    }
  } catch (_) {
    if (row?.isConnected) {
      const imageEl = row.querySelector('.word-result-image');
      if (imageEl && prevImageHtml) imageEl.outerHTML = prevImageHtml;
      else setWordRowImage(row, '', 'failed');
    }
  } finally {
    if (btn._generateAbort === abort) {
      btn._generateAbort = null;
      if (btn.classList.contains('btn-generate--busy')) {
        btn.classList.remove('btn-generate--busy', 'btn-generate--cancellable');
        btn.innerHTML = 'generate';
        _pendingGenerates = Math.max(0, _pendingGenerates - 1);
        renderStatus();
      }
    }
  }
}

async function generateWordAudio(event, wordId, word, btn) {
  event.stopPropagation();
  if (btn._generateAbort) {
    btn._generateAbort.abort();
    return;
  }
  if (btn.classList.contains('btn-generate--busy')) return;
  const abort = new AbortController();
  btn._generateAbort = abort;
  btn.classList.add('btn-generate--busy', 'btn-generate--cancellable');
  btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">generating\u2026</span><span class="btn-gen-cancel">cancel</span>';
  _pendingGenerates++;
  renderStatus();
  try {
    const vv = getVoicevoxSettings();
    const res = await fetch('/api/words/' + wordId + '/generate-audio', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ word, speaker: vv.speaker, speedScale: vv.speedScale, intonationScale: vv.intonationScale }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    updateWordAudioFlags(wordId, data.hasWordAudio, data.hasSentenceAudio);
    const row = btn.closest('.word-result-row');
    if (row) {
      if (data.hasWordAudio) row._hasWordAudio = true;
      if (data.hasSentenceAudio) row._hasSentenceAudio = true;
    }
  } finally {
    if (btn._generateAbort === abort) {
      btn._generateAbort = null;
      if (btn.classList.contains('btn-generate--busy')) {
        btn.classList.remove('btn-generate--busy', 'btn-generate--cancellable');
        btn.innerHTML = 'generate';
        _pendingGenerates = Math.max(0, _pendingGenerates - 1);
        renderStatus();
      }
    }
  }
}

function removeWordRow(event, btn) {
  const word = btn.dataset.word;
  event.stopPropagation();
  openRemoveConfirm('Remove "' + word + '" from the lexicon?', async () => {
    btn.disabled = true;
    const res = await fetch('/admin/words/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ words: [word] }),
    });
    if (!res.ok) { btn.disabled = false; return; }
    const row = btn.closest('.word-result-row');
    row.remove();
    const idx = _addedWords.indexOf(word);
    if (idx !== -1) _addedWords.splice(idx, 1);
    if (document.querySelectorAll('#add-result-modal-body .word-result-row').length === 0) {
      closeAddResultModal();
      return;
    }
    renderStatus();
    updateAddResultFooter();
  });
}

function saveWordRowEdits(row) {
  if (!row._wordId) return;
  const reading   = (row.querySelector('.detail-reading .detail-input')?.textContent ?? '').trim();
  const type      = row.querySelector('.detail-pos-select')?.value ?? '';
  const meaning   = (row.querySelector('.detail-meaning .detail-input')?.textContent ?? '').trim();
  const exInputs  = row.querySelectorAll('.detail-ex .detail-input');
  const exampleJp = (exInputs[0]?.textContent ?? '').trim();
  const exampleEn = (exInputs[1]?.textContent ?? '').trim();
  const targetEl  = row.querySelector('.drill-target-val');
  const target    = targetEl ? (parseInt(targetEl.dataset.target, 10) || 0) : 0;
  const kanjiData = Array.from(row.querySelectorAll('.kanji-reading-input')).map(el => ({
    id: parseInt(el.dataset.kanjiId, 10),
    reading: el.textContent.trim(),
  }));
  fetch('/api/words/' + row._wordId, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ reading, type, meaning, exampleJp, exampleEn, target, kanjiData }),
  });
}

async function adjustWordTarget(event, wordId, delta, btn) {
  event.stopPropagation();
  const stepper = btn.closest('.target-stepper');
  const valEl = stepper.querySelector('.drill-target-val');
  const drillRow = btn.closest('.word-result-drill');

  const currentTarget = parseInt(valEl.dataset.target, 10);
  const correctMatch = drillRow.querySelector('.drill-correct').textContent.match(/\d+/);
  const correct = correctMatch ? parseInt(correctMatch[0], 10) : 0;
  const newTarget = Math.max(correct, currentTarget + delta);
  if (newTarget === currentTarget) return;

  btn.disabled = true;
  try {
    const res = await fetch('/api/words/' + wordId + '/target', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target: newTarget }),
    });
    if (!res.ok) throw new Error(await res.text());
    valEl.dataset.target = newTarget;
    valEl.textContent = newTarget;
  } finally {
    btn.disabled = false;
  }
}


function clearAutofillSpinners() {
  document.querySelectorAll('#add-result-modal-body .btn-generate--busy').forEach(btn => {
    btn._generateAbort = null;
    btn.classList.remove('btn-generate--busy', 'btn-generate--cancellable');
    btn.innerHTML = 'generate';
  });
  _pendingGenerates = 0;
}

function cancelAllGenerates() {
  document.querySelectorAll('#add-result-modal-body .btn-generate--cancellable').forEach(btn => {
    if (btn._generateAbort) btn._generateAbort.abort();
  });
  clearAutofillSpinners();
  renderStatus();
}

function generateAll(includeAdded, includeSkipped) {
  if (includeAdded)
    document.querySelectorAll('#add-result-modal-body .result-added .btn-generate:not(.btn-generate--busy):not([disabled])').forEach(btn => btn.dispatchEvent(new MouseEvent('mousedown')));
  if (includeSkipped)
    document.querySelectorAll('#add-result-modal-body .result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])').forEach(btn => btn.dispatchEvent(new MouseEvent('mousedown')));
}

function renderStatus() {
  // Update modal title
  const titleEl = document.getElementById('add-result-modal-title');
  if (titleEl) {
    titleEl.textContent = 'Edit words';
  }

  // Update header close button state
  const closeBtnHdr = document.getElementById('add-result-modal-close');
  if (closeBtnHdr) {
    const locked = _addPhase === 'loading' || _pendingGenerates > 0;
    closeBtnHdr.style.opacity = locked ? '0.3' : '';
    closeBtnHdr.style.cursor  = locked ? 'not-allowed' : '';
    if (locked) {
      closeBtnHdr.dataset.tooltip = _addPhase === 'loading'
        ? 'Please wait for words to finish being added'
        : 'Please wait for generation to finish';
    } else {
      delete closeBtnHdr.dataset.tooltip;
    }
  }

  const sel = document.getElementById('add-result-model-select');
  if (sel) {
    const busyLock = _pendingGenerates > 0;
    sel.disabled = busyLock || !(_providers && (_providers.anthropic || _providers.openai || _providers.google || _providers.mistral || _providers.glm));
    if (busyLock) {
      sel.dataset.tooltip = 'Unavailable while generation is in progress';
    } else {
      delete sel.dataset.tooltip;
    }
  }
  const el = document.getElementById('add-result-modal-status');
  const actionEl = document.getElementById('add-result-modal-action');
  const skippedHtml = _skippedCount > 0
    ? ', <span class="status-skipped">' + _skippedCount + ' skipped</span>'
    : '';
  const countsHtml = '<span>' + _addedWords.length + ' added' + skippedHtml + '</span>';
  const hasProviders = _providers && (_providers.anthropic || _providers.openai || _providers.google || _providers.mistral || _providers.glm);
  const genType = getGenerateType();
  const audioReady = _voicevoxAvailable && _ffmpegAvailable;
  const genAllTooltip = genType === 'audio'
    ? audioReadyTooltip()
    : genType === 'image'
      ? 'Uses an AI API request to find and download an image for each word'
      : 'Uses an AI API request to get the reading, part-of-speech, meaning, and an example sentence for each word';
  const genAllEnabled =
    document.querySelectorAll('#add-result-modal-body .word-result-row .btn-generate:not(.btn-generate--busy):not([disabled])').length > 0 &&
    (genType === 'audio' ? audioReady : hasProviders) && _addPhase !== 'loading';
  const actionHtml = _pendingGenerates > 0
    ? '<button class="btn-danger btn-generate--cancel">' +
        '<span class="spinner"></span>Cancel generation' +
      '</button>'
    : '<button class="btn-save btn-generate--all"' +
        (genAllEnabled ? '' : ' disabled') +
        ' data-tooltip="' + genAllTooltip + '"' +
        '>Generate' + (_isSingleEdit ? '' : ' all') + '</button>';
  if (actionEl) {
    actionEl.innerHTML = actionHtml;
    const actionBtn = actionEl.querySelector('button');
    if (actionBtn) actionBtn.addEventListener('mousedown', _pendingGenerates > 0 ? cancelAllGenerates : (_isSingleEdit ? () => generateAll(true, true) : openGenerateConfirm));
  }
  if (_addPhase === 'loading') {
    el.className = 'modal-status modal-status-loading';
    el.innerHTML = countsHtml + (_pendingGenerates === 0 ? '<span class="spinner"></span>' : '');
  } else if (_addPhase === 'cancelled') {
    el.className = 'modal-status modal-status-cancelled';
    el.innerHTML = countsHtml + (_pendingGenerates === 0 ? '<span class="status-cancelled-note"> — cancelled</span>' : '');
  } else {
    el.className = 'modal-status ' + (_pendingGenerates > 0 ? 'modal-status-loading' : 'modal-status-done');
    el.innerHTML = countsHtml;
  }
  updateAddResultFooter();
  updateGenerateBtnStates();
  const sourceDisplay = genType === 'image' ? '' : 'none';
  const sourceSel  = document.getElementById('add-result-image-source-select');
  const sourceIcon = document.getElementById('add-result-image-source-icon');
  if (sourceSel)  sourceSel.style.display  = sourceDisplay;
  if (sourceIcon) sourceIcon.style.display = sourceDisplay;
}

function initAddResultFooter() {
  const providerModels = [
    { key: 'anthropic', label: 'Anthropic', envKey: 'ANTHROPIC_API_KEY', models: [
      ['anthropic/claude-haiku-4-5-20251001', 'claude-haiku (fast)'],
      ['anthropic/claude-sonnet-4-6',         'claude-sonnet (better)'],
    ]},
    { key: 'openai',   label: 'OpenAI',   envKey: 'OPENAI_API_KEY',   models: [
      ['openai/gpt-4o-mini', 'gpt-4o-mini (fast)'],
      ['openai/gpt-4o',      'gpt-4o (better)'],
    ]},
    { key: 'google',   label: 'Google',   envKey: 'GOOGLE_API_KEY',   models: [
      ['google/gemini-2.0-flash', 'gemini-2.0-flash (fast)'],
      ['google/gemini-1.5-pro',   'gemini-1.5-pro (better)'],
    ]},
    { key: 'mistral',  label: 'Mistral',  envKey: 'MISTRAL_API_KEY',  models: [
      ['mistral/mistral-small-latest', 'mistral-small (fast)'],
      ['mistral/mistral-large-latest', 'mistral-large (better)'],
    ]},
    { key: 'glm',      label: 'GLM',      envKey: 'GLM_API_KEY',      models: [
      ['glm/glm-4',       'glm-4 (better)'],
      ['glm/glm-3-turbo', 'glm-3-turbo (fast)'],
    ]},
  ];

  const imageSources = [
    { key: 'unsplash', label: 'Unsplash', envKey: 'UNSPLASH_ACCESS_KEY' },
    { key: 'pexels',   label: 'Pexels',   envKey: 'PEXELS_API_KEY'      },
    { key: 'pixabay',  label: 'Pixabay',  envKey: 'PIXABAY_API_KEY'     },
    { key: 'bing',     label: 'Bing',     envKey: 'BING_API_KEY'        },
  ];

  const footer = document.getElementById('add-result-modal-footer');
  const hasProviders = _providers && providerModels.some(p => _providers[p.key]);
  const progTip = _providers
    ? (() => {
        const lines = providerModels
          .filter(p => !_providers[p.key])
          .map(p => p.label + ': set ' + p.envKey + ' to enable');
        return lines.length ? lines.join('\n') + '\n— then restart the program' : null;
      })()
    : null;
  const optgroupsHtml = providerModels.map(({ key, label, models }) => {
    const avail = _providers && _providers[key];
    const groupLabel = avail ? label : label + ' — no API key';
    const options = models.map(([val, text]) => '<option value="' + val + '">' + text + '</option>').join('');
    return '<optgroup label="' + groupLabel + '"' + (avail ? '' : ' disabled') + '>' + options + '</optgroup>';
  }).join('');

  const imageSourceOptions =
    '<option value="wikimedia">Wikimedia</option>' +
    imageSources.map(({ key, label }) => {
      const avail = _imageSources && _imageSources[key];
      return '<option value="' + key + '"' + (avail ? '' : ' disabled') + '>' +
        label + (avail ? '' : ' — no key') + '</option>';
    }).join('');
  const imageSourceTip = _imageSources
    ? (() => {
        const lines = imageSources
          .filter(s => !_imageSources[s.key])
          .map(s => s.label + ': set ' + s.envKey + ' to enable');
        return lines.length ? lines.join('\n') + '\n— then restart the program' : null;
      })()
    : null;

  footer.innerHTML =
    '<select id="add-result-model-select" class="add-result-model-select"' +
      (hasProviders ? '' : ' disabled') +
    '>' +
      (!hasProviders ? '<option value="" selected>no API keys configured</option>' : '') +
      optgroupsHtml +
    '</select>' +
    (progTip ? '<span class="provider-info-icon" data-tooltip="' + progTip + '">?</span>' : '') +
    '<select id="add-result-generate-type" class="add-result-model-select">' +
      '<option value="word-info">word info</option>' +
      '<option value="image">image</option>' +
      '<option value="audio">audio</option>' +
    '</select>' +
    '<select id="add-result-image-source-select" class="add-result-model-select" style="display:none">' +
      imageSourceOptions +
    '</select>' +
    (imageSourceTip ? '<span id="add-result-image-source-icon" class="provider-info-icon" style="display:none" data-tooltip="' + imageSourceTip + '">?</span>' : '') +
    '<div id="add-result-modal-action" style="margin-left:0.4rem"></div>' +
    '<div id="add-result-modal-status" class="modal-status" style="padding:0;border:none;margin-left:auto"></div>' +
    '<button id="btn-add-result-remove" class="btn-danger">Remove the added words</button>' +
    '<button id="btn-add-result-close" class="btn-save">Close</button>';

  if (hasProviders) {
    const sel = document.getElementById('add-result-model-select');
    const first = sel.querySelector('optgroup:not([disabled]) option');
    if (first) sel.value = first.value;
  }

  document.getElementById('add-result-generate-type').addEventListener('change', () => renderStatus());

  document.getElementById('btn-add-result-remove').onclick = function () {
    const count = _addedWords.length;
    const label = count === 1 ? '"' + _addedWords[0] + '"' : count + ' added words';
    openRemoveConfirm('Remove ' + label + ' from the lexicon?', async () => {
      const toRemove = _addedWords.slice();
      if (_addPhase === 'loading') {
        _addPhase = 'done'; // mark before abort so the AbortError catch is a no-op
        _abortController.abort();
      }
      await fetch('/admin/words/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ words: toRemove }),
      });
      _addedWords = [];
      document.querySelectorAll('#add-result-modal-body .badge-added').forEach(badge => {
        badge.closest('.word-result-row').remove();
      });
      if (document.querySelectorAll('#add-result-modal-body .word-result-row').length === 0) {
        closeAddResultModal();
        await reloadWords();
        return;
      }
      renderStatus();
      await reloadWords();
      updateAddResultFooter();
    });
  };
  document.getElementById('btn-add-result-close').onclick = closeAddResultModal;
  updateAddResultFooter();
}

function updateAddResultFooter() {
  const btnRemove = document.getElementById('btn-add-result-remove');
  const btnClose  = document.getElementById('btn-add-result-close');
  if (!btnRemove) return;
  btnRemove.disabled = _addedWords.length === 0;
  btnRemove.textContent = 'Remove the added words';
  btnClose.disabled = _addPhase === 'loading' || _pendingGenerates > 0;
}
