import { state as lexiconState, typeLabels, reloadWords, renderTable, closeAddModal, updateWordImagePath } from './lexicon.js';
import { esc } from './lexicon-utils.js';
import { playWordAudio, playSentenceAudio, playDing, PROVIDER_MODELS } from './common.js';
import {
  applyWordRowDetailsUpdate,
  generateAllAutofillBatchRequest,
  generateWordAutofillRequest,
  generateWordImageRequest,
  streamBatchAdd,
} from './lexicon-add-modal-utils.js';
import {
  adjustWordTarget,
  bindWordResultImageUpload,
  bindWordResultEditorEvents,
  buildWordResultDetails,
  buildWordResultImage,
  getWordBtnLabel,
  saveWordRowEdits,
  setWordRowImage,
  sortAddResultRows,
} from './lexicon-add-modal.js';
import {
  buildImageSourceOptionsHtml,
  buildProviderOptionsHtml,
  hasAvailableProvider,
  imageSourceUnavailableTooltip,
  providerUnavailableTooltip,
} from './generation-ui-utils.js';
import { createWordResultModalController } from './word-result-modal-controller.js';
import {
  applyGenerateButtonState,
  buildCountsHtml,
  buildWordResultActionHtml,
  buildWordResultFooterHtml,
  buildWordResultRowMarkup,
} from './word-result-modal-ui.js';

const LEXICON_AUDIO_OPTIONS = { preferSynthesis: true, fallbackToBrowserTts: true };

const els = {
  addModalSaveBtn: document.querySelector('#add-modal-backdrop .btn-save'),
  addWordsInput: document.getElementById('add-words-input'),
  addResultAction: null,
  addResultBody: document.getElementById('add-result-modal-body'),
  addResultClose: document.getElementById('add-result-modal-close'),
  addResultCloseBtn: null,
  addResultFooter: document.getElementById('add-result-modal-footer'),
  addResultImageSourceIcon: null,
  addResultImageSourceSelect: null,
  addResultModelSelect: null,
  addResultModalBackdrop: document.getElementById('add-result-modal-backdrop'),
  addResultRemoveBtn: null,
  addResultStatus: null,
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
  splitBtnMenu: null,
};

export const state = {
  abortController: null,
  addedWords: [],
  addPhase: 'idle', // 'loading' | 'done' | 'cancelled'
  eventsBound: false,
  fieldErrorTimer: null,
  generateType: 'word-info',
  generationCancelled: false,
  isSingleEdit: false,
  pendingGenerates: 0,
  prevPendingGenerates: 0,
  pendingRemoveAction: null,
  skippedCount: 0,
};

function cacheAddResultFooterEls() {
  els.addResultAction = document.getElementById('add-result-modal-action');
  els.addResultCloseBtn = document.getElementById('btn-add-result-close');
  els.addResultImageSourceIcon = document.getElementById('add-result-image-source-icon');
  els.addResultImageSourceSelect = document.getElementById('add-result-image-source-select');
  els.addResultModelSelect = document.getElementById('add-result-model-select');
  els.addResultRemoveBtn = document.getElementById('btn-add-result-remove');
  els.addResultStatus = document.getElementById('add-result-modal-status');
  els.splitBtnMenu = document.getElementById('split-btn-menu');
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
  const editBtn = event.target.closest('.btn-edit');
  const rowEl = event.target.closest('[data-word-id]');
  const wordId = parseInt(editBtn?.dataset.wordId || rowEl?.dataset.wordId || '', 10);
  const w = lexiconState.words.find(word => word.id === wordId) || rowEl?._word;
  if (!w) return;

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
  });

  els.addResultModalBackdrop.classList.remove('hidden');
  initAddResultFooter();
  els.addResultRemoveBtn.style.display = 'none';
  renderStatus();
  els.addResultBody.querySelector('.result-badge').style.display = 'none';
}

document.addEventListener('mousedown', () => {
  if (els.splitBtnMenu) els.splitBtnMenu.hidden = true;
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
    exampleJp: text,
  }, 1, LEXICON_AUDIO_OPTIONS);
});

els.addModalSaveBtn.addEventListener('click', saveAddModal);
const modalController = createWordResultModalController({
  els,
  state,
  closeModal: closeAddResultModal,
  canClose: () => state.addPhase !== 'loading' && state.pendingGenerates === 0,
  closeButtonId: 'btn-add-result-close',
  onGenerateAll: generateAll,
  onPlayExampleSentence: ({ row, text }) => {
    playSentenceAudio({
      word: row?._resolvedWord ?? '',
      exampleJp: text,
    }, 1, LEXICON_AUDIO_OPTIONS);
  },
  onUploadComplete: (wordId, imagePath) => updateWordImagePath(wordId, imagePath),
  onSaveRowEdits: saveWordRowEdits,
  bindWordResultEditorEvents,
  bindWordResultImageUpload,
});
modalController.bindBaseEvents();

