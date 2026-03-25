const words = lexiconWords;

function updateWordCount() {
  const active = words.filter(w => w.correct < w.target).length;
  document.getElementById('word-count').textContent =
    words.length + ' words (' + active + ' active)';
}
updateWordCount();

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
    '</div></td>' +
    '<td class="cell-reading" data-tooltip="Reading (Pronunciation)">' + w.reading + '</td>' +
    '<td><span class="type-badge" data-tooltip="' + (typeLabels[w.type] || w.type) + '">' + w.type + '</span></td>' +
    '<td class="cell-meaning"><div class="cell-meaning-inner" data-tooltip="Meaning: ' + w.meaning + '">' + w.meaning + '</div></td>' +
    '<td class="cell-correct" data-tooltip="Times answered correctly">' + w.correct + '</td>' +
    '<td class="cell-incorrect" data-tooltip="Times answered incorrectly">' + w.incorrect + '</td>' +
    '<td class="cell-target">' +
      '<div class="target-stepper">' +
        '<button class="btn-target-adj" onclick="adjustTargetInline(event,-4)" data-tooltip="Decrease target">−</button>' +
        '<span data-tooltip="Drills to target remaining">' + w.target + '</span>' +
        '<button class="btn-target-adj" onclick="adjustTargetInline(event,4)" data-tooltip="Increase target">+</button>' +
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

renderTable(getSortedWords('added', 'desc'));

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
  document.getElementById('modal-backdrop').classList.remove('hidden');
}

function closeModal() {
  document.getElementById('modal-backdrop').classList.add('hidden');
}

function handleBackdropClick(event) {
  if (event.target === document.getElementById('modal-backdrop')) closeModal();
}

function saveModal() {
  const w = _modalTrMain._word;
  w.reading   = document.getElementById('edit-reading').value;
  w.type      = document.getElementById('edit-type').value;
  w.meaning   = document.getElementById('edit-meaning').value;
  w.exampleJp = document.getElementById('edit-ex-jp').value;
  w.exampleEn = document.getElementById('edit-ex-en').value;
  w.target    = parseInt(document.getElementById('edit-target').value, 10);
  renderRow(w, _modalTrMain, _modalTrMain._trEx);
  closeModal();
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
  lexTooltip.style.top = (e.clientY - 10) + 'px';
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

function handleAddBackdropClick(event) {
  if (event.target === document.getElementById('add-modal-backdrop')) closeAddModal();
}

document.getElementById('autofill-check').addEventListener('change', function () {
  document.getElementById('ai-model-select').disabled = !this.checked;
});

// --- Progress modal ---
let _progressPhase = 'idle'; // 'loading' | 'done' | 'cancelled'
let _progressCancel = false;
let _progressAdded = [];

document.getElementById('progress-modal-backdrop').addEventListener('click', function (e) {
  if (e.target === this && _progressPhase !== 'loading') closeProgressModal();
});

function closeProgressModal() {
  document.getElementById('progress-modal-backdrop').classList.add('hidden');
  const activeBtn = document.querySelector('.btn-sort--active');
  renderTable(getSortedWords(activeBtn.dataset.sort, activeBtn.dataset.dir || 'desc'));
  updateWordCount();
}

function saveAddModal() {
  const wordList = document.getElementById('add-words-input').value
    .split(/[\s,、。・;:!?()（）「」【】『』\[\]]+/)
    .map(t => t.trim()).filter(t => t.length > 0);

  if (wordList.length === 0) return;

  const useAI = document.getElementById('autofill-check').checked;
  closeAddModal();

  _progressPhase = 'loading';
  _progressCancel = false;
  _progressAdded = [];

  document.getElementById('progress-modal-body').innerHTML = '';
  document.getElementById('progress-modal-backdrop').classList.remove('hidden');
  setProgressStatus('loading', 'Processing\u2026');
  initProgressFooter();

  let i = 0;
  function processNext() {
    if (_progressCancel || i >= wordList.length) {
      if (_progressCancel) {
        _progressPhase = 'cancelled';
        setProgressStatus('cancelled', 'Cancelled \u2014 ' + _progressAdded.length + ' word(s) added before cancel');
      } else {
        _progressPhase = 'done';
        const skipped = wordList.length - _progressAdded.length;
        setProgressStatus('done', _progressAdded.length + ' added, ' + skipped + ' skipped');
      }
      updateProgressFooter();
      return;
    }

    const word = wordList[i++];
    const isDup = words.some(w => w.word === word);

    if (isDup) {
      appendProgressResult({ word, added: false, reason: 'duplicate' });
    } else {
      const fakeAI = useAI ? {
        reading: 'よみがな',
        part_of_speech: 'noun',
        meaning: 'example meaning (AI placeholder)',
        example_jp: 'これは例文です。',
        example_en: 'This is an example sentence.',
      } : {};
      words.push({
        word,
        reading:   fakeAI.reading        || '',
        type:      fakeAI.part_of_speech || 'noun',
        meaning:   fakeAI.meaning        || '',
        exampleJp: fakeAI.example_jp     || '',
        exampleEn: fakeAI.example_en     || '',
        correct: 0, incorrect: 0, target: 3,
        createdAt: new Date().toISOString(), lastDrilled: null,
      });
      _progressAdded.push(word);
      appendProgressResult({ word, added: true, ...fakeAI });
    }

    updateProgressFooter();
    setTimeout(processNext, 280);
  }

  setTimeout(processNext, 180);
}

function appendProgressResult(data) {
  const row = document.createElement('div');
  row.className = 'word-result-row ' + (data.added ? 'result-added' : 'result-skipped');

  const badge = data.added
    ? '<span class="result-badge badge-added">added</span>'
    : '<span class="result-badge badge-skipped">' + esc(data.reason) + '</span>';

  let details = '';
  if (data.reading || data.part_of_speech || data.meaning || data.example_jp) {
    const items = [];
    if (data.reading)        items.push(detailItem('reading', data.reading));
    if (data.part_of_speech) items.push(detailItem('pos', data.part_of_speech));
    if (data.meaning)        items.push(detailItem('meaning', data.meaning));
    if (data.example_jp)     items.push(detailItem('ex.', data.example_jp + (data.example_en ? '  ' + data.example_en : '')));
    details = '<div class="word-result-details">' + items.join('') + '</div>';
  }

  row.innerHTML =
    '<div class="word-result-main"><span class="result-word">' + esc(data.word) + '</span>' + badge + '</div>' +
    details;
  document.getElementById('progress-modal-body').appendChild(row);
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
    _progressCancel = true;
  };
  document.getElementById('btn-prog-remove').onclick = function () {
    const toRemove = _progressAdded.slice();
    toRemove.forEach(w => {
      const idx = words.findIndex(x => x.word === w);
      if (idx !== -1) words.splice(idx, 1);
    });
    _progressAdded = [];
    document.querySelectorAll('#progress-modal-body .badge-added').forEach(badge => {
      badge.className = 'result-badge badge-removed';
      badge.textContent = 'removed';
    });
    setProgressStatus('done', 'Removed \u2014 0 words added from this batch');
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
