import { setWordRowImage } from './lexicon-add-modal.js';

export async function streamBatchAdd({ rawWords, signal, onUpdated, onDone, onRow }) {
  const form = new FormData();
  form.append('words', rawWords);
  form.append('autofill', 'off');

  const res = await fetch('/admin/words/batch', {
    method: 'POST',
    body: form,
    signal,
  });
  if (!res.ok) throw new Error(await res.text());
  if (res.status === 204 || !res.body) {
    onDone?.({ done: true });
    return;
  }

  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = '';
  let completed = false;
  const handleLine = line => {
    if (!line.startsWith('data: ')) return;
    const data = JSON.parse(line.slice(6));
    if (data.updated) {
      onUpdated?.(data);
      return;
    }
    if (data.done) {
      completed = true;
      onDone?.(data);
      return;
    }
    onRow?.(data);
  };
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += dec.decode(value, { stream: true });
    const lines = buf.split('\n');
    buf = lines.pop();
    for (const line of lines) {
      handleLine(line);
      if (completed) {
        return;
      }
    }
  }
  buf += dec.decode();
  if (buf.trim()) handleLine(buf.trim());
  if (!completed) onDone?.({ done: true });
}

export function applyWordRowDetailsUpdate({ containerEl, data, buildWordResultDetails, typeLabels, getWordBtnLabel, generateType, onBusyResolved }) {
  const row = Array.from(containerEl.children).find(el => el._resolvedWord === data.word);
  if (!row) return null;
  row.querySelector('.word-result-details').outerHTML = buildWordResultDetails(row._resolvedWord, data, typeLabels);
  const genBtn = row.querySelector('.btn-generate');
  if (genBtn && genBtn.classList.contains('btn-generate--busy') && !genBtn._generateAbort) {
    genBtn.classList.remove('btn-generate--busy');
    genBtn.innerHTML = getWordBtnLabel(generateType);
    onBusyResolved?.();
  }
  return row;
}

export async function generateWordAutofillRequest({ event, wordId, word, btn, aiModel, state, renderStatus, getWordBtnLabel, generateType, onWordUpdated }) {
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
    const res = await fetch('/api/words/' + wordId + '/autofill', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ word, ai_model: aiModel }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    data.word = word;
    onWordUpdated(data);
  } finally {
    if (btn._generateAbort === abort) {
      btn._generateAbort = null;
      if (btn.classList.contains('btn-generate--busy')) {
        btn.classList.remove('btn-generate--busy', 'btn-generate--cancellable');
        btn.innerHTML = getWordBtnLabel(generateType);
        state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
        renderStatus();
      }
    }
  }
}

export async function generateWordImageRequest({
  event,
  wordId,
  word,
  btn,
  aiModel,
  imageSource,
  state,
  renderStatus,
  getWordBtnLabel,
  generateType,
  onImageUpdated,
}) {
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
  btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">finding image…</span><span class="btn-gen-cancel">cancel</span>';
  state.pendingGenerates++;
  renderStatus();
  const meaning = (row?.querySelector('.detail-meaning .detail-input')?.textContent ?? '').trim();
  const prevImageHtml = row?.querySelector('.word-result-image')?.outerHTML ?? null;
  setWordRowImage(row, '', 'loading');
  try {
    const res = await fetch('/api/words/' + wordId + '/find-image', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ word, meaning, ai_model: aiModel, image_source: imageSource }),
      signal: abort.signal,
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    if (row?.isConnected) {
      setWordRowImage(row, data.image_path, '', Date.now());
      onImageUpdated?.(wordId, data.image_path, row);
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
        btn.innerHTML = getWordBtnLabel(generateType);
        state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
        renderStatus();
      }
    }
  }
}

export async function generateAllAutofillBatchRequest({ rows, aiModel, state, renderStatus, getWordBtnLabel, generateType, updateWordRowDetails }) {
  const abort = new AbortController();
  const wordItems = [];
  for (const row of rows.filter(Boolean)) {
    if (!row._wordId) continue;
    const btn = row.querySelector('.btn-generate:not(.btn-generate--busy):not([disabled])');
    if (!btn) continue;
    btn._generateAbort = abort;
    btn.classList.add('btn-generate--busy', 'btn-generate--cancellable');
    btn.innerHTML = '<span class="spinner"></span><span class="btn-gen-label">generating…</span><span class="btn-gen-cancel">cancel generation</span>';
    state.pendingGenerates++;
    wordItems.push({ id: row._wordId, word: row._resolvedWord, btn });
  }
  if (wordItems.length === 0) return;
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
    for (const { btn } of wordItems) {
      btn._generateAbort = null;
      btn.classList.remove('btn-generate--cancellable');
      if (btn.classList.contains('btn-generate--busy')) {
        btn.classList.remove('btn-generate--busy');
        btn.innerHTML = getWordBtnLabel(generateType);
        state.pendingGenerates = Math.max(0, state.pendingGenerates - 1);
      }
    }
    renderStatus();
  }
}
