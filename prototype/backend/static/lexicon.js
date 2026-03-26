let words = [];

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
      '<button class="btn-edit" onclick="openModal(event)" data-tooltip="Edit word">✎</button>' +
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
  updateWordCount();
  renderTable(getSortedWords('added', 'desc'));
  applyProviderAvailability(providers);
}

function applyProviderAvailability(providers) {
  const select       = document.getElementById('ai-model-select');
  const checkbox     = document.getElementById('autofill-check');
  const msg          = document.getElementById('autofill-msg');
  const anthropicGrp = document.querySelector('#ai-model-select optgroup[label="Anthropic"]');
  const openaiGrp    = document.querySelector('#ai-model-select optgroup[label="OpenAI"]');

  if (!providers.anthropic) {
    anthropicGrp.disabled = true;
    anthropicGrp.label = 'Anthropic — no API key';
  }
  if (!providers.openai) {
    openaiGrp.disabled = true;
    openaiGrp.label = 'OpenAI — no API key';
  }

  if (!providers.anthropic && !providers.openai) {
    checkbox.disabled = true;
    select.disabled = true;
    msg.textContent = '— no AI providers configured';
  } else {
    checkbox.checked = true;
    select.disabled = false;
    const first = select.querySelector('optgroup:not([disabled]) option');
    if (first) select.value = first.value;
  }

  // Apply to edit modal AI select and reroll buttons
  const editSelect       = document.getElementById('edit-ai-model-select');
  const editAnthropicGrp = editSelect.querySelector('optgroup[label="Anthropic"]');
  const editOpenaiGrp    = editSelect.querySelector('optgroup[label="OpenAI"]');
  const btnMeaning       = document.getElementById('btn-reroll-meaning');
  const btnExamples      = document.getElementById('btn-reroll-examples');

  if (!providers.anthropic) {
    editAnthropicGrp.disabled = true;
    editAnthropicGrp.label = 'Anthropic — no API key';
  }
  if (!providers.openai) {
    editOpenaiGrp.disabled = true;
    editOpenaiGrp.label = 'OpenAI — no API key';
  }

  if (!providers.anthropic && !providers.openai) {
    editSelect.disabled = true;
    btnMeaning.disabled = true;
    btnExamples.disabled = true;
    document.getElementById('edit-sidebar-empty').innerHTML =
      '<span class="sidebar-no-providers">No AI providers configured.<br><br>' +
      'Set <code>ANTHROPIC_API_KEY</code> or <code>OPENAI_API_KEY</code> ' +
      'as environment variables and restart the server.</span>';
  } else {
    const firstEdit = editSelect.querySelector('optgroup:not([disabled]) option');
    if (firstEdit) editSelect.value = firstEdit.value;
  }
}

init();

// --- Modal ---
let _modalTrMain = null;

function openModal(event) {
  event.stopPropagation();
  const trMain = event.target.closest('tr');
  _modalTrMain = trMain;
  const w = trMain._word;
  document.getElementById('modal-word-label').textContent = w.word;
  document.getElementById('edit-reading').value  = w.reading;
  document.getElementById('edit-type').value     = w.type;
  document.getElementById('edit-meaning').value  = w.meaning;
  document.getElementById('edit-ex-jp').value    = w.exampleJp;
  document.getElementById('edit-ex-en').value    = w.exampleEn;
  document.getElementById('edit-target').value   = w.target;
  document.getElementById('edit-error').classList.add('hidden');
  document.getElementById('btn-edit-save').textContent = 'Save';
  resetEditSidebar();
  document.getElementById('modal-backdrop').classList.remove('hidden');
}

function closeModal() {
  document.getElementById('modal-backdrop').classList.add('hidden');
}

function resetEditSidebar() {
  document.getElementById('edit-sidebar-empty').classList.remove('hidden');
  document.getElementById('edit-sidebar-content').classList.add('hidden');
}

