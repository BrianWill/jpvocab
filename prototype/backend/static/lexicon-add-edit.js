import { state as lexiconState, typeLabels, reloadWords, renderTable, getSortedWords, closeAddModal, updateWordImagePath, updateWordAudioFlags } from './lexicon.js';
import { esc } from './lexicon-utils.js';
import { getVoicevoxSettings, playWordAudio, playSentenceAudio, playDing, PROVIDER_MODELS } from './common.js';
import {
  adjustWordTarget,
  bindWordResultImageUpload,
  bindWordResultEditorEvents,
  buildWordResultDetails,
  buildWordResultImage,
  getAudioGenerationTooltip,
  getWordBtnLabel,
  saveWordRowEdits,
  setWordRowImage,
  sortAddResultRows,
} from './add-to-lexicon.js';

const els = {
  addModalSaveBtn: document.querySelector('#add-modal-backdrop .btn-save'),
  addResultBody: document.getElementById('add-result-modal-body'),
  addResultClose: document.getElementById('add-result-modal-close'),
  addResultFooter: document.getElementById('add-result-modal-footer'),
  addResultModalBackdrop: document.getElementById('add-result-modal-backdrop'),
  addResultTitle: document.getElementById('add-result-modal-title'),
  generateConfirmAddedCheckbox: document.getElementById('generate-confirm-added-checkbox'),
  generateConfirmAddedText: document.getElementById('generate-confirm-added-text'),
  generateConfirmCancel: document.getElementById('generate-confirm-cancel'),
  generateConfirmModalBackdrop: document.getElementById('generate-confirm-modal-backdrop'),
  generateConfirmOk: document.getElementById('generate-confirm-ok'),
  generateConfirmSkippedCheckbox: document.getElementById('generate-confirm-skipped-checkbox'),
  generateConfirmSkippedText: document.getElementById('generate-confirm-skipped-text'),
  removeConfirmCancel: document.getElementById('remove-confirm-cancel'),
  removeConfirmModalBackdrop: document.getElementById('remove-confirm-modal-backdrop'),
  removeConfirmOk: document.getElementById('remove-confirm-ok'),
  removeConfirmText: document.getElementById('remove-confirm-text'),
};

export const state = {
  abortController: null,
  addedWords: [],
  addPhase: 'idle', // 'loading' | 'done' | 'cancelled'
  fieldErrorTimer: null,
  generateType: 'word-info',
  generationCancelled: false,
  isSingleEdit: false,
  pendingGenerates: 0,
  prevPendingGenerates: 0,
  pendingRemoveAction: null,
  skippedCount: 0,
};

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

  state.addPhase = 'done';
  state.isSingleEdit = true;
  state.addedWords = [];
  state.skippedCount = 0;
  state.pendingGenerates = 0;
  state.abortController = null;

  els.addResultBody.innerHTML = '';

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

  els.addResultModalBackdrop.classList.remove('hidden');
  initAddResultFooter();
  document.getElementById('btn-add-result-remove').style.display = 'none';
  renderStatus();
  els.addResultBody.querySelector('.result-badge').style.display = 'none';
}

document.addEventListener('mousedown', () => {
  const menu = document.getElementById('split-btn-menu');
  if (menu) menu.hidden = true;
});

els.addResultModalBackdrop.addEventListener('click', function (e) {
  if (e.target === this && state.addPhase !== 'loading' && state.pendingGenerates === 0) closeAddResultModal();
});
els.addResultClose.addEventListener('click', closeAddResultModal);

