import { esc, detailItemPosSelect, detailItemKanjiReadings, detailItemInput, detailItemExInput, getFirstImageFile } from './lexicon-utils.js';
import { getFieldLanguageErrorMsg, getFieldLanguageFilter, getFieldLanguageKind, sanitizeFieldInput } from './add-to-lexicon-utils.js';

const imagePlaceholderSvg =
  '<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">' +
    '<rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="1.5"/>' +
    '<circle cx="8.5" cy="8.5" r="1.5" fill="currentColor"/>' +
    '<polyline points="3,21 8,14 12,18 16,13 21,18" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/>' +
  '</svg>';

export function sortAddResultRows(container) {
  if (!container) return;
  const rows = Array.from(container.children);
  rows.sort((a, b) => {
    const aLexicon = a.dataset.reason === 'already in lexicon' ? 1 : 0;
    const bLexicon = b.dataset.reason === 'already in lexicon' ? 1 : 0;
    return aLexicon - bLexicon;
  });
  rows.forEach(row => container.appendChild(row));
}

export function buildWordResultImage(imagePath, imageState, bust = '') {
  const baseAttrs =
    ' type="button" data-tooltip="Choose an image from your computer"' +
    ' aria-label="Choose an image from your computer"';
  if (imagePath) {
    return '<button class="word-result-image"' + baseAttrs + '><img src="/static/' + esc(imagePath) + (bust ? '?v=' + bust : '') + '" alt=""></button>';
  }
  const classes = ['word-result-image', 'word-result-image--empty'];
  let overlay = '';
  if (imageState === 'loading') {
    classes.push('word-result-image--loading');
    overlay = '<span class="spinner word-result-image-spinner" aria-hidden="true"></span>';
  } else if (imageState === 'failed') {
    classes.push('word-result-image--failed');
  }
  return '<button class="' + classes.join(' ') + '"' + baseAttrs + '>' + overlay + imagePlaceholderSvg + '</button>';
}

export function setWordRowImage(row, imagePath, imageState = '', bust = '') {
  const imageEl = row?.querySelector('.word-result-image');
  if (!imageEl) return;
  imageEl.outerHTML = buildWordResultImage(imagePath, imageState, bust);
}

export function bindWordResultImageUpload({ containerEl, onUploadComplete }) {
  const inputEl = document.createElement('input');
  inputEl.type = 'file';
  inputEl.accept = 'image/*';
  inputEl.hidden = true;
  document.body.appendChild(inputEl);

  let activeRow = null;

  function setDragState(row, active) {
    const imageEl = row?.querySelector('.word-result-image');
    if (!imageEl) return;
    imageEl.classList.toggle('word-result-image--drop-target', active);
  }

  function clearDragState(row) {
    setDragState(row, false);
  }

  async function uploadImageForRow(row, file) {
    if (!row || !file || !row._wordId || row._imageUploadBusy) return;

    row._imageUploadBusy = true;
    const prevImageHtml = row.querySelector('.word-result-image')?.outerHTML ?? null;
    setWordRowImage(row, '', 'loading');

    try {
      const formData = new FormData();
      formData.append('image', file);

      const res = await fetch('/api/words/' + row._wordId + '/upload-image', {
        method: 'POST',
        body: formData,
      });
      if (!res.ok) throw new Error(await res.text());

      const data = await res.json();
      if (!row.isConnected) return;
      setWordRowImage(row, data.image_path, '', Date.now());
      onUploadComplete?.(row._wordId, data.image_path, row);
    } catch (_) {
      if (!row.isConnected) return;
      const imageEl = row.querySelector('.word-result-image');
      if (imageEl && prevImageHtml) imageEl.outerHTML = prevImageHtml;
      else setWordRowImage(row, '', 'failed');
    } finally {
      row._imageUploadBusy = false;
    }
  }

  containerEl.addEventListener('click', event => {
    const imageEl = event.target.closest('.word-result-image');
    if (!imageEl) return;
    const row = imageEl.closest('.word-result-row');
    if (!row?._wordId || row._imageUploadBusy) return;
    activeRow = row;
    inputEl.value = '';
    inputEl.click();
  });

  inputEl.addEventListener('change', async () => {
    const row = activeRow;
    activeRow = null;
    const file = inputEl.files?.[0];
    if (!row || !file) return;
    await uploadImageForRow(row, file);
  });

  containerEl.addEventListener('dragover', event => {
    const imageEl = event.target.closest('.word-result-image');
    if (!imageEl) return;
    const row = imageEl.closest('.word-result-row');
    if (!row?._wordId || row._imageUploadBusy) return;
    if (!Array.from(event.dataTransfer?.types || []).includes('Files')) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'copy';
    setDragState(row, true);
  });

  containerEl.addEventListener('dragleave', event => {
    const imageEl = event.target.closest('.word-result-image');
    if (!imageEl) return;
    const nextTarget = event.relatedTarget;
    if (nextTarget && imageEl.contains(nextTarget)) return;
    clearDragState(imageEl.closest('.word-result-row'));
  });

  containerEl.addEventListener('drop', async event => {
    const imageEl = event.target.closest('.word-result-image');
    if (!imageEl) return;
    const row = imageEl.closest('.word-result-row');
    clearDragState(row);
    if (!row?._wordId || row._imageUploadBusy) return;
    if (!Array.from(event.dataTransfer?.types || []).includes('Files')) return;
    event.preventDefault();
    const file = getFirstImageFile(event.dataTransfer?.files);
    if (!file) {
      setWordRowImage(row, '', 'failed');
      return;
    }
    await uploadImageForRow(row, file);
  });
}