function showEditSidebarLoading(label) {
  document.getElementById('edit-sidebar-empty').classList.add('hidden');
  const contentEl = document.getElementById('edit-sidebar-content');
  contentEl.classList.remove('hidden');
  document.getElementById('edit-sidebar-label').textContent = label;
  document.getElementById('edit-alternatives-list').innerHTML =
    '<div class="edit-sidebar-loading"><span class="spinner"></span> Generating\u2026</div>';
}

function showEditSidebarError(message) {
  document.getElementById('edit-alternatives-list').innerHTML =
    '<div class="edit-sidebar-loading edit-sidebar-error">' + esc(message) + '</div>';
}

async function doRerollMeaning() {
  const w = _modalTrMain._word;
  const currentMeaning = document.getElementById('edit-meaning').value;
  const aiModel = document.getElementById('edit-ai-model-select').value;
  showEditSidebarLoading('Alternative meanings');
  try {
    const res = await fetch('/api/words/' + w.id + '/reroll-meaning', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ word: w.word, current: currentMeaning, ai_model: aiModel }),
    });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    const data = await res.json();
    renderMeaningAlternatives(data.alternatives);
  } catch (err) {
    showEditSidebarError(err.message);
  }
}

async function doRerollExamples() {
  const w = _modalTrMain._word;
  const aiModel = document.getElementById('edit-ai-model-select').value;
  showEditSidebarLoading('Alternative examples');
  try {
    const res = await fetch('/api/words/' + w.id + '/reroll-examples', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ word: w.word, ai_model: aiModel }),
    });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    const data = await res.json();
    renderExampleAlternatives(data.alternatives);
  } catch (err) {
    showEditSidebarError(err.message);
  }
}

function renderMeaningAlternatives(alternatives) {
  const list = document.getElementById('edit-alternatives-list');
  list.innerHTML = '';
  alternatives.forEach(alt => {
    const item = document.createElement('div');
    item.className = 'alt-item';
    item.innerHTML =
      '<span class="alt-text">' + esc(alt) + '</span>' +
      '<button class="btn-replace">Replace</button>';
    item.querySelector('.btn-replace').onclick = () => {
      document.getElementById('edit-meaning').value = alt;
    };
    list.appendChild(item);
  });
}

function renderExampleAlternatives(alternatives) {
  const list = document.getElementById('edit-alternatives-list');
  list.innerHTML = '';
  alternatives.forEach(alt => {
    const item = document.createElement('div');
    item.className = 'alt-item';
    item.innerHTML =
      '<span class="alt-text">' + esc(alt.jp) + '</span>' +
      '<span class="alt-text-sub">' + esc(alt.en) + '</span>' +
      '<button class="btn-replace">Replace</button>';
    item.querySelector('.btn-replace').onclick = () => {
      document.getElementById('edit-ex-jp').value = alt.jp;
      document.getElementById('edit-ex-en').value = alt.en;
    };
    list.appendChild(item);
  });
}

function onBackdropClick(event, closeFn) {
  if (event.target === event.currentTarget) closeFn();
}

async function saveModal() {
  const w         = _modalTrMain._word;
  const reading   = document.getElementById('edit-reading').value;
  const type      = document.getElementById('edit-type').value;
  const meaning   = document.getElementById('edit-meaning').value;
  const exampleJp = document.getElementById('edit-ex-jp').value;
  const exampleEn = document.getElementById('edit-ex-en').value;
  const target    = parseInt(document.getElementById('edit-target').value, 10);

  const btn   = document.getElementById('btn-edit-save');
  const errEl = document.getElementById('edit-error');
  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span>';
  errEl.classList.add('hidden');

  try {
    const res = await fetch('/api/words/' + w.id, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ reading, type, meaning, exampleJp, exampleEn, target }),
    });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    w.reading = reading; w.type = type; w.meaning = meaning;
    w.exampleJp = exampleJp; w.exampleEn = exampleEn; w.target = target;
    renderRow(w, _modalTrMain, _modalTrMain._trEx);
    updateWordCount();
    closeModal();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('hidden');
    btn.disabled = false;
    btn.textContent = 'Save';
  }
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
  w.target = Math.max(w.correct, w.target + delta);
  renderRow(w, trMain, trMain._trEx);
  updateWordCount();
}

