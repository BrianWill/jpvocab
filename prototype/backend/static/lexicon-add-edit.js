import { defaultDrillTarget, typeLabels, _providers, reloadWords, renderTable, getSortedWords, closeAddModal } from './lexicon.js';
import { esc, isKanji } from './lexicon-utils.js';

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
  });

  document.getElementById('add-result-modal-backdrop').classList.remove('hidden');
  initAddResultFooter();
  document.getElementById('btn-add-result-remove').style.display = 'none';
  renderStatus();
  document.getElementById('add-result-modal-status').style.display = 'none';
  resultBody.querySelector('.result-badge').style.display = 'none';
}

export let _addPhase = 'idle'; // 'loading' | 'done' | 'cancelled'
let _addedWords = [];
let _skippedCount = 0;
export let _pendingGenerates = 0;
let _abortController = null;

document.getElementById('add-result-modal-backdrop').addEventListener('click', function (e) {
  if (e.target === this && _addPhase !== 'loading' && _pendingGenerates === 0) closeAddResultModal();
});
document.getElementById('add-result-modal-close').addEventListener('click', closeAddResultModal);

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

  document.getElementById('add-result-modal-status').style.display = '';
  _addPhase = 'loading';
  _addedWords = [];
  _skippedCount = 0;
  _pendingGenerates = 0;
  _abortController = new AbortController();

  const resultBody = document.getElementById('add-result-modal-body');
  resultBody.innerHTML = '';
  document.getElementById('add-result-modal-backdrop').classList.remove('hidden');
  renderStatus();
  initAddResultFooter();

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
      detailItemPosSelect(data.part_of_speech) +
      detailItemInput('meaning', data.meaning,        'detail-meaning') +
      detailItemExInput(data.example_jp, data.example_en) +
    '</div>';

  row.innerHTML =
    '<div class="word-result-main"><span class="result-word">' + esc(data.word) + '</span>' + badge + inlineExtra + '</div>' +
    details;

  const removeBtnEl = row.querySelector('.btn-word-remove');
  if (removeBtnEl) removeBtnEl.addEventListener('mousedown', e => removeWordRow(e, removeBtnEl));

  if (data.word_id) {
    const genBtnEl = row.querySelector('.btn-generate');
    if (genBtnEl) genBtnEl.addEventListener('mousedown', e => generateWordAutofill(e, data.word_id, data.word, genBtnEl));

    const [adjMinusEl, adjPlusEl] = row.querySelectorAll('.btn-target-adj');
    if (adjMinusEl) adjMinusEl.addEventListener('mousedown', e => adjustWordTarget(e, data.word_id, -1, adjMinusEl));
    if (adjPlusEl) adjPlusEl.addEventListener('mousedown', e => adjustWordTarget(e, data.word_id, 1, adjPlusEl));
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
      detailItemPosSelect(data.part_of_speech) +
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

function detailItemRaw(label, html, muted, cls) {
  return '<span class="detail-item' + (cls ? ' ' + cls : '') + (muted ? ' detail-item--muted' : '') + '">' +
    '<span class="detail-label">' + esc(label) + '</span> ' + html + '</span>';
}

function detailItemPosSelect(value) {
  const known = value in typeLabels;
  let options = known ? '' : '<option value="" selected>—</option>';
  options += Object.entries(typeLabels).map(([key, label]) => {
    const short = label.split(' — ')[0].split(' (')[0].toUpperCase();
    return '<option value="' + esc(key) + '"' + (value === key ? ' selected' : '') + '>' + esc(short) + '</option>';
  }).join('');
  return '<span class="detail-item detail-pos">' +
    '<span class="detail-label">pos</span> ' +
    '<select class="detail-pos-select">' + options + '</select>' +
    '</span>';
}

function detailItemKanjiReadings(word, kanjiData) {
  if (!word || !kanjiData || kanjiData.length === 0) return '';
  let kanjiIdx = 0;
  let pairs = '';
  for (const ch of word) {
    if (isKanji(ch) && kanjiIdx < kanjiData.length) {
      const entry = kanjiData[kanjiIdx++];
      pairs +=
        '<span class="kanji-reading-pair">' +
          '<span class="kanji-reading-char">' + esc(ch) + '</span>' +
          '<span class="detail-input kanji-reading-input" contenteditable="true"' +
            ' data-kanji-id="' + entry.id + '">' + esc((entry.reading || '').trim()) + '</span>' +
        '</span>';
    }
  }
  if (!pairs) return '';
  return '<span class="detail-item detail-kanji">' +
    '<span class="detail-label">kanji readings</span> ' + pairs +
    '</span>';
}

function detailItemInput(label, value, cls) {
  return '<span class="detail-item ' + cls + '">' +
    '<span class="detail-label">' + esc(label) + '</span> ' +
    '<span class="detail-input" contenteditable="true">' + esc((value || '').trim()) + '</span>' +
    '</span>';
}

function detailItemExInput(exJp, exEn) {
  return '<span class="detail-item detail-ex">' +
    '<span class="detail-label">example</span> ' +
    '<span class="detail-ex-inputs">' +
      '<span class="detail-ex-flag">🇯🇵</span><span class="detail-input" contenteditable="true">' + esc((exJp || '').trim()) + '</span>' +
      '<span class="detail-ex-sep">🏴󠁧󠁢󠁥󠁮󠁧󠁿</span><span class="detail-input detail-input--en" contenteditable="true">' + esc((exEn || '').trim()) + '</span>' +
    '</span>' +
    '</span>';
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
  const skippedHtml = _skippedCount > 0
    ? ', <span class="status-skipped">' + _skippedCount + ' skipped</span>'
    : '';
  const countsHtml = '<span>' + _addedWords.length + ' added' + skippedHtml + '</span>';
  const hasProviders = _providers && (_providers.anthropic || _providers.openai || _providers.google || _providers.mistral || _providers.glm);
  const actionHtml = _pendingGenerates > 0
    ? '<button class="btn-generate btn-generate--cancel">' +
        '<span class="spinner"></span>cancel generation' +
      '</button>'
    : '<button class="btn-generate btn-generate--all"' +
        (document.querySelectorAll('#add-result-modal-body .word-result-row .btn-generate:not(.btn-generate--busy):not([disabled])').length > 0 && hasProviders && _addPhase !== 'loading' ? '' : ' disabled') +
        ' data-tooltip="Uses an AI API request to get the reading, part-of-speech, meaning, and an example sentence for each word"' +
        '>generate all</button>';
  if (_addPhase === 'loading') {
    el.className = 'modal-status modal-status-loading';
    el.innerHTML = countsHtml + actionHtml + (_pendingGenerates === 0 ? '<span class="spinner"></span>' : '');
  } else if (_addPhase === 'cancelled') {
    el.className = 'modal-status modal-status-cancelled';
    el.innerHTML = countsHtml + actionHtml + (_pendingGenerates === 0 ? '<span class="status-cancelled-note"> — cancelled</span>' : '');
  } else {
    el.className = 'modal-status ' + (_pendingGenerates > 0 ? 'modal-status-loading' : 'modal-status-done');
    el.innerHTML = countsHtml + actionHtml;
  }
  const actionBtn = el.querySelector('.btn-generate');
  if (actionBtn) actionBtn.addEventListener('mousedown', _pendingGenerates > 0 ? cancelAllGenerates : openGenerateConfirm);
  updateAddResultFooter();
}

function initAddResultFooter() {
  function providerSelectTooltip(providers) {
    const lines = [];
    if (!providers.anthropic) lines.push('Anthropic: set ANTHROPIC_API_KEY to enable');
    if (!providers.openai)    lines.push('OpenAI: set OPENAI_API_KEY to enable');
    if (!providers.google)    lines.push('Google: set GOOGLE_API_KEY to enable');
    if (!providers.mistral)   lines.push('Mistral: set MISTRAL_API_KEY to enable');
    if (!providers.glm)       lines.push('GLM: set GLM_API_KEY to enable');
    if (lines.length === 0) return null;
    return lines.join('\n') + '\n— then restart the program';
  }

  const footer = document.getElementById('add-result-modal-footer');
  const hasProviders = _providers && (_providers.anthropic || _providers.openai || _providers.google || _providers.mistral || _providers.glm);
  const progTip = _providers ? providerSelectTooltip(_providers) : null;
  footer.innerHTML =
    '<button id="btn-add-result-remove" class="btn-danger">Remove added words</button>' +
    '<select id="add-result-model-select" class="add-result-model-select"' +
      (hasProviders ? '' : ' disabled') +
    '>' +
      '<optgroup label="' + (_providers && !_providers.anthropic ? 'Anthropic — no API key' : 'Anthropic') + '"' + (_providers && !_providers.anthropic ? ' disabled' : '') + '>' +
        '<option value="anthropic/claude-haiku-4-5-20251001">claude-haiku (fast)</option>' +
        '<option value="anthropic/claude-sonnet-4-6">claude-sonnet (better)</option>' +
      '</optgroup>' +
      '<optgroup label="' + (_providers && !_providers.openai ? 'OpenAI — no API key' : 'OpenAI') + '"' + (_providers && !_providers.openai ? ' disabled' : '') + '>' +
        '<option value="openai/gpt-4o-mini">gpt-4o-mini (fast)</option>' +
        '<option value="openai/gpt-4o">gpt-4o (better)</option>' +
      '</optgroup>' +
      '<optgroup label="' + (_providers && !_providers.google ? 'Google — no API key' : 'Google') + '"' + (_providers && !_providers.google ? ' disabled' : '') + '>' +
        '<option value="google/gemini-2.0-flash">gemini-2.0-flash (fast)</option>' +
        '<option value="google/gemini-1.5-pro">gemini-1.5-pro (better)</option>' +
      '</optgroup>' +
      '<optgroup label="' + (_providers && !_providers.mistral ? 'Mistral — no API key' : 'Mistral') + '"' + (_providers && !_providers.mistral ? ' disabled' : '') + '>' +
        '<option value="mistral/mistral-small-latest">mistral-small (fast)</option>' +
        '<option value="mistral/mistral-large-latest">mistral-large (better)</option>' +
      '</optgroup>' +
      '<optgroup label="' + (_providers && !_providers.glm ? 'GLM — no API key' : 'GLM') + '"' + (_providers && !_providers.glm ? ' disabled' : '') + '>' +
        '<option value="glm/glm-4">glm-4 (better)</option>' +
        '<option value="glm/glm-3-turbo">glm-3-turbo (fast)</option>' +
      '</optgroup>' +
    '</select>' +
    (progTip ? '<span class="provider-info-icon" data-tooltip="' + progTip + '">?</span>' : '') +
    '<button id="btn-add-result-close" class="btn-save" style="margin-left:auto">Close</button>';

  if (hasProviders) {
    const sel = document.getElementById('add-result-model-select');
    const first = sel.querySelector('optgroup:not([disabled]) option');
    if (first) sel.value = first.value;
  }

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
  btnRemove.textContent = _addedWords.length > 0
    ? 'Remove the ' + _addedWords.length + ' added word' + (_addedWords.length === 1 ? '' : 's')
    : 'Remove added words';
  btnClose.disabled = _addPhase === 'loading' || _pendingGenerates > 0;
}
