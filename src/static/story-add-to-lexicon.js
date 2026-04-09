import { esc, typeLabels } from './lexicon-utils.js';
import { playWordAudio, playSentenceAudio, playDing, PROVIDER_MODELS } from './common.js';
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
} from './add-to-lexicon.js';

const STORY_LEXICON_AUDIO_OPTIONS = { preferSynthesis: true, fallbackToBrowserTts: true };

const els = {};

const state = {
  addedWords: [],
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

let eventsBound = false;

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

function openRemoveConfirm(message, action) {
  state.pendingRemoveAction = action;
  els.removeConfirmText.textContent = message;
  els.removeConfirmModalBackdrop.classList.remove('hidden');
}

function closeRemoveConfirm() {
  state.pendingRemoveAction = null;
  els.removeConfirmModalBackdrop.classList.add('hidden');
}

function openGenerateConfirm() {
  const addedCount = els.addResultBody.querySelectorAll('.result-added .btn-generate:not(.btn-generate--busy):not([disabled])').length;
  const skippedCount = els.addResultBody.querySelectorAll('.result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])').length;
  els.generateConfirmAddedText.textContent = addedCount + ' newly added words';
  els.generateConfirmSkippedText.textContent = skippedCount + ' already existing words';
  els.generateConfirmAddedCheckbox.checked = true;
  els.generateConfirmSkippedCheckbox.checked = false;
  els.generateConfirmModalBackdrop.classList.remove('hidden');
}

function closeGenerateConfirm() {
  els.generateConfirmModalBackdrop.classList.add('hidden');
}

function bindEvents() {
  if (eventsBound) return;
  eventsBound = true;

  document.addEventListener('mousedown', () => {
    const menu = document.getElementById('story-split-btn-menu');
    if (menu) menu.hidden = true;
  });

  els.addResultModalBackdrop.addEventListener('click', e => {
    if (e.target === els.addResultModalBackdrop && state.pendingGenerates === 0) closeAddResultModal();
  });
  els.addResultClose.addEventListener('click', closeAddResultModal);

  els.addResultBody.addEventListener('click', e => {
    if (!e.target.closest('.detail-ex-play')) return;
    const row = e.target.closest('.word-result-row');
    const jpInput = e.target.closest('.detail-ex-inputs')?.querySelector('.detail-input:not(.detail-input--en)');
    const text = jpInput?.textContent.trim();
    if (text) {
      playSentenceAudio({
        word: row?._resolvedWord ?? '',
        exampleJp: text,
      }, 1, STORY_LEXICON_AUDIO_OPTIONS);
    }
  });

  bindWordResultEditorEvents({
    containerEl: els.addResultBody,
    footerEl: els.addResultFooter,
    closeButtonId: 'story-btn-add-result-close',
    state,
    onSaveRowEdits: saveWordRowEdits,
  });

  bindWordResultImageUpload({
    containerEl: els.addResultBody,
  });

  els.removeConfirmModalBackdrop.addEventListener('click', e => {
    if (e.target === els.removeConfirmModalBackdrop) closeRemoveConfirm();
  });
  els.removeConfirmCancel.addEventListener('click', closeRemoveConfirm);
  els.removeConfirmOk.addEventListener('click', () => {
    const action = state.pendingRemoveAction;
    closeRemoveConfirm();
    if (action) action();
  });

  els.generateConfirmModalBackdrop.addEventListener('click', e => {
    if (e.target === els.generateConfirmModalBackdrop) closeGenerateConfirm();
  });
  els.generateConfirmCancel.addEventListener('click', closeGenerateConfirm);
  els.generateConfirmOk.addEventListener('click', () => {
    const includeAdded = els.generateConfirmAddedCheckbox.checked;
    const includeSkipped = els.generateConfirmSkippedCheckbox.checked;
    closeGenerateConfirm();
    generateAll(includeAdded, includeSkipped);
  });
}

async function loadSupportState() {
  const providers = await fetch('/api/providers').then(r => r.json()).catch(() => null);
  state.providers = providers?.ai ?? null;
  state.imageSources = providers?.image_sources ?? null;
}

export async function initStoryAddToLexicon() {
  ensureModalDom();
  cacheEls();
  bindEvents();
  await loadSupportState();
}

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

  const form = new FormData();
  form.append('words', rawWords);
  form.append('autofill', 'off');

  const res = await fetch('/admin/words/batch', { method: 'POST', body: form });
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
      if (data.updated) {
        updateWordRowDetails(data);
        continue;
      }
      if (data.done) {
        sortAddResultRows(els.addResultBody);
        renderStatus();
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
}

function appendWordRow(data) {
  const row = document.createElement('div');
  row._resolvedWord = data.word;
  row._wordId = data.word_id || null;
  row.className = 'word-result-row ' + (data.added ? 'result-added' : 'result-skipped');
  row.dataset.reason = data.added ? 'added' : (data.reason || '');

  const badge = data.added
    ? '<span class="result-badge badge-added">added</span>'
    : '<span class="result-badge badge-skipped">' + esc(data.reason) + '</span>';
  const removeBtn = '<button class="btn-delete btn-word-remove" data-tooltip="Remove word" data-word="' + esc(data.word) + '">x</button>';
  const hasProviders = !!(state.providers && PROVIDER_MODELS.some(p => state.providers[p.key]));
  const generateBtn = data.word_id
    ? '<button class="btn-generate"' + (hasProviders ? '' : ' disabled') + ' data-tooltip="Uses an AI API request to get the word\'s reading, part-of-speech, meaning, and an example sentence">' + getWordBtnLabel(state.generateType) + '</button>'
    : '';
  let inlineExtra;
  if (data.word_id) {
    const correct = data.drill_count ?? 0;
    const incorrect = data.drill_incorrect ?? 0;
    const target = data.drill_target ?? 0;
    inlineExtra =
      '<span class="word-result-drill">' +
        '<span class="word-result-actions">' + generateBtn + removeBtn + '</span>' +
        '<span class="drill-correct" data-tooltip="Times answered correctly">✓ ' + correct + '</span>' +
        '<span class="drill-incorrect" data-tooltip="Times answered incorrectly">✕ ' + incorrect + '</span>' +
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

  row.innerHTML =
    '<div class="word-result-main"><span class="result-word">' + esc(data.word) + '</span>' + badge + inlineExtra + '</div>' +
    '<div class="word-result-body">' + details + buildWordResultImage(data.image_path, '') + '</div>';

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
  const row = Array.from(els.addResultBody.children).find(el => el._resolvedWord === data.word);
  if (!row) return;
  row.querySelector('.word-result-details').outerHTML = buildWordResultDetails(data.word, data, typeLabels);
}

function getGenerateType() {
  return state.generateType || 'word-info';
}

function getImageSource() {
  return document.getElementById('story-add-result-image-source-select')?.value || 'wikimedia';
}

async function generateWordAutofill(event, wordId, word, btn) {
  event.stopPropagation();
  if (btn._generateAbort) {
    btn._generateAbort.abort();
    return;
  }
  if (btn.classList.contains('btn-generate--busy')) return;
  const abort = new AbortController();
  btn._generateAbort = abort;
  btn.classList.add('btn-generate--busy', 'btn-generate--cancellable');
  btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">generating…</span><span class="btn-gen-cancel">cancel generation</span>';
  state.pendingGenerates++;
  renderStatus();
  try {
    const aiModel = document.getElementById('story-add-result-model-select')?.value;
    const res = await fetch('/api/words/' + wordId + '/autofill', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ word, ai_model: aiModel }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    updateWordRowDetails(await res.json());
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
  const abort = new AbortController();
  btn._generateAbort = abort;
  btn.classList.add('btn-generate--busy', 'btn-generate--cancellable');
  btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">finding image…</span><span class="btn-gen-cancel">cancel</span>';
  state.pendingGenerates++;
  renderStatus();
  const row = btn.closest('.word-result-row');
  const meaning = row.querySelector('.detail-meaning .detail-input')?.textContent.trim() || '';
  const prevImageHtml = row?.querySelector('.word-result-image')?.outerHTML ?? null;
  try {
    setWordRowImage(row, '', 'loading');
    const res = await fetch('/api/words/' + wordId + '/find-image', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        word,
        meaning,
        ai_model: document.getElementById('story-add-result-model-select')?.value,
        image_source: getImageSource(),
      }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    setWordRowImage(row, data.image_path, '', Date.now());
  } catch (_) {
    const imageEl = row?.querySelector('.word-result-image');
    if (imageEl && prevImageHtml) imageEl.outerHTML = prevImageHtml;
    else setWordRowImage(row, '', 'failed');
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
  const abort = new AbortController();
  const aiModel = document.getElementById('story-add-result-model-select')?.value;
  const wordItems = rows.filter(Boolean).map(row => ({ id: row._wordId, word: row._resolvedWord, btn: row.querySelector('.btn-generate') }));
  if (wordItems.length === 0) return;
  for (const item of wordItems) {
    item.btn._generateAbort = abort;
    item.btn.classList.add('btn-generate--busy', 'btn-generate--cancellable');
    item.btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">generating…</span><span class="btn-gen-cancel">cancel generation</span>';
    state.pendingGenerates++;
  }
  renderStatus();
  try {
    const res = await fetch('/api/words/autofill-batch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ words: wordItems.map(item => ({ id: item.id, word: item.word })), ai_model: aiModel }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    const results = await res.json();
    for (const item of wordItems) {
      if (item.btn._generateAbort === abort) item.btn._generateAbort = null;
    }
    for (const result of results) {
      if (!result.error) updateWordRowDetails(result);
    }
  } finally {
    for (const item of wordItems) {
      item.btn._generateAbort = null;
      item.btn.classList.remove('btn-generate--cancellable');
      if (item.btn.classList.contains('btn-generate--busy')) {
        item.btn.classList.remove('btn-generate--busy');
        item.btn.innerHTML = getWordBtnLabel(state.generateType);
        state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
      }
    }
    renderStatus();
  }
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
  const sel = document.getElementById('story-add-result-model-select');
  if (sel) sel.disabled = state.pendingGenerates > 0 || !(state.providers && PROVIDER_MODELS.some(p => state.providers[p.key]));
  const statusEl = document.getElementById('story-add-result-modal-status');
  const actionEl = document.getElementById('story-add-result-modal-action');
  const skippedHtml = state.skippedCount > 0 ? '<span class="status-skipped">' + state.skippedCount + ' skipped</span>' : '';
  statusEl.className = 'modal-status ' + (state.pendingGenerates > 0 ? 'modal-status-loading' : 'modal-status-done');
  statusEl.innerHTML =
    '<span class="modal-status-counts">' +
      '<span>' + state.addedWords.length + ' added</span>' +
      skippedHtml +
    '</span>';

  const hasProviders = !!(state.providers && PROVIDER_MODELS.some(p => state.providers[p.key]));
  const genType = getGenerateType();
  const enabled = els.addResultBody.querySelectorAll('.word-result-row .btn-generate:not(.btn-generate--busy):not([disabled])').length > 0 &&
    hasProviders;
  const labels = { 'word-info': 'Generate word info', 'image': 'Generate images' };
  if (state.pendingGenerates > 0) {
    actionEl.innerHTML =
      '<button class="btn-danger btn-generate--cancel">' +
        '<span class="spinner"></span>Cancel generation' +
      '</button>';
    actionEl.querySelector('button')?.addEventListener('mousedown', cancelAllGenerates);
  } else {
    actionEl.innerHTML =
      '<div class="split-btn-wrap">' +
        '<button class="btn-save btn-generate--all split-btn-main story-split-btn-main"' + (enabled ? '' : ' disabled') + '>' + labels[genType] + '</button>' +
        '<button class="btn-save btn-generate--all split-btn-arrow story-split-btn-arrow"' + (enabled ? '' : ' disabled') + '>▾</button>' +
        '<div class="split-btn-menu" id="story-split-btn-menu" hidden>' +
          ['word-info', 'image'].map(type => '<button class="split-btn-option' + (type === genType ? ' split-btn-option--active' : '') + '" data-type="' + type + '">' + labels[type] + '</button>').join('') +
        '</div>' +
      '</div>';

    const mainBtn = actionEl.querySelector('.split-btn-main');
    const arrowBtn = actionEl.querySelector('.split-btn-arrow');
    const menu = document.getElementById('story-split-btn-menu');
    mainBtn?.addEventListener('mousedown', openGenerateConfirm);
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
  const sourceSel = document.getElementById('story-add-result-image-source-select');
  const sourceIcon = document.getElementById('story-add-result-image-source-icon');
  if (sourceSel) sourceSel.style.display = sourceDisplay;
  if (sourceIcon) sourceIcon.style.display = sourceDisplay;
}

function initAddResultFooter() {
  const imageSources = [
    { key: 'unsplash', label: 'Unsplash', envKey: 'UNSPLASH_ACCESS_KEY' },
    { key: 'pexels', label: 'Pexels', envKey: 'PEXELS_API_KEY' },
    { key: 'pixabay', label: 'Pixabay', envKey: 'PIXABAY_API_KEY' },
    { key: 'bing', label: 'Bing', envKey: 'BING_API_KEY' },
  ];
  const hasProviders = !!(state.providers && PROVIDER_MODELS.some(p => state.providers[p.key]));
  const progTip = state.providers
    ? PROVIDER_MODELS.filter(p => !state.providers[p.key]).map(p => p.label + ': set ' + p.envKey + ' to enable').join('\n')
    : null;
  const optgroupsHtml = PROVIDER_MODELS.map(({ key, label, models }) => {
    const avail = state.providers && state.providers[key];
    const groupLabel = avail ? label : label + ' - no API key';
    const options = models.map(([val, text]) => '<option value="' + val + '">' + text + '</option>').join('');
    return '<optgroup label="' + groupLabel + '"' + (avail ? '' : ' disabled') + '>' + options + '</optgroup>';
  }).join('');
  const imageSourceOptions =
    '<option value="wikimedia">Wikimedia</option>' +
    imageSources.map(({ key, label }) => {
      const avail = state.imageSources && state.imageSources[key];
      return '<option value="' + key + '"' + (avail ? '' : ' disabled') + '>' + label + (avail ? '' : ' - no key') + '</option>';
    }).join('');
  const imageSourceTip = state.imageSources
    ? imageSources.filter(source => !state.imageSources[source.key]).map(source => source.label + ': set ' + source.envKey + ' to enable').join('\n')
    : null;

  els.addResultFooter.innerHTML =
    '<select id="story-add-result-model-select" class="add-result-model-select"' + (hasProviders ? '' : ' disabled') + '>' +
      (!hasProviders ? '<option value="" selected>no API keys configured</option>' : '') +
      optgroupsHtml +
    '</select>' +
    (progTip ? '<span class="provider-info-icon" data-tooltip="' + esc(progTip) + '">?</span>' : '') +
    '<div id="story-add-result-modal-action" style="margin-left:0.4rem;display:flex;align-items:center;gap:0.4rem"></div>' +
    '<select id="story-add-result-image-source-select" class="add-result-model-select" style="display:none">' + imageSourceOptions + '</select>' +
    (imageSourceTip ? '<span id="story-add-result-image-source-icon" class="provider-info-icon" style="display:none" data-tooltip="' + esc(imageSourceTip) + '">?</span>' : '') +
    '<div id="story-add-result-modal-status" class="modal-status" style="padding:0;border:none;margin-left:auto"></div>' +
    '<button id="story-btn-add-result-remove" class="btn-danger">Remove the added words</button>' +
    '<button id="story-btn-add-result-close" class="btn-save">Close</button>';

  if (hasProviders) {
    const sel = document.getElementById('story-add-result-model-select');
    const first = sel.querySelector('optgroup:not([disabled]) option');
    if (first) sel.value = first.value;
  }

  document.getElementById('story-btn-add-result-remove').addEventListener('click', () => {
    const count = state.addedWords.length;
    const label = count === 1 ? '"' + state.addedWords[0] + '"' : count + ' added words';
    openRemoveConfirm('Remove ' + label + ' from the lexicon?', async () => {
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
  document.getElementById('story-btn-add-result-close').addEventListener('click', closeAddResultModal);
  updateAddResultFooter();
}

function updateAddResultFooter() {
  const btnRemove = document.getElementById('story-btn-add-result-remove');
  const btnClose = document.getElementById('story-btn-add-result-close');
  if (!btnRemove || !btnClose) return;
  btnRemove.disabled = state.addedWords.length === 0;
  btnClose.disabled = state.pendingGenerates > 0;
}

export function closeAddResultModal() {
  if (state.pendingGenerates > 0) return;
  els.addResultModalBackdrop.classList.add('hidden');
}