export function buildWordResultDetails(word, data, typeLabels) {
  return (
    '<div class="word-result-details">' +
      detailItemInput('reading', data.reading, 'detail-reading') +
      detailItemKanjiReadings(word, data.kanji_data) +
      detailItemPosSelect(data.part_of_speech, typeLabels) +
      detailItemInput('meaning', data.meaning, 'detail-meaning') +
      detailItemExInput(data.example_jp, data.example_en) +
    '</div>'
  );
}

export function getWordBtnLabel(generateType) {
  return generateType === 'image'
    ? 'generate image'
    : 'generate word info';
}

export function bindWordResultEditorEvents({ containerEl, footerEl, closeButtonId, state, onSaveRowEdits }) {
  containerEl.addEventListener('keydown', event => {
    if (event.key !== 'Enter' || event.isComposing || !event.target.classList.contains('detail-input')) return;
    event.preventDefault();
    event.target.blur();
  });

  containerEl.addEventListener('input', event => {
    if (event.isComposing || !event.target.classList.contains('detail-input')) return;
    enforceFieldLanguage(event.target, footerEl, closeButtonId, state);
  });

  containerEl.addEventListener('compositionend', event => {
    if (!event.target.classList.contains('detail-input')) return;
    enforceFieldLanguage(event.target, footerEl, closeButtonId, state);
  });

  containerEl.addEventListener('paste', event => {
    const fieldEl = event.target;
    if (!fieldEl.classList.contains('detail-input')) return;
    const filter = getFieldLanguageFilter(fieldEl);
    if (!filter) return;
    event.preventDefault();
    const text = (event.clipboardData || window.clipboardData).getData('text/plain');
    document.execCommand('insertText', false, filter(text));
  });

  containerEl.addEventListener('focusout', event => {
    if (!event.target.classList.contains('detail-input')) return;
    const row = event.target.closest('.word-result-row');
    if (row) onSaveRowEdits(row);
  });

  containerEl.addEventListener('change', event => {
    if (!event.target.classList.contains('detail-pos-select')) return;
    const row = event.target.closest('.word-result-row');
    if (row) onSaveRowEdits(row);
  });
}

export function saveWordRowEdits(row) {
  if (!row._wordId) return;
  const reading = (row.querySelector('.detail-reading .detail-input')?.textContent ?? '').trim();
  const type = row.querySelector('.detail-pos-select')?.value ?? '';
  const meaning = (row.querySelector('.detail-meaning .detail-input')?.textContent ?? '').trim();
  const exInputs = row.querySelectorAll('.detail-ex .detail-input');
  const exampleJp = (exInputs[0]?.textContent ?? '').trim();
  const exampleEn = (exInputs[1]?.textContent ?? '').trim();
  const targetEl = row.querySelector('.drill-target-val');
  const target = targetEl ? (parseInt(targetEl.dataset.target, 10) || 0) : 0;
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

export async function adjustWordTarget(event, wordId, delta, btn) {
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

function showFieldError(el, footerEl, closeButtonId, state, msg) {
  el.classList.remove('detail-input--flash-error');
  void el.offsetWidth;
  el.classList.add('detail-input--flash-error');
  el.addEventListener('animationend', () => el.classList.remove('detail-input--flash-error'), { once: true });

  let errEl = footerEl.querySelector('.footer-field-error');
  if (!errEl) {
    errEl = document.createElement('span');
    errEl.className = 'footer-field-error';
    const closeBtn = document.getElementById(closeButtonId);
    footerEl.insertBefore(errEl, closeBtn);
  }
  errEl.textContent = msg;
  clearTimeout(state.fieldErrorTimer);
  state.fieldErrorTimer = setTimeout(() => errEl.remove(), 3000);
}

function enforceFieldLanguage(el, footerEl, closeButtonId, state) {
  const kind = getFieldLanguageKind(el);
  const filter = getFieldLanguageFilter(kind);
  if (!filter) return;
  const original = el.textContent;
  const filtered = sanitizeFieldInput(original, kind);
  if (filtered === original) return;
  const selection = window.getSelection();
  const rawOffset = selection.rangeCount > 0 ? selection.getRangeAt(0).startOffset : 0;
  const removedBefore = rawOffset - filter(original.slice(0, rawOffset)).length;
  const newOffset = Math.max(0, rawOffset - removedBefore);
  el.textContent = filtered;
  if (el.firstChild) {
    const range = document.createRange();
    range.setStart(el.firstChild, Math.min(newOffset, filtered.length));
    range.collapse(true);
    selection.removeAllRanges();
    selection.addRange(range);
  }
  showFieldError(el, footerEl, closeButtonId, state, getFieldLanguageErrorMsg(kind));
}
