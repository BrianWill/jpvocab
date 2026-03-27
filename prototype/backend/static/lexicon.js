let words = [];
let defaultDrillTarget = 8; // updated from /api/providers at init

function updateWordCount() {
  const active = words.filter(w => w.correct < w.target).length;
  document.getElementById('word-count').textContent =
    words.length + ' words (' + active + ' active)';
}

const typeLabels = {
  'godan-verb':   'Godan verb — Group 1 (五段動詞)',
  'ichidan-verb': 'Ichidan verb — Group 2 (一段動詞)',
  'noun':         'Noun (名詞)',
  'i-adjective':  'い-adjective (い形容詞)',
  'na-adjective': 'な-adjective (な形容詞)',
  'adverb':       'Adverb (副詞)',
  'other':        'Other',
};

function timeAgo(dateStr) {
  const sec = Math.floor((Date.now() - new Date(dateStr)) / 1000);
  const min = Math.floor(sec / 60);
  if (min < 1) return 'just now';
  if (min < 60)   return min + ' minute' + (min === 1 ? '' : 's') + ' ago';
  const hr = Math.floor(min / 60);
  if (hr < 24)    return hr + ' hour' + (hr === 1 ? '' : 's') + ' ago';
  const day = Math.floor(hr / 24);
  if (day < 30)   return day + ' day' + (day === 1 ? '' : 's') + ' ago';
  const mo = Math.floor(day / 30);
  if (mo < 12)    return mo + ' month' + (mo === 1 ? '' : 's') + ' ago';
  const yr = Math.floor(day / 365);
  return yr + ' year' + (yr === 1 ? '' : 's') + ' ago';
}

