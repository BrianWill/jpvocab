import { esc, typeLabels } from './lexicon-utils.js';
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

const STORY_LEXICON_AUDIO_OPTIONS = { preferSynthesis: true, fallbackToBrowserTts: true };

const els = {};

const state = {
  addedWords: [],
  eventsBound: false,
  fieldErrorTimer: null,
  generateType: 'word-info',
  generationCancelled: false,
  imageSources: null,
  pendingGenerates: 0,
  pendingRemoveAction: null,
  prevPendingGenerates: 0,
  providers: null,
  skippedCount: 0,
};

function cacheFooterEls() {
  els.addResultAction = document.getElementById('story-add-result-modal-action');
  els.addResultCloseBtn = document.getElementById('story-btn-add-result-close');
  els.addResultImageSourceIcon = document.getElementById('story-add-result-image-source-icon');
  els.addResultImageSourceSelect = document.getElementById('story-add-result-image-source-select');
  els.addResultModelSelect = document.getElementById('story-add-result-model-select');
  els.addResultRemoveBtn = document.getElementById('story-btn-add-result-remove');
  els.addResultStatus = document.getElementById('story-add-result-modal-status');
  els.splitBtnMenu = document.getElementById('story-split-btn-menu');
}

function ensureModalDom() {
  if (document.getElementById('story-add-result-modal-backdrop')) return;
  const wrapper = document.createElement('div');
  wrapper.innerHTML = `
    <div class="modal-backdrop hidden" id="story-add-result-modal-backdrop">
      <div class="modal modal-wide">
        <div class="modal-header">
          <span id="story-add-result-modal-title">Edit words</span>
          <button class="modal-close" id="story-add-result-modal-close">x</button>
        </div>
        <div class="modal-body modal-body-scroll" id="story-add-result-modal-body"></div>
        <div class="modal-footer" id="story-add-result-modal-footer"></div>
      </div>
    </div>
    <div id="story-generate-confirm-modal-backdrop" class="modal-backdrop hidden">
      <div class="modal">
        <div class="modal-body">
          <p>Generate readings, POS, meanings, and sentences for:</p>
          <label class="generate-confirm-option">
            <input type="checkbox" id="story-generate-confirm-added-checkbox">
            <span id="story-generate-confirm-added-text"></span>
          </label>
          <label class="generate-confirm-option">
            <input type="checkbox" id="story-generate-confirm-skipped-checkbox">
            <span id="story-generate-confirm-skipped-text"></span>
          </label>
        </div>
        <div class="modal-footer">
          <button class="btn-cancel" id="story-generate-confirm-cancel">Cancel</button>
          <button id="story-generate-confirm-ok" class="btn-save">Generate</button>
        </div>
      </div>
    </div>
    <div id="story-remove-confirm-modal-backdrop" class="modal-backdrop hidden">
      <div class="modal">
        <div class="modal-body">
          <p id="story-remove-confirm-text"></p>
        </div>
        <div class="modal-footer">
          <button class="btn-cancel" id="story-remove-confirm-cancel">Cancel</button>
          <button id="story-remove-confirm-ok" class="btn-danger">Remove</button>
        </div>
      </div>
    </div>`;
  document.body.append(...wrapper.children);
}

function cacheEls() {
  els.addResultBody = document.getElementById('story-add-result-modal-body');
  els.addResultClose = document.getElementById('story-add-result-modal-close');
  els.addResultFooter = document.getElementById('story-add-result-modal-footer');
  els.addResultModalBackdrop = document.getElementById('story-add-result-modal-backdrop');
  els.addResultTitle = document.getElementById('story-add-result-modal-title');
  els.generateConfirmAddedCheckbox = document.getElementById('story-generate-confirm-added-checkbox');
  els.generateConfirmAddedText = document.getElementById('story-generate-confirm-added-text');
  els.generateConfirmCancel = document.getElementById('story-generate-confirm-cancel');
  els.generateConfirmModalBackdrop = document.getElementById('story-generate-confirm-modal-backdrop');
  els.generateConfirmOk = document.getElementById('story-generate-confirm-ok');
  els.generateConfirmSkippedCheckbox = document.getElementById('story-generate-confirm-skipped-checkbox');
  els.generateConfirmSkippedText = document.getElementById('story-generate-confirm-skipped-text');
  els.removeConfirmCancel = document.getElementById('story-remove-confirm-cancel');
  els.removeConfirmModalBackdrop = document.getElementById('story-remove-confirm-modal-backdrop');
  els.removeConfirmOk = document.getElementById('story-remove-confirm-ok');
  els.removeConfirmText = document.getElementById('story-remove-confirm-text');
}

async function loadSupportState() {
  const providers = await fetch('/api/providers').then(r => r.json()).catch(() => null);
  state.providers = providers?.ai ?? null;
  state.imageSources = providers?.image_sources ?? null;
}

export async function initStoryAddToLexicon() {
  ensureModalDom();
  cacheEls();
  modalController.bindBaseEvents();
  await loadSupportState();
}

