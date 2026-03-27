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