els.addResultBody.addEventListener('click', e => {
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
function openRemoveConfirm(message, action) {
  state.pendingRemoveAction = action;
  els.removeConfirmText.textContent = message;
  els.removeConfirmModalBackdrop.classList.remove('hidden');
}
function closeRemoveConfirm() {
  state.pendingRemoveAction = null;
  els.removeConfirmModalBackdrop.classList.add('hidden');
}
els.removeConfirmModalBackdrop.addEventListener('click', e => {
  if (e.target === els.removeConfirmModalBackdrop) closeRemoveConfirm();
});
els.removeConfirmCancel.addEventListener('click', closeRemoveConfirm);
els.removeConfirmOk.addEventListener('click', () => {
  const action = state.pendingRemoveAction;
  closeRemoveConfirm();
  if (action) action();
});

// --- Generate-confirm mini-modal ---
function openGenerateConfirm() {
  const addedCount   = els.addResultBody.querySelectorAll('.result-added .btn-generate:not(.btn-generate--busy):not([disabled])').length;
  const skippedCount = els.addResultBody.querySelectorAll('.result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])').length;

  els.generateConfirmAddedText.textContent   = addedCount   + ' newly added words';
  els.generateConfirmSkippedText.textContent = skippedCount + ' already existing words';
  els.generateConfirmAddedCheckbox.checked   = true;
  els.generateConfirmSkippedCheckbox.checked = false;

  els.generateConfirmModalBackdrop.classList.remove('hidden');
}
function closeGenerateConfirm() {
  els.generateConfirmModalBackdrop.classList.add('hidden');
}
els.generateConfirmModalBackdrop.addEventListener('click', e => {
  if (e.target === els.generateConfirmModalBackdrop) closeGenerateConfirm();
});
els.generateConfirmCancel.addEventListener('click', closeGenerateConfirm);
els.generateConfirmOk.addEventListener('click', () => {
  const includeAdded   = els.generateConfirmAddedCheckbox.checked;
  const includeSkipped = els.generateConfirmSkippedCheckbox.checked;
  closeGenerateConfirm();
  generateAll(includeAdded, includeSkipped);
});

els.addModalSaveBtn.addEventListener('click', saveAddModal);

bindWordResultEditorEvents({
  containerEl: els.addResultBody,
  footerEl: els.addResultFooter,
  closeButtonId: 'btn-add-result-close',
  state,
  onSaveRowEdits: saveWordRowEdits,
});

bindWordResultImageUpload({
  containerEl: els.addResultBody,
  onUploadComplete: (wordId, imagePath) => updateWordImagePath(wordId, imagePath),
});

export async function closeAddResultModal() {
  if (state.addPhase === 'loading' || state.pendingGenerates > 0) return;
  els.addResultModalBackdrop.classList.add('hidden');
  await reloadWords();
  const activeBtn = document.querySelector('.btn-sort--active');
  renderTable(getSortedWords(activeBtn.dataset.sort, activeBtn.dataset.dir || 'desc'));
}

async function saveAddModal() {
  function setModalStatus(type, text) {
    const el = document.getElementById('add-result-modal-status');
    const spinner = type === 'loading' ? '<span class="spinner"></span>' : '';
    el.className = 'modal-status modal-status-' + type;
    el.innerHTML = spinner + '<span>' + esc(text) + '</span>';
  }

  const rawWords = document.getElementById('add-words-input').value.trim();
  if (!rawWords) return;

  closeAddModal();

  state.addPhase = 'loading';
  state.isSingleEdit = false;
  state.addedWords = [];
  state.skippedCount = 0;
  state.pendingGenerates = 0;
  state.abortController = new AbortController();

  els.addResultBody.innerHTML = '';
  els.addResultModalBackdrop.classList.remove('hidden');
  initAddResultFooter();
  document.getElementById('add-result-modal-status').style.display = '';
  renderStatus();

  const form = new FormData();
  form.append('words', rawWords);
  form.append('autofill', 'off');

  try {
    const res = await fetch('/admin/words/batch', {
      method: 'POST', body: form, signal: state.abortController.signal,
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
          state.addPhase = 'done';
          clearAutofillSpinners();
          sortAddResultRows(els.addResultBody);
          renderStatus();
          await reloadWords();
          updateAddResultFooter();
          return;
        }
        if (data.added) state.addedWords.push(data.word);
        else state.skippedCount++;
        appendWordRow(data);
        renderStatus();
        updateAddResultFooter();
      }
    }
  } catch (err) {
    if (err.name === 'AbortError') {
      if (state.addPhase === 'loading') {
        // Abort came from Cancel button — handle as cancellation.
        state.addPhase = 'cancelled';
        clearAutofillSpinners();
        renderStatus();
        await reloadWords();
        updateAddResultFooter();
      }
      // else: abort was triggered by the Remove handler, which manages cleanup itself.
    } else {
      state.addPhase = 'done';
      setModalStatus('done', 'Error: ' + err.message);
      await reloadWords();
      updateAddResultFooter();
    }
  }
}

function appendWordRow(data) {
  // Find the pre-inserted placeholder row for this word; fall back to appending a new one
  let row = null;
  for (const el of els.addResultBody.children) {
    if (el._pendingWord === data.word) { row = el; break; }
  }
  if (!row) {
    row = document.createElement('div');
    els.addResultBody.appendChild(row);
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
  const hasProviders = lexiconState.providers && (lexiconState.providers.anthropic || lexiconState.providers.openai || lexiconState.providers.google || lexiconState.providers.mistral || lexiconState.providers.glm);
  const generateBtn = data.word_id
    ? '<button class="btn-generate"' +
        (hasProviders ? '' : ' disabled') +
        ' data-tooltip="Uses an AI API request to get the word\'s reading, part-of-speech, meaning, and an example sentence"' +
        '>' + getWordBtnLabel(state.generateType) + '</button>'
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

  const details = buildWordResultDetails(data.word, data, typeLabels);

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
  let row = null;
  for (const el of els.addResultBody.children) {
    if (el._resolvedWord === data.word) { row = el; break; }
  }
  if (!row) return;
  const newDetails = buildWordResultDetails(row._resolvedWord, data, typeLabels);
  row.querySelector('.word-result-details').outerHTML = newDetails;
  const genBtn = row.querySelector('.btn-generate');
  if (genBtn && genBtn.classList.contains('btn-generate--busy') && !genBtn._generateAbort) {
    genBtn.classList.remove('btn-generate--busy');
    genBtn.innerHTML = getWordBtnLabel(state.generateType);
    state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
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
  return state.generateType;
}

function getImageSource() {
  return document.getElementById('add-result-image-source-select')?.value ?? 'wikimedia';
}

function updateGenerateBtnStates() {
  const type = getGenerateType();
  const hasProviders = lexiconState.providers && (lexiconState.providers.anthropic || lexiconState.providers.openai || lexiconState.providers.google || lexiconState.providers.mistral || lexiconState.providers.glm);
  const audioReady = lexiconState.voicevoxAvailable && lexiconState.ffmpegAvailable;
  const disabled = type === 'audio' ? !audioReady : !hasProviders;
  const tooltip = type === 'audio'
    ? getAudioGenerationTooltip({
        voicevoxAvailable: lexiconState.voicevoxAvailable,
        ffmpegAvailable: lexiconState.ffmpegAvailable,
        readyMessage: 'Generates audio via the local VoiceVox engine',
      })
    : type === 'image'
      ? 'Uses an AI API request to find and download an image for this word'
      : 'Uses an AI API request to get the word\'s reading, part-of-speech, meaning, and an example sentence';
  els.addResultBody.querySelectorAll('.btn-generate:not(.btn-generate--busy)').forEach(btn => {
    btn.disabled = disabled;
    btn.dataset.tooltip = tooltip;
    btn.innerHTML = getWordBtnLabel(state.generateType);
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
  state.pendingGenerates++;
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
        btn.innerHTML = getWordBtnLabel(state.generateType);
        state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
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
  state.pendingGenerates++;
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
        btn.innerHTML = getWordBtnLabel(state.generateType);
        state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
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
  state.pendingGenerates++;
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
        btn.innerHTML = getWordBtnLabel(state.generateType);
        state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
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
    const idx = state.addedWords.indexOf(word);
    if (idx !== -1) state.addedWords.splice(idx, 1);
    if (els.addResultBody.querySelectorAll('.word-result-row').length === 0) {
      closeAddResultModal();
      return;
    }
    renderStatus();
    updateAddResultFooter();
  });
}

function clearAutofillSpinners() {
  els.addResultBody.querySelectorAll('.btn-generate--busy').forEach(btn => {
    btn._generateAbort = null;
    btn.classList.remove('btn-generate--busy', 'btn-generate--cancellable');
    btn.innerHTML = getWordBtnLabel(state.generateType);
  });
  state.pendingGenerates = 0;
}

function cancelAllGenerates() {
  state.generationCancelled = true;
  els.addResultBody.querySelectorAll('.btn-generate--cancellable').forEach(btn => {
    if (btn._generateAbort) btn._generateAbort.abort();
  });
  clearAutofillSpinners();
  renderStatus();
}

function generateAll(includeAdded, includeSkipped) {
  const type = getGenerateType();
  if (type === 'word-info') {
    const rows = [];
    if (includeAdded)
      els.addResultBody.querySelectorAll('.result-added .btn-generate:not(.btn-generate--busy):not([disabled])').forEach(btn => {
        const row = btn.closest('.word-result-row');
        if (row) rows.push(row);
      });
    if (includeSkipped)
      els.addResultBody.querySelectorAll('.result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])').forEach(btn => {
        const row = btn.closest('.word-result-row');
        if (row) rows.push(row);
      });
    generateAllAutofillBatch(rows);
  } else {
    if (includeAdded)
      els.addResultBody.querySelectorAll('.result-added .btn-generate:not(.btn-generate--busy):not([disabled])').forEach(btn => btn.dispatchEvent(new MouseEvent('mousedown')));
    if (includeSkipped)
      els.addResultBody.querySelectorAll('.result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])').forEach(btn => btn.dispatchEvent(new MouseEvent('mousedown')));
  }
}

async function generateAllAutofillBatch(rows) {
  const abort = new AbortController();
  const aiModel = document.getElementById('add-result-model-select').value;
  const wordItems = [];
  for (const row of rows) {
    if (!row._wordId) continue;
    const btn = row.querySelector('.btn-generate:not(.btn-generate--busy):not([disabled])');
    if (!btn) continue;
    btn._generateAbort = abort;
    btn.classList.add('btn-generate--busy', 'btn-generate--cancellable');
    btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">generating\u2026</span><span class="btn-gen-cancel">cancel generation</span>';
    state.pendingGenerates++;
    wordItems.push({ id: row._wordId, word: row._resolvedWord, btn });
  }
  if (wordItems.length === 0) return;
  renderStatus();
  try {
    const res = await fetch('/api/words/autofill-batch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ words: wordItems.map(w => ({ id: w.id, word: w.word })), ai_model: aiModel }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    const results = await res.json();
    // Null out _generateAbort on all batch buttons so updateWordRowDetails can clean them up
    for (const item of wordItems) {
      if (item.btn._generateAbort === abort) item.btn._generateAbort = null;
    }
    for (const result of results) {
      if (!result.error) updateWordRowDetails(result);
    }
  } finally {
    // Clean up any buttons still busy (errors, aborted, or missing from results).
    // Always remove btn-generate--cancellable: updateWordRowDetails only strips
    // btn-generate--busy, leaving the red cancellable style if we don't clear it here.
    for (const { btn } of wordItems) {
      btn._generateAbort = null;
      btn.classList.remove('btn-generate--cancellable');
      if (btn.classList.contains('btn-generate--busy')) {
        btn.classList.remove('btn-generate--busy');
        btn.innerHTML = getWordBtnLabel(state.generateType);
        state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
      }
    }
    renderStatus();
  }
}

function renderStatus() {
  if (state.pendingGenerates > 0 && state.prevPendingGenerates === 0) state.generationCancelled = false;
  if (state.prevPendingGenerates > 0 && state.pendingGenerates === 0 && !state.generationCancelled) playDing();
  state.prevPendingGenerates = state.pendingGenerates;

  els.addResultTitle.textContent = 'Edit words';

  const locked = state.addPhase === 'loading' || state.pendingGenerates > 0;
  els.addResultClose.style.opacity = locked ? '0.3' : '';
  els.addResultClose.style.cursor  = locked ? 'not-allowed' : '';
  if (locked) {
    els.addResultClose.dataset.tooltip = state.addPhase === 'loading'
      ? 'Please wait for words to finish being added'
      : 'Please wait for generation to finish';
  } else {
    delete els.addResultClose.dataset.tooltip;
  }

  const sel = document.getElementById('add-result-model-select');
  if (sel) {
    const busyLock = state.pendingGenerates > 0;
    sel.disabled = busyLock || !(lexiconState.providers && (lexiconState.providers.anthropic || lexiconState.providers.openai || lexiconState.providers.google || lexiconState.providers.mistral || lexiconState.providers.glm));
    if (busyLock) {
      sel.dataset.tooltip = 'Unavailable while generation is in progress';
    } else {
      delete sel.dataset.tooltip;
    }
  }
  const el = document.getElementById('add-result-modal-status');
  const actionEl = document.getElementById('add-result-modal-action');
  const skippedHtml = state.skippedCount > 0
    ? '<span class="status-skipped">' + state.skippedCount + ' skipped</span>'
    : '';
  const countsHtml =
    '<span class="modal-status-counts">' +
      '<span>' + state.addedWords.length + ' added</span>' +
      skippedHtml +
    '</span>';
  const hasProviders = lexiconState.providers && (lexiconState.providers.anthropic || lexiconState.providers.openai || lexiconState.providers.google || lexiconState.providers.mistral || lexiconState.providers.glm);
  const genType = getGenerateType();
  const audioReady = lexiconState.voicevoxAvailable && lexiconState.ffmpegAvailable;
  const genAllTooltip = genType === 'audio'
    ? getAudioGenerationTooltip({
        voicevoxAvailable: lexiconState.voicevoxAvailable,
        ffmpegAvailable: lexiconState.ffmpegAvailable,
        readyMessage: 'Generates audio via the local VoiceVox engine',
      })
    : genType === 'image'
      ? 'Uses an AI API request to find and download an image for each word'
      : 'Uses an AI API request to get the reading, part-of-speech, meaning, and an example sentence for each word';
  const genAllEnabled =
    els.addResultBody.querySelectorAll('.word-result-row .btn-generate:not(.btn-generate--busy):not([disabled])').length > 0 &&
    (genType === 'audio' ? audioReady : hasProviders) && state.addPhase !== 'loading';
  const genTypeLabels = { 'word-info': 'Generate word info', 'image': 'Generate images', 'audio': 'Generate audio' };
  const actionHtml = state.pendingGenerates > 0
    ? '<button class="btn-danger btn-generate--cancel">' +
        '<span class="spinner"></span>Cancel generation' +
      '</button>'
    : '<div class="split-btn-wrap">' +
        '<button class="btn-save btn-generate--all split-btn-main"' +
          (genAllEnabled ? '' : ' disabled') +
          ' data-tooltip="' + genAllTooltip + '">' +
          genTypeLabels[state.generateType] +
        '</button>' +
        '<button class="btn-save btn-generate--all split-btn-arrow"' +
          (genAllEnabled ? '' : ' disabled') +
          '>▾</button>' +
        '<div class="split-btn-menu" id="split-btn-menu" hidden>' +
          ['word-info', 'image', 'audio'].map(t =>
            '<button class="split-btn-option' + (t === state.generateType ? ' split-btn-option--active' : '') + '" data-type="' + t + '">' +
              genTypeLabels[t] +
            '</button>'
          ).join('') +
        '</div>' +
      '</div>' +
      (state.generateType === 'audio' ? '<span class="provider-info-icon" data-tooltip="VoiceVox must be running on this &#10;machine at http://localhost:50021&#10;&#10;Download: https://voicevox.hiroshiba.jp/">?</span>' : '');
  if (actionEl) {
    actionEl.innerHTML = actionHtml;
    if (state.pendingGenerates > 0) {
      actionEl.querySelector('button').addEventListener('mousedown', cancelAllGenerates);
    } else {
      const mainBtn = actionEl.querySelector('.split-btn-main');
      const arrowBtn = actionEl.querySelector('.split-btn-arrow');
      const menu = document.getElementById('split-btn-menu');
      if (mainBtn) mainBtn.addEventListener('mousedown', state.isSingleEdit ? () => generateAll(true, true) : openGenerateConfirm);
      if (arrowBtn && menu) {
        arrowBtn.addEventListener('mousedown', (e) => {
          e.stopPropagation();
          menu.hidden = !menu.hidden;
        });
        menu.querySelectorAll('.split-btn-option').forEach(opt => {
          opt.addEventListener('mousedown', (e) => {
            e.stopPropagation();
            state.generateType = opt.dataset.type;
            menu.hidden = true;
            renderStatus();
          });
        });
      }
    }
  }
  if (state.addPhase === 'loading') {
    el.className = 'modal-status modal-status-loading';
    el.innerHTML = countsHtml + (state.pendingGenerates === 0 ? '<span class="spinner"></span>' : '');
  } else if (state.addPhase === 'cancelled') {
    el.className = 'modal-status modal-status-cancelled';
    el.innerHTML = countsHtml + (state.pendingGenerates === 0 ? '<span class="status-cancelled-note"> — cancelled</span>' : '');
  } else {
    el.className = 'modal-status ' + (state.pendingGenerates > 0 ? 'modal-status-loading' : 'modal-status-done');
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
  const imageSources = [
    { key: 'unsplash', label: 'Unsplash', envKey: 'UNSPLASH_ACCESS_KEY' },
    { key: 'pexels',   label: 'Pexels',   envKey: 'PEXELS_API_KEY'      },
    { key: 'pixabay',  label: 'Pixabay',  envKey: 'PIXABAY_API_KEY'     },
    { key: 'bing',     label: 'Bing',     envKey: 'BING_API_KEY'        },
  ];

  const hasProviders = lexiconState.providers && PROVIDER_MODELS.some(p => lexiconState.providers[p.key]);
  const progTip = lexiconState.providers
    ? (() => {
        const lines = PROVIDER_MODELS
          .filter(p => !lexiconState.providers[p.key])
          .map(p => p.label + ': set ' + p.envKey + ' to enable');
        return lines.length ? lines.join('\n') + '\n— then restart the program' : null;
      })()
    : null;
  const optgroupsHtml = PROVIDER_MODELS.map(({ key, label, models }) => {
    const avail = lexiconState.providers && lexiconState.providers[key];
    const groupLabel = avail ? label : label + ' — no API key';
    const options = models.map(([val, text]) => '<option value="' + val + '">' + text + '</option>').join('');
    return '<optgroup label="' + groupLabel + '"' + (avail ? '' : ' disabled') + '>' + options + '</optgroup>';
  }).join('');

  const imageSourceOptions =
    '<option value="wikimedia">Wikimedia</option>' +
    imageSources.map(({ key, label }) => {
      const avail = lexiconState.imageSources && lexiconState.imageSources[key];
      return '<option value="' + key + '"' + (avail ? '' : ' disabled') + '>' +
        label + (avail ? '' : ' — no key') + '</option>';
    }).join('');
  const imageSourceTip = lexiconState.imageSources
    ? (() => {
        const lines = imageSources
          .filter(s => !lexiconState.imageSources[s.key])
          .map(s => s.label + ': set ' + s.envKey + ' to enable');
        return lines.length ? lines.join('\n') + '\n— then restart the program' : null;
      })()
    : null;

  els.addResultFooter.innerHTML =
    '<select id="add-result-model-select" class="add-result-model-select"' +
      (hasProviders ? '' : ' disabled') +
    '>' +
      (!hasProviders ? '<option value="" selected>no API keys configured</option>' : '') +
      optgroupsHtml +
    '</select>' +
    (progTip ? '<span class="provider-info-icon" data-tooltip="' + progTip + '">?</span>' : '') +
    '<div id="add-result-modal-action" style="margin-left:0.4rem;display:flex;align-items:center;gap:0.4rem"></div>' +
    '<select id="add-result-image-source-select" class="add-result-model-select" style="display:none">' +
      imageSourceOptions +
    '</select>' +
    (imageSourceTip ? '<span id="add-result-image-source-icon" class="provider-info-icon" style="display:none" data-tooltip="' + imageSourceTip + '">?</span>' : '') +
    '<div id="add-result-modal-status" class="modal-status" style="padding:0;border:none;margin-left:auto"></div>' +
    '<button id="btn-add-result-remove" class="btn-danger">Remove the added words</button>' +
    '<button id="btn-add-result-close" class="btn-save">Close</button>';

  if (hasProviders) {
    const sel = document.getElementById('add-result-model-select');
    const first = sel.querySelector('optgroup:not([disabled]) option');
    if (first) sel.value = first.value;
  }

  document.getElementById('btn-add-result-remove').addEventListener('click', function () {
    const count = state.addedWords.length;
    const label = count === 1 ? '"' + state.addedWords[0] + '"' : count + ' added words';
    openRemoveConfirm('Remove ' + label + ' from the lexicon?', async () => {
      const toRemove = state.addedWords.slice();
      if (state.addPhase === 'loading') {
        state.addPhase = 'done'; // mark before abort so the AbortError catch is a no-op
        state.abortController.abort();
      }
      await fetch('/admin/words/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ words: toRemove }),
      });
      state.addedWords = [];
      els.addResultBody.querySelectorAll('.badge-added').forEach(badge => {
        badge.closest('.word-result-row').remove();
      });
      if (els.addResultBody.querySelectorAll('.word-result-row').length === 0) {
        closeAddResultModal();
        await reloadWords();
        return;
      }
      renderStatus();
      await reloadWords();
      updateAddResultFooter();
    });
  });
  document.getElementById('btn-add-result-close').addEventListener('click', closeAddResultModal);
  updateAddResultFooter();
}

function updateAddResultFooter() {
  const btnRemove = document.getElementById('btn-add-result-remove');
  const btnClose  = document.getElementById('btn-add-result-close');
  if (!btnRemove) return;
  btnRemove.disabled = state.addedWords.length === 0;
  btnRemove.textContent = 'Remove the added words';
  btnClose.disabled = state.addPhase === 'loading' || state.pendingGenerates > 0;
}