function fullDateTime(dateStr) {
  return new Date(dateStr).toLocaleString(undefined, {
    year: 'numeric', month: 'long', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

function renderRow(w, trMain, trEx) {
  trMain.innerHTML =
    '<td><div class="cell-word" data-tooltip="Word">' + w.word +
      '<button class="btn-edit" onclick="openEditModal(event)" data-tooltip="Edit word">✎</button>' +
      '<button class="btn-delete" onclick="openDeleteModal(event)" data-tooltip="Delete word">✕</button>' +
    '</div></td>' +
    '<td class="cell-reading" data-tooltip="Reading (Pronunciation)">' + w.reading + '</td>' +
    '<td><span class="type-badge" data-tooltip="' + (typeLabels[w.type] || w.type) + '">' + w.type + '</span></td>' +
    '<td class="cell-meaning"><div class="cell-meaning-inner" data-tooltip="Meaning: ' + w.meaning + '">' + w.meaning + '</div></td>' +
    '<td class="cell-correct" data-tooltip="Times answered correctly">' + w.correct + '</td>' +
    '<td class="cell-incorrect" data-tooltip="Times answered incorrectly">' + w.incorrect + '</td>' +
    '<td class="cell-target" data-tooltip="Remaining drills to target">' +
      '<div class="target-stepper">' +
        '<button class="btn-target-adj" onmousedown="adjustTargetInline(event,-4)">−</button>' +
        '<span>' + w.target + '</span>' +
        '<button class="btn-target-adj" onmousedown="adjustTargetInline(event,4)">+</button>' +
      '</div>' +
    '</td>' +
    '<td></td>';
  trMain._word = w;
  trMain._trEx  = trEx;

  trEx.innerHTML =
    '<td colspan="2" class="cell-date">' +
      '<span class="cell-date-added" data-tooltip="Date added: ' + fullDateTime(w.createdAt) + '">added ' + timeAgo(w.createdAt) + '</span>' +
      '<span class="cell-date-sep"> · </span>' +
      (w.lastDrilled
        ? '<span class="cell-date-drilled" data-tooltip="Last drilled: ' + fullDateTime(w.lastDrilled) + '">drilled ' + timeAgo(w.lastDrilled) + '</span>'
        : '<span class="cell-date-drilled cell-date-never">never drilled</span>') +
    '</td>' +
    '<td colspan="5" class="cell-ex">' +
      '<span class="cell-ex-jp" data-tooltip="Example sentence">' + w.exampleJp + '</span> ' +
      '<span class="cell-ex-en" data-tooltip="Example sentence">' + w.exampleEn + '</span>' +
    '</td>' +
    '<td></td>';
}

function getSortedWords(key, dir) {
  const asc = dir === 'asc';
  const byDate = (a, b, field) => {
    if (!a[field] && !b[field]) return 0;
    if (!a[field]) return asc ? -1 : 1;
    if (!b[field]) return asc ? 1 : -1;
    const diff = new Date(a[field]) - new Date(b[field]);
    return asc ? diff : -diff;
  };
  return [...words].sort((a, b) => {
    switch (key) {
      case 'added':    return byDate(a, b, 'createdAt');
      case 'drilled':  return byDate(a, b, 'lastDrilled');
      case 'correct': {
        const d = asc ? a.correct - b.correct : b.correct - a.correct;
        return d || new Date(b.createdAt) - new Date(a.createdAt);
      }
      case 'incorrect': {
        const d = asc ? a.incorrect - b.incorrect : b.incorrect - a.incorrect;
        return d || new Date(b.createdAt) - new Date(a.createdAt);
      }
      case 'target': {
        const d = asc ? a.target - b.target : b.target - a.target;
        return d || new Date(b.createdAt) - new Date(a.createdAt);
      }
      case 'type': {
        if (a.type < b.type) return -1;
        if (a.type > b.type) return  1;
        if (!a.lastDrilled && !b.lastDrilled) return 0;
        if (!a.lastDrilled) return 1;
        if (!b.lastDrilled) return -1;
        return new Date(b.lastDrilled) - new Date(a.lastDrilled);
      }
      default: return 0;
    }
  });
}

const tbody = document.getElementById('word-tbody');

function renderTable(sortedWords) {
  tbody.innerHTML = '';
  sortedWords.forEach(w => {
    const trMain = document.createElement('tr');
    trMain.className = 'row-main';
    const trEx = document.createElement('tr');
    trEx.className = 'row-example';
    renderRow(w, trMain, trEx);
    tbody.appendChild(trMain);
    tbody.appendChild(trEx);
  });
}

async function reloadWords() {
  words = await fetch('/api/words').then(r => r.json());
  updateWordCount();
}

async function init() {
  const [wordsData, providers] = await Promise.all([
    fetch('/api/words').then(r => r.json()),
    fetch('/api/providers').then(r => r.json()),
  ]);
  words = wordsData;
  if (providers.default_drill_target) defaultDrillTarget = providers.default_drill_target;
  updateWordCount();
  renderTable(getSortedWords('added', 'desc'));
  applyProviderAvailability(providers);
}

function providerSelectTooltip(providers) {
  const lines = [];
  if (!providers.anthropic) lines.push('Anthropic: set ANTHROPIC_API_KEY to enable');
  if (!providers.openai)    lines.push('OpenAI: set OPENAI_API_KEY to enable');
  if (lines.length === 0) return null;
  return lines.join(' · ') + ' — then restart the program';
}

function applyProviderAvailability(providers) {
  _providers = providers;
}

init();

function onBackdropClick(event, closeFn) {
  if (event.target === event.currentTarget) closeFn();
}

// --- Delete modal ---
let _deleteTrMain = null;

function openDeleteModal(event) {
  event.stopPropagation();
  _deleteTrMain = event.target.closest('tr');
  const w = _deleteTrMain._word;
  document.getElementById('delete-modal-label').textContent = w.word;
  document.getElementById('delete-error').classList.add('hidden');
  document.getElementById('btn-delete-confirm').disabled = false;
  document.getElementById('btn-delete-confirm').textContent = 'Delete';
  document.getElementById('delete-modal-backdrop').classList.remove('hidden');
}

function closeDeleteModal() {
  document.getElementById('delete-modal-backdrop').classList.add('hidden');
}


async function confirmDelete() {
  const w   = _deleteTrMain._word;
  const btn = document.getElementById('btn-delete-confirm');
  const errEl = document.getElementById('delete-error');
  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span>';
  errEl.classList.add('hidden');

  try {
    const res = await fetch('/api/words/' + w.id, { method: 'DELETE' });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    words.splice(words.indexOf(w), 1);
    _deleteTrMain._trEx.remove();
    _deleteTrMain.remove();
    updateWordCount();
    closeDeleteModal();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('hidden');
    btn.disabled = false;
    btn.textContent = 'Delete';
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

// --- Edit modal (reuses add-result modal with a single word) ---
function openEditModal(event) {
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
  });

  document.getElementById('add-result-modal-backdrop').classList.remove('hidden');
  initAddResultFooter();
  renderStatus();
}

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    closeAddModal();
    closeDeleteModal();
    if (_addPhase !== 'loading' && _pendingGenerates === 0) closeAddResultModal();
  }
});