const STEP_INTERVAL = 230;
let _stepTimer = null;
function startStep(fn, ...args) { fn(...args); _stepTimer = setInterval(() => fn(...args), STEP_INTERVAL); }
function stopStep() { clearInterval(_stepTimer); _stepTimer = null; }

function capTargetInput(input) {
  if (input.value.length > 2) input.value = input.value.slice(0, 2);
}

function adjustTarget(delta) {
  const input = document.getElementById('edit-target');
  input.value = Math.min(99, Math.max(0, (parseInt(input.value, 10) || 0) + delta));
}

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    closeModal();
    closeAddModal();
    closeDeleteModal();
    if (_progressPhase !== 'loading') closeProgressModal();
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


document.getElementById('autofill-check').addEventListener('change', function () {
  document.getElementById('ai-model-select').disabled = !this.checked;
});

// --- Progress modal ---
let _progressPhase = 'idle'; // 'loading' | 'done' | 'cancelled'
let _progressAdded = [];
let _progressTotal = 0;
let _abortController = null;

document.getElementById('progress-modal-backdrop').addEventListener('click', function (e) {
  if (e.target === this && _progressPhase !== 'loading') closeProgressModal();
});

function closeProgressModal() {
  document.getElementById('progress-modal-backdrop').classList.add('hidden');
  const activeBtn = document.querySelector('.btn-sort--active');
  renderTable(getSortedWords(activeBtn.dataset.sort, activeBtn.dataset.dir || 'desc'));
  updateWordCount();
}

async function saveAddModal() {
  const wordList = document.getElementById('add-words-input').value
    .split(/[\s,、。・;:!?()（）「」【】『』\[\]]+/)
    .map(t => t.trim()).filter(t => t.length > 0);
  if (wordList.length === 0) return;

  const useAI   = document.getElementById('autofill-check').checked;
  const aiModel = document.getElementById('ai-model-select').value;
  closeAddModal();

  _progressPhase = 'loading';
  _progressAdded = [];
  _progressTotal = wordList.length;
  _abortController = new AbortController();

  document.getElementById('progress-modal-body').innerHTML = '';
  document.getElementById('progress-modal-backdrop').classList.remove('hidden');
  setProgressStatus('loading', 'Processing\u2026');
  initProgressFooter();

  const form = new FormData();
  form.append('words', wordList.join('\n'));
  form.append('autofill', useAI ? 'on' : 'off');
  form.append('ai_model', aiModel);

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
        if (data.done) {
          _progressPhase = 'done';
          const skipped = _progressTotal - _progressAdded.length;
          const doneEl = document.getElementById('progress-modal-status');
          doneEl.className = 'modal-status modal-status-done';
          doneEl.innerHTML = '<span>' + _progressAdded.length + ' added' +
            (skipped > 0 ? ', <span class="status-skipped">' + skipped + ' skipped</span>' : '') +
            '</span>';
          await reloadWords();
          updateProgressFooter();
          return;
        }
        if (data.added) _progressAdded.push(data.word);
        appendProgressResult(data);
        updateProgressFooter();
      }
    }
  } catch (err) {
    if (err.name === 'AbortError') {
      _progressPhase = 'cancelled';
      setProgressStatus('cancelled', 'Cancelled \u2014 ' + _progressAdded.length + ' word(s) added before cancel');
    } else {
      _progressPhase = 'done';
      setProgressStatus('done', 'Error: ' + err.message);
    }
    await reloadWords();
    updateProgressFooter();
  }
}