export async function closeAddResultModal() {
  if (state.addPhase === 'loading' || state.pendingGenerates > 0) return;
  els.addResultModalBackdrop.classList.add('hidden');
  await reloadWords();
  renderTable();
}

async function saveAddModal() {
  function setModalStatus(type, text) {
    const el = els.addResultStatus;
    const spinner = type === 'loading' ? '<span class="spinner"></span>' : '';
    el.className = 'modal-status modal-status-' + type;
    el.innerHTML = spinner + '<span>' + esc(text) + '</span>';
  }

  const rawWords = els.addWordsInput.value.trim();
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
  els.addResultStatus.style.display = '';
  renderStatus();

  try {
    await streamBatchAdd({
      rawWords,
      signal: state.abortController.signal,
      onUpdated: data => {
        updateWordRowDetails(data);
      },
      onDone: async () => {
        state.addPhase = 'done';
        clearAutofillSpinners();
        sortAddResultRows(els.addResultBody);
        renderStatus();
        await reloadWords();
        updateAddResultFooter();
      },
      onRow: data => {
        if (data.added) state.addedWords.push(data.word);
        else state.skippedCount++;
        appendWordRow(data);
        renderStatus();
        updateAddResultFooter();
      },
    });
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

  row.innerHTML = buildWordResultRowMarkup({
    data,
    esc,
    typeLabels,
    buildWordResultDetails,
    buildWordResultImage,
    getWordBtnLabel,
    generateType: state.generateType,
    hasProviders: hasAvailableProvider(lexiconState.providers, PROVIDER_MODELS),
    removeSymbol: '✕',
    incorrectSymbol: '✗',
  });

  const resultWordEl = row.querySelector('.result-word');
  if (resultWordEl) resultWordEl.addEventListener('click', () =>
    playWordAudio({ word: data.word }, 1, LEXICON_AUDIO_OPTIONS)
  );

  const removeBtnEl = row.querySelector('.btn-word-remove');
  if (removeBtnEl) removeBtnEl.addEventListener('mousedown', e => removeWordRow(e, removeBtnEl));

  if (data.word_id) {
    const genBtnEl = row.querySelector('.btn-generate');
    if (genBtnEl) genBtnEl.addEventListener('mousedown', e => {
      const t = getGenerateType();
      if (t === 'image') generateWordImage(e, data.word_id, data.word, genBtnEl);
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
  applyWordRowDetailsUpdate({
    containerEl: els.addResultBody,
    data,
    buildWordResultDetails,
    typeLabels,
    getWordBtnLabel,
    generateType: state.generateType,
    onBusyResolved: () => {
      state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
      renderStatus();
    },
  });
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
  return els.addResultImageSourceSelect?.value ?? 'wikimedia';
}

function updateGenerateBtnStates() {
  const type = getGenerateType();
  const hasProviders = hasAvailableProvider(lexiconState.providers, PROVIDER_MODELS);
  const tooltip = type === 'image'
    ? 'Uses an AI API request to find and download an image for this word'
    : 'Uses an AI API request to get the word\'s reading, part-of-speech, meaning, and an example sentence';
  applyGenerateButtonState(els.addResultBody, {
    disabled: !hasProviders,
    tooltip,
    label: getWordBtnLabel(state.generateType),
  });
}

async function generateWordAutofill(event, wordId, word, btn) {
  await generateWordAutofillRequest({
    event,
    wordId,
    word,
    btn,
    aiModel: els.addResultModelSelect.value,
    state,
    renderStatus,
    getWordBtnLabel,
    generateType: state.generateType,
    onWordUpdated: updateWordRowDetails,
  });
}

async function generateWordImage(event, wordId, word, btn) {
  await generateWordImageRequest({
    event,
    wordId,
    word,
    btn,
    aiModel: els.addResultModelSelect.value,
    imageSource: getImageSource(),
    state,
    renderStatus,
    getWordBtnLabel,
    generateType: state.generateType,
    onImageUpdated: (id, imagePath) => updateWordImagePath(id, imagePath),
  });
}

function removeWordRow(event, btn) {
  const word = btn.dataset.word;
  event.stopPropagation();
    modalController.openRemoveConfirm('Remove "' + word + '" from the lexicon?', async () => {
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
  await generateAllAutofillBatchRequest({
    rows,
    aiModel: els.addResultModelSelect.value,
    state,
    renderStatus,
    getWordBtnLabel,
    generateType: state.generateType,
    updateWordRowDetails,
  });
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

  const sel = els.addResultModelSelect;
  if (sel) {
    const busyLock = state.pendingGenerates > 0;
    sel.disabled = busyLock || !(lexiconState.providers && (lexiconState.providers.anthropic || lexiconState.providers.openai || lexiconState.providers.google || lexiconState.providers.mistral || lexiconState.providers.glm));
    if (busyLock) {
      sel.dataset.tooltip = 'Unavailable while generation is in progress';
    } else {
      delete sel.dataset.tooltip;
    }
  }
  const el = els.addResultStatus;
  const actionEl = els.addResultAction;
  const countsHtml = buildCountsHtml({ addedCount: state.addedWords.length, skippedCount: state.skippedCount });
  const hasProviders = hasAvailableProvider(lexiconState.providers, PROVIDER_MODELS);
  const genType = getGenerateType();
  const genAllTooltip = genType === 'image'
    ? 'Uses an AI API request to find and download an image for each word'
    : 'Uses an AI API request to get the reading, part-of-speech, meaning, and an example sentence for each word';
  const genAllEnabled =
    els.addResultBody.querySelectorAll('.word-result-row .btn-generate:not(.btn-generate--busy):not([disabled])').length > 0 &&
    hasProviders && state.addPhase !== 'loading';
  const genTypeLabels = { 'word-info': 'Generate word info', 'image': 'Generate images' };
  const actionHtml = buildWordResultActionHtml({
    pendingGenerates: state.pendingGenerates,
    enabled: genAllEnabled,
    generateType: state.generateType,
    labels: genTypeLabels,
    tooltip: genAllTooltip,
    menuId: 'split-btn-menu',
  });
  if (actionEl) {
    actionEl.innerHTML = actionHtml;
    els.splitBtnMenu = actionEl.querySelector('.split-btn-menu');
    if (state.pendingGenerates > 0) {
      actionEl.querySelector('button').addEventListener('mousedown', cancelAllGenerates);
    } else {
      const mainBtn = actionEl.querySelector('.split-btn-main');
      const arrowBtn = actionEl.querySelector('.split-btn-arrow');
      const menu = els.splitBtnMenu;
      if (mainBtn) mainBtn.addEventListener('mousedown', state.isSingleEdit ? () => generateAll(true, true) : modalController.openGenerateConfirm);
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
  const sourceSel  = els.addResultImageSourceSelect;
  const sourceIcon = els.addResultImageSourceIcon;
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

  const hasProviders = hasAvailableProvider(lexiconState.providers, PROVIDER_MODELS);
  const progTip = providerUnavailableTooltip(lexiconState.providers, PROVIDER_MODELS);
  const optgroupsHtml = buildProviderOptionsHtml(lexiconState.providers, PROVIDER_MODELS);
  const imageSourceOptions = buildImageSourceOptionsHtml(lexiconState.imageSources);
  const imageSourceTip = imageSourceUnavailableTooltip(lexiconState.imageSources);

  els.addResultFooter.innerHTML = buildWordResultFooterHtml({
    hasProviders,
    optgroupsHtml,
    progTip,
    imageSourceOptions,
    imageSourceTip,
  });

  cacheAddResultFooterEls();

  if (hasProviders) {
    const sel = els.addResultModelSelect;
    const first = sel.querySelector('optgroup:not([disabled]) option');
    if (first) sel.value = first.value;
  }

  els.addResultRemoveBtn.addEventListener('click', function () {
    const count = state.addedWords.length;
    const label = count === 1 ? '"' + state.addedWords[0] + '"' : count + ' added words';
    modalController.openRemoveConfirm('Remove ' + label + ' from the lexicon?', async () => {
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
  els.addResultCloseBtn.addEventListener('click', closeAddResultModal);
  updateAddResultFooter();
}

function updateAddResultFooter() {
  if (!els.addResultRemoveBtn) return;
  els.addResultRemoveBtn.disabled = state.addedWords.length === 0;
  els.addResultRemoveBtn.textContent = 'Remove the added words';
  els.addResultCloseBtn.disabled = state.addPhase === 'loading' || state.pendingGenerates > 0;
}