// Sort button active state, direction toggle, and sorting
const sortBtns = document.querySelectorAll('.btn-sort');
sortBtns.forEach(btn => {
  btn.addEventListener('click', e => {
    e.stopPropagation();
    const wasActive = btn.classList.contains('btn-sort--active');
    sortBtns.forEach(b => {
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

// --- Tooltip ---
const lexTooltip = document.createElement('div');
lexTooltip.className = 'lex-tooltip';
document.body.appendChild(lexTooltip);

document.addEventListener('mouseover', e => {
  const el = e.target.closest('[data-tooltip]');
  if (!el) { lexTooltip.classList.remove('visible'); return; }
  lexTooltip.textContent = el.dataset.tooltip;
  lexTooltip.classList.add('visible');
});
document.addEventListener('mousemove', e => {
  if (!lexTooltip.classList.contains('visible')) return;
  const x = e.clientX + 14;
  lexTooltip.style.left = (x + lexTooltip.offsetWidth > window.innerWidth)
    ? (e.clientX - lexTooltip.offsetWidth) + 'px'
    : x + 'px';
  lexTooltip.style.top = (e.clientY + 18) + 'px';
});

// --- Add words modal ---
function openAddModal() {
  document.getElementById('add-words-input').value = '';
  document.getElementById('add-modal-backdrop').classList.remove('hidden');
  document.getElementById('add-words-input').focus();
}

function closeAddModal() {
  document.getElementById('add-modal-backdrop').classList.add('hidden');
}


// --- Add-result / edit modal ---
// Used in two ways:
//   1. After "Add words": words stream in via SSE, starting as placeholders and
//      filling in one by one as the server processes them.
//   2. From the edit button (✎) on a lexicon row: opens with a single word,
//      bypassing the streaming machinery entirely (see openEditModal).
// The word row helpers (appendWordRow, saveWordRowEdits, etc.) serve both cases.
let _addPhase = 'idle'; // 'loading' | 'done' | 'cancelled'
let _addedWords = [];
let _skippedCount = 0;
let _pendingGenerates = 0;
let _abortController = null;
let _providers = null;

document.getElementById('add-result-modal-backdrop').addEventListener('click', function (e) {
  if (e.target === this && _addPhase !== 'loading' && _pendingGenerates === 0) closeAddResultModal();
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

async function closeAddResultModal() {
  if (_addPhase === 'loading' || _pendingGenerates > 0) return;
  document.getElementById('add-result-modal-backdrop').classList.add('hidden');
  await reloadWords();
  const activeBtn = document.querySelector('.btn-sort--active');
  renderTable(getSortedWords(activeBtn.dataset.sort, activeBtn.dataset.dir || 'desc'));
}

async function saveAddModal() {
  const wordList = document.getElementById('add-words-input').value
    .split(/[\s,、。・;:!?()（）「」【】『』\[\]]+/)
    .map(t => t.trim()).filter(t => t.length > 0);
  if (wordList.length === 0) return;

  closeAddModal();

  _addPhase = 'loading';
  _addedWords = [];
  _skippedCount = 0;
  _pendingGenerates = 0;
  _abortController = new AbortController();

  const resultBody = document.getElementById('add-result-modal-body');
  resultBody.innerHTML = '';
  const pendingDash = '<span class="detail-dash">\u2014</span>';
  const pendingDetails =
    '<div class="word-result-details">' +
      detailItemRaw('reading', pendingDash, true, 'detail-reading') +
      detailItemRaw('pos',     pendingDash, true, 'detail-pos') +
      detailItemRaw('meaning', pendingDash, true, 'detail-meaning') +
      detailItemRaw('ex.',     pendingDash, true, 'detail-ex') +
    '</div>';
  const pendingDrillDisplay =
    '<span class="word-result-drill">' +
      '<span class="drill-correct" data-tooltip="Times answered correctly">✓ 0</span>' +
      '<span class="target-stepper" data-tooltip="Remaining drills to target">' +
        '<span class="drill-target-label">🎯</span>' +
        '<span class="drill-target-val">' + defaultDrillTarget + '</span>' +
        '<button class="btn-target-adj" disabled>−</button>' +
        '<button class="btn-target-adj" disabled>+</button>' +
      '</span>' +
    '</span>';
  wordList.forEach(word => {
    const row = document.createElement('div');
    row.className = 'word-result-row word-result-pending';
    row._pendingWord = word;
    row.innerHTML =
      '<div class="word-result-main">' +
        '<span class="result-word">' + esc(word) + '</span>' +
        '<span class="word-pending-badge"><span class="spinner"></span></span>' +
        pendingDrillDisplay +
      '</div>' +
      pendingDetails;
    resultBody.appendChild(row);
  });
  document.getElementById('add-result-modal-backdrop').classList.remove('hidden');
  renderStatus();
  initAddResultFooter();

  const form = new FormData();
  form.append('words', wordList.join('\n'));
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

function sortWordRows() {
  const body = document.getElementById('add-result-modal-body');
  const rows = Array.from(body.children);
  rows.sort((a, b) => {
    const aLexicon = a.dataset.reason === 'already in lexicon' ? 0 : 1;
    const bLexicon = b.dataset.reason === 'already in lexicon' ? 0 : 1;
    return aLexicon - bLexicon;
  });
  rows.forEach(r => body.appendChild(r));
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
      ' data-word="' + esc(data.word) + '" onmousedown="removeWordRow(event,this)">✕</button>';
  const hasProviders = _providers && (_providers.anthropic || _providers.openai);
  const generateBtn = data.word_id
    ? '<button class="btn-generate"' +
        (hasProviders ? '' : ' disabled') +
        ' data-tooltip="Uses an AI API request to get the word\'s reading, part-of-speech, meaning, and an example sentence"' +
        ' onmousedown="generateWordAutofill(event,' + data.word_id + ',\'' + esc(data.word) + '\',this)">generate</button>'
    : '';
  let inlineExtra;
  if (data.word_id) {
    const correct   = data.drill_count     ?? 0;
    const incorrect = data.drill_incorrect ?? 0;
    const target    = data.drill_target    ?? 0;
    inlineExtra =
      '<span class="word-result-drill">' +
        generateBtn +
        removeBtn +
        '<span class="drill-correct" data-tooltip="Times answered correctly">✓ ' + correct + '</span>' +
        '<span class="drill-incorrect" data-tooltip="Times answered incorrectly">✗ ' + incorrect + '</span>' +
        '<span class="target-stepper" data-tooltip="Remaining drills to target">' +
          '<span class="drill-target-label">🎯</span>' +
          '<span class="drill-target-val" data-target="' + target + '">' + target + '</span>' +
          '<button class="btn-target-adj" onmousedown="adjustWordTarget(event,' + data.word_id + ',-1,this)">−</button>' +
          '<button class="btn-target-adj" onmousedown="adjustWordTarget(event,' + data.word_id + ',1,this)">+</button>' +
        '</span>' +
      '</span>';
  } else {
    inlineExtra = '<span class="word-result-drill">' + removeBtn + '</span>';
  }

  const details =
    '<div class="word-result-details">' +
      detailItemInput('reading', data.reading,        'detail-reading') +
      detailItemPosSelect(data.part_of_speech) +
      detailItemInput('meaning', data.meaning,        'detail-meaning') +
      detailItemExInput(data.example_jp, data.example_en) +
    '</div>';

  row.innerHTML =
    '<div class="word-result-main"><span class="result-word">' + esc(data.word) + '</span>' + badge + inlineExtra + '</div>' +
    details;
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

async function removeWordRow(event, btn) {
  const word = btn.dataset.word;
  event.stopPropagation();
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
  renderStatus();
  updateAddResultFooter();
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
  fetch('/api/words/' + row._wordId, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ reading, type, meaning, exampleJp, exampleEn, target }),
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
    const short = label.split(' — ')[0].split(' (')[0];
    return '<option value="' + esc(key) + '"' + (value === key ? ' selected' : '') + '>' + esc(short) + '</option>';
  }).join('');
  return '<span class="detail-item detail-pos">' +
    '<span class="detail-label">pos</span> ' +
    '<select class="detail-pos-select">' + options + '</select>' +
    '</span>';
}

function detailItemInput(label, value, cls) {
  return '<span class="detail-item ' + cls + '">' +
    '<span class="detail-label">' + esc(label) + '</span> ' +
    '<span class="detail-input" contenteditable="true">' + esc(value || '') + '</span>' +
    '</span>';
}

function detailItemExInput(exJp, exEn) {
  return '<span class="detail-item detail-ex">' +
    '<span class="detail-label">ex.</span> ' +
    '<span class="detail-input" contenteditable="true">' + esc(exJp || '') + '</span>' +
    ' <span class="detail-input detail-input--en" contenteditable="true">' + esc(exEn || '') + '</span>' +
    '</span>';
}

function esc(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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

function generateAllAdded() {
  document.querySelectorAll('#add-result-modal-body .result-added .btn-generate:not(.btn-generate--busy):not([disabled])').forEach(btn => {
    btn.dispatchEvent(new MouseEvent('mousedown'));
  });
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
    sel.disabled = busyLock || !(_providers && (_providers.anthropic || _providers.openai));
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
  const hasProviders = _providers && (_providers.anthropic || _providers.openai);
  const actionHtml = _pendingGenerates > 0
    ? '<button class="btn-generate btn-generate--cancel" onmousedown="cancelAllGenerates()">' +
        '<span class="spinner"></span>cancel generation' +
      '</button>'
    : '<button class="btn-generate btn-generate--all"' +
        (_addedWords.length > 0 && hasProviders && _addPhase !== 'loading' ? '' : ' disabled') +
        ' data-tooltip="Uses an AI API request to get the reading, part-of-speech, meaning, and an example sentence for each newly added word"' +
        ' onmousedown="generateAllAdded()">generate all</button>';
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
  updateAddResultFooter();
}

function setModalStatus(type, text) {
  const el = document.getElementById('add-result-modal-status');
  const spinner = type === 'loading' ? '<span class="spinner"></span>' : '';
  el.className = 'modal-status modal-status-' + type;
  el.innerHTML = spinner + '<span>' + esc(text) + '</span>';
}

function initAddResultFooter() {
  const footer = document.getElementById('add-result-modal-footer');
  const hasProviders = _providers && (_providers.anthropic || _providers.openai);
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
    '</select>' +
    (progTip ? '<span class="provider-info-icon" data-tooltip="' + progTip + '">?</span>' : '') +
    '<button id="btn-add-result-close" class="btn-save" style="margin-left:auto">Close</button>';

  if (hasProviders) {
    const sel = document.getElementById('add-result-model-select');
    const first = sel.querySelector('optgroup:not([disabled]) option');
    if (first) sel.value = first.value;
  }

  document.getElementById('btn-add-result-remove').onclick = async function () {
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
    renderStatus();
    await reloadWords();
    updateAddResultFooter();
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