function appendProgressResult(data) {
  const row = document.createElement('div');
  row.className = 'word-result-row ' + (data.added ? 'result-added' : 'result-skipped');

  const badge = data.added
    ? '<span class="result-badge badge-added">added</span>'
    : '<span class="result-badge badge-skipped">' + esc(data.reason) + '</span>';

  let inlineExtra = '';
  let details = '';
  if (data.reason === 'already in lexicon' && data.word_id) {
    const correct   = data.drill_count  ?? 0;
    const target    = data.drill_target ?? 0;
    const remaining = target - correct;
    inlineExtra =
      '<span class="word-result-drill">' +
        '<span class="drill-correct" data-tooltip="Times answered correctly">✓ ' + correct + '</span>' +
        '<span class="target-stepper" data-tooltip="Remaining drills to target">' +
          '<span class="drill-target-label">🎯</span>' +
          '<button class="btn-target-adj" onmousedown="adjustProgressTarget(event,' + data.word_id + ',-1,this)">−</button>' +
          '<span class="drill-target-val" data-target="' + target + '">' + target + '</span>' +
          '<button class="btn-target-adj" onmousedown="adjustProgressTarget(event,' + data.word_id + ',1,this)">+</button>' +
        '</span>' +
      '</span>';
  } else if (data.reading || data.part_of_speech || data.meaning || data.example_jp) {
    const items = [];
    if (data.reading)        items.push(detailItem('reading', data.reading));
    if (data.part_of_speech) items.push(detailItem('pos', data.part_of_speech));
    if (data.meaning)        items.push(detailItem('meaning', data.meaning));
    if (data.example_jp)     items.push(detailItem('ex.', data.example_jp + (data.example_en ? '  ' + data.example_en : '')));
    details = '<div class="word-result-details">' + items.join('') + '</div>';
  }

  row.innerHTML =
    '<div class="word-result-main"><span class="result-word">' + esc(data.word) + '</span>' + badge + inlineExtra + '</div>' +
    details;
  document.getElementById('progress-modal-body').appendChild(row);
}

async function adjustProgressTarget(event, wordId, delta, btn) {
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

function detailItem(label, text) {
  return '<span class="detail-item"><span class="detail-label">' + esc(label) + '</span> ' + esc(text) + '</span>';
}

function esc(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function setProgressStatus(type, text) {
  const el = document.getElementById('progress-modal-status');
  const spinner = type === 'loading' ? '<span class="spinner"></span>' : '';
  el.className = 'modal-status modal-status-' + type;
  el.innerHTML = spinner + '<span>' + esc(text) + '</span>';
}

function initProgressFooter() {
  const footer = document.getElementById('progress-modal-footer');
  footer.innerHTML =
    '<button id="btn-prog-cancel" class="btn-cancel">Cancel request</button>' +
    '<button id="btn-prog-remove" class="btn-danger">Remove added words</button>' +
    '<button id="btn-prog-close" class="btn-save">Close</button>';

  document.getElementById('btn-prog-cancel').onclick = function () {
    _abortController.abort();
  };
  document.getElementById('btn-prog-remove').onclick = async function () {
    const toRemove = _progressAdded.slice();
    await fetch('/admin/words/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ words: toRemove }),
    });
    _progressAdded = [];
    document.querySelectorAll('#progress-modal-body .badge-added').forEach(badge => {
      badge.className = 'result-badge badge-removed';
      badge.textContent = 'removed';
    });
    setProgressStatus('done', 'Removed \u2014 0 words added from this batch');
    await reloadWords();
    updateProgressFooter();
  };
  document.getElementById('btn-prog-close').onclick = closeProgressModal;
  updateProgressFooter();
}

function updateProgressFooter() {
  const btnCancel = document.getElementById('btn-prog-cancel');
  const btnRemove = document.getElementById('btn-prog-remove');
  const btnClose  = document.getElementById('btn-prog-close');
  if (!btnCancel) return;
  btnCancel.disabled = _progressPhase !== 'loading';
  btnRemove.disabled = _progressAdded.length === 0 || _progressPhase === 'loading';
  btnRemove.textContent = _progressAdded.length > 0
    ? 'Remove the ' + _progressAdded.length + ' added word' + (_progressAdded.length === 1 ? '' : 's')
    : 'Remove added words';
  btnClose.disabled = _progressPhase === 'loading';
}
