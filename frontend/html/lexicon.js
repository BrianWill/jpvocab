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

const tbody = document.getElementById('word-tbody');
words.forEach(w => {
  const trMain = document.createElement('tr');
  trMain.className = 'row-main';
  const trEx = document.createElement('tr');
  trEx.className = 'row-example';
  renderRow(w, trMain, trEx);
  tbody.appendChild(trMain);
  tbody.appendChild(trEx);
});

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

document.addEventListener('keydown', e => { if (e.key === 'Escape') { closeModal(); closeAddModal(); } });

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

function saveAddModal() {
  const lines = document.getElementById('add-words-input').value
    .split(/[\s,、。・;:!?()（）「」【】『』\[\]]+/)
    .map(t => t.trim()).filter(t => t.length > 0);

  const today = new Date().toISOString().slice(0, 10);
  lines.forEach(word => {
    if (words.some(w => w.word === word)) return; // basic duplicate check
    const w = {
      word, reading: '', type: 'noun', meaning: '',
      exampleJp: '', exampleEn: '',
      correct: 0, incorrect: 0, target: 3,
      createdAt: today, lastDrilled: null,
    };
    words.push(w);
    const trMain = document.createElement('tr');
    trMain.className = 'row-main';
    const trEx = document.createElement('tr');
    trEx.className = 'row-example';
    renderRow(w, trMain, trEx);
    tbody.appendChild(trMain);
    tbody.appendChild(trEx);
  });

  updateWordCount();
  closeAddModal();
}