const modalController = createWordResultModalController({
  els,
  state,
  closeModal: closeAddResultModal,
  canClose: () => state.pendingGenerates === 0,
  closeButtonId: 'story-btn-add-result-close',
  onGenerateAll: generateAll,
  onPlayExampleSentence: ({ row, text }) => {
    playSentenceAudio({
      word: row?._resolvedWord ?? '',
      exampleJp: text,
    }, 1, STORY_LEXICON_AUDIO_OPTIONS);
  },
  onUploadComplete: undefined,
  onSaveRowEdits: saveWordRowEdits,
  bindWordResultEditorEvents,
  bindWordResultImageUpload,
});

export async function addWordsToLexicon(words) {
  const rawWords = words.map(word => String(word ?? '').trim()).filter(Boolean).join('\n');
  if (!rawWords) return;

  state.addedWords = [];
  state.skippedCount = 0;
  state.pendingGenerates = 0;
  els.addResultBody.innerHTML = '';
  els.addResultModalBackdrop.classList.remove('hidden');
  initAddResultFooter();
  renderStatus();

  await streamBatchAdd({
    rawWords,
    onUpdated: data => {
      updateWordRowDetails(data);
    },
    onDone: () => {
      sortAddResultRows(els.addResultBody);
      renderStatus();
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
}

function appendWordRow(data) {
  const row = document.createElement('div');
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
    hasProviders: hasAvailableProvider(state.providers, PROVIDER_MODELS),
    removeSymbol: 'x',
    incorrectSymbol: '✕',
  });

  row.querySelector('.result-word')?.addEventListener('click', () =>
    playWordAudio({ word: data.word }, 1, STORY_LEXICON_AUDIO_OPTIONS)
  );
  row.querySelector('.btn-word-remove')?.addEventListener('mousedown', e => removeWordRow(e, row.querySelector('.btn-word-remove')));

  if (data.word_id) {
    row.querySelector('.btn-generate')?.addEventListener('mousedown', e => {
      const btn = row.querySelector('.btn-generate');
      const type = getGenerateType();
      if (type === 'image') generateWordImage(e, data.word_id, data.word, btn);
      else generateWordAutofill(e, data.word_id, data.word, btn);
    });
    const [minusBtn, plusBtn] = row.querySelectorAll('.btn-target-adj');
    minusBtn?.addEventListener('mousedown', e => adjustWordTarget(e, data.word_id, -1, minusBtn));
    plusBtn?.addEventListener('mousedown', e => adjustWordTarget(e, data.word_id, 1, plusBtn));
  }

  els.addResultBody.appendChild(row);
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

function getGenerateType() {
  return state.generateType || 'word-info';
}

function getImageSource() {
  return els.addResultImageSourceSelect?.value || 'wikimedia';
}

async function generateWordAutofill(event, wordId, word, btn) {
  await generateWordAutofillRequest({
    event,
    wordId,
    word,
    btn,
    aiModel: els.addResultModelSelect?.value,
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
    aiModel: els.addResultModelSelect?.value,
    imageSource: getImageSource(),
    state,
    renderStatus,
    getWordBtnLabel,
    generateType: state.generateType,
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
    if (!res.ok) {
      btn.disabled = false;
      return;
    }
    btn.closest('.word-result-row').remove();
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

function generateAll(includeAdded, includeSkipped) {
  const buttons = [];
  if (state.generateType === 'word-info') {
    if (includeAdded) buttons.push(...els.addResultBody.querySelectorAll('.result-added .btn-generate:not(.btn-generate--busy):not([disabled])'));
    if (includeSkipped) buttons.push(...els.addResultBody.querySelectorAll('.result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])'));
    generateAllAutofillBatch(buttons.map(btn => btn.closest('.word-result-row')));
    return;
  }
  if (includeAdded) buttons.push(...els.addResultBody.querySelectorAll('.result-added .btn-generate:not(.btn-generate--busy):not([disabled])'));
  if (includeSkipped) buttons.push(...els.addResultBody.querySelectorAll('.result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])'));
  buttons.forEach(btn => btn.dispatchEvent(new MouseEvent('mousedown')));
}

async function generateAllAutofillBatch(rows) {
  await generateAllAutofillBatchRequest({
    rows,
    aiModel: els.addResultModelSelect?.value,
    state,
    renderStatus,
    getWordBtnLabel,
    generateType: state.generateType,
    updateWordRowDetails,
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

function renderStatus() {
  if (state.pendingGenerates > 0 && state.prevPendingGenerates === 0) state.generationCancelled = false;
  if (state.prevPendingGenerates > 0 && state.pendingGenerates === 0 && !state.generationCancelled) playDing();
  state.prevPendingGenerates = state.pendingGenerates;

  els.addResultTitle.textContent = 'Edit words';
  const sel = els.addResultModelSelect;
  if (sel) sel.disabled = state.pendingGenerates > 0 || !(state.providers && PROVIDER_MODELS.some(p => state.providers[p.key]));
  const statusEl = els.addResultStatus;
  const actionEl = els.addResultAction;
  const hasProviders = hasAvailableProvider(state.providers, PROVIDER_MODELS);
  const genType = getGenerateType();
  const tooltip = genType === 'image'
    ? 'Uses an AI API request to find and download an image for each word'
    : 'Uses an AI API request to get the reading, part-of-speech, meaning, and an example sentence for each word';
  applyGenerateButtonState(els.addResultBody, {
    disabled: !hasProviders,
    tooltip,
    label: getWordBtnLabel(state.generateType),
  });
  statusEl.className = 'modal-status ' + (state.pendingGenerates > 0 ? 'modal-status-loading' : 'modal-status-done');
  statusEl.innerHTML = buildCountsHtml({ addedCount: state.addedWords.length, skippedCount: state.skippedCount });
  const enabled = els.addResultBody.querySelectorAll('.word-result-row .btn-generate:not(.btn-generate--busy):not([disabled])').length > 0 &&
    hasProviders;
  const labels = { 'word-info': 'Generate word info', 'image': 'Generate images' };
  actionEl.innerHTML = buildWordResultActionHtml({
    pendingGenerates: state.pendingGenerates,
    enabled,
    generateType: genType,
    labels,
    tooltip,
    menuId: 'story-split-btn-menu',
    mainButtonClass: 'story-split-btn-main',
    arrowButtonClass: 'story-split-btn-arrow',
  });
  els.splitBtnMenu = actionEl.querySelector('.split-btn-menu');
  if (state.pendingGenerates > 0) {
    actionEl.querySelector('button')?.addEventListener('mousedown', cancelAllGenerates);
  } else {
    const mainBtn = actionEl.querySelector('.split-btn-main');
    const arrowBtn = actionEl.querySelector('.split-btn-arrow');
    const menu = els.splitBtnMenu;
    mainBtn?.addEventListener('mousedown', modalController.openGenerateConfirm);
    arrowBtn?.addEventListener('mousedown', e => {
      e.stopPropagation();
      menu.hidden = !menu.hidden;
    });
    menu?.querySelectorAll('.split-btn-option').forEach(option => {
      option.addEventListener('mousedown', e => {
        e.stopPropagation();
        state.generateType = option.dataset.type;
        menu.hidden = true;
        renderStatus();
      });
    });
  }

  const sourceDisplay = genType === 'image' ? '' : 'none';
  const sourceSel = els.addResultImageSourceSelect;
  const sourceIcon = els.addResultImageSourceIcon;
  if (sourceSel) sourceSel.style.display = sourceDisplay;
  if (sourceIcon) sourceIcon.style.display = sourceDisplay;
}

function initAddResultFooter() {
  const hasProviders = hasAvailableProvider(state.providers, PROVIDER_MODELS);
  const progTip = providerUnavailableTooltip(state.providers, PROVIDER_MODELS);
  const optgroupsHtml = buildProviderOptionsHtml(state.providers, PROVIDER_MODELS);
  const imageSourceOptions = buildImageSourceOptionsHtml(state.imageSources);
  const imageSourceTip = imageSourceUnavailableTooltip(state.imageSources);

  els.addResultFooter.innerHTML = buildWordResultFooterHtml({
    prefix: 'story-',
    hasProviders,
    optgroupsHtml,
    progTip: esc(progTip),
    imageSourceOptions,
    imageSourceTip: imageSourceTip ? esc(imageSourceTip) : imageSourceTip,
  });

  cacheFooterEls();

  if (hasProviders) {
    const sel = els.addResultModelSelect;
    const first = sel.querySelector('optgroup:not([disabled]) option');
    if (first) sel.value = first.value;
  }

  els.addResultRemoveBtn.addEventListener('click', () => {
    const count = state.addedWords.length;
    const label = count === 1 ? '"' + state.addedWords[0] + '"' : count + ' added words';
    modalController.openRemoveConfirm('Remove ' + label + ' from the lexicon?', async () => {
      await fetch('/admin/words/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ words: state.addedWords.slice() }),
      });
      state.addedWords = [];
      els.addResultBody.querySelectorAll('.badge-added').forEach(badge => badge.closest('.word-result-row').remove());
      if (els.addResultBody.querySelectorAll('.word-result-row').length === 0) {
        closeAddResultModal();
        return;
      }
      renderStatus();
      updateAddResultFooter();
    });
  });
  els.addResultCloseBtn.addEventListener('click', closeAddResultModal);
  updateAddResultFooter();
}

function updateAddResultFooter() {
  if (!els.addResultRemoveBtn || !els.addResultCloseBtn) return;
  els.addResultRemoveBtn.disabled = state.addedWords.length === 0;
  els.addResultCloseBtn.disabled = state.pendingGenerates > 0;
}

export function closeAddResultModal() {
  if (state.pendingGenerates > 0) return;
  els.addResultModalBackdrop.classList.add('hidden');
}

