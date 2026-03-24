const placeholder = '<span class="detail-placeholder">- - -</span>';

const words = drillWords;

const DEFAULT_ROUND_SIZE = 10;

function shuffle(arr) {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

function timeAgo(date) {
  const sec = Math.floor((Date.now() - date) / 1000);
  const min = Math.floor(sec / 60);
  if (min < 1) return 'just now';
  if (min < 60) return min + ' minute' + (min === 1 ? '' : 's') + ' ago';
  const hr = Math.floor(min / 60);
  if (hr < 24) return hr + ' hour' + (hr === 1 ? '' : 's') + ' ago';
  const day = Math.floor(hr / 24);
  return day + ' day' + (day === 1 ? '' : 's') + ' ago';
}

// Session state
let poolSize = words.length;
let roundSize = DEFAULT_ROUND_SIZE;
let pool = shuffle([...words]);
let round = 1;
let redo = [];
let doneCount = 0;
let drillStartedAt = Date.now();

function buildRound() {
  const slots = Math.max(0, roundSize - redo.length);
  const picked = pool.splice(0, slots);
  return [...redo, ...picked];
}

let remaining = buildRound();
let currentWord = remaining[0];

function updateStats() {
  document.getElementById('stat-togo').textContent = (poolSize - doneCount) + ' to go of ' + poolSize;
  document.getElementById('sidebar-title').textContent = 'Round ' + round;
  document.getElementById('header-began').textContent = 'began ' + timeAgo(drillStartedAt);

  const pct = (doneCount / poolSize) * 100;
  document.querySelector('.progress-bar').style.width = pct + '%';
}

function showWord() {
  document.getElementById('prompt-word-jp').textContent = currentWord.word;
  document.getElementById('prompt-example-jp').textContent = currentWord.exampleJp;

  const list = document.getElementById('sidebar-list');
  list.querySelectorAll('.sidebar-item.current').forEach(el => el.classList.remove('current'));
  const item = list.querySelector('[data-id="' + currentWord.word + '"]');
  if (item) item.classList.add('current');
}

function reveal(knew) {
  const answered = currentWord;

  // Update drill state
  remaining.shift();
  if (knew) {
    doneCount++;
  } else {
    redo.push(answered);
  }

  addToSidebar(answered, knew);
  updateStats();

  // Show answered word in last-word card
  document.getElementById('last-word-card').style.display = '';
  const lastWordEl = document.getElementById('last-word-jp');
  lastWordEl.textContent = answered.word;
  lastWordEl.className = 'tooltip-word ' + (knew ? 'knew' : 'missed');
  document.getElementById('last-reading').textContent = answered.reading;
  document.getElementById('last-pos').textContent = answered.type;
  document.getElementById('last-meaning').textContent = answered.meaning;
  document.getElementById('last-example-jp').textContent = answered.exampleJp;
  document.getElementById('last-example-en').textContent = answered.exampleEn;
  renderKanjiInfo(document.getElementById('last-kanji-info'), answered);

  // Advance
  if (remaining.length === 0) {
    if (redo.length > 0 || pool.length > 0) {
      startNextRound();
      return;
    } else {
      document.getElementById('prompt-word-jp').textContent = 'Done!';
      document.getElementById('prompt-example-jp').textContent = 'All words cleared.';
      document.getElementById('action-prompt').style.display = 'none';
      return;
    }
  }

  currentWord = remaining[0];
  showWord();
}

function initSidebar() {
  const list = document.getElementById('sidebar-list');
  remaining.forEach(word => {
    const li = document.createElement('li');
    li.className = 'sidebar-item unseen';
    li.textContent = word.word;
    li.dataset.word = JSON.stringify(word);
    li.dataset.id = word.word;
    list.appendChild(li);
  });
}
initSidebar();

function addToSidebar(word, knew) {
  const list = document.getElementById('sidebar-list');

  // Remove existing entry for this word if present
  const existing = list.querySelector('[data-id="' + word.word + '"]');
  if (existing) existing.remove();

  const li = document.createElement('li');
  li.className = 'sidebar-item ' + (knew ? 'known flash-known' : 'missed flash-missed');
  li.textContent = word.word;
  li.dataset.word = JSON.stringify(word);
  li.dataset.id = word.word;
  li.addEventListener('animationend', () => li.classList.remove('flash-known', 'flash-missed'));

  // Order: missed, then known, then unseen-redo, then unseen
  if (!knew) {
    // Missed: before known, unseen-redo, and unseen
    const firstNonMissed = list.querySelector('.sidebar-item.known, .sidebar-item.unseen-redo, .sidebar-item.unseen');
    if (firstNonMissed) {
      list.insertBefore(li, firstNonMissed);
    } else {
      list.appendChild(li);
    }
  } else {
    // Known: before unseen-redo and unseen
    const firstUnseen = list.querySelector('.sidebar-item.unseen-redo, .sidebar-item.unseen');
    if (firstUnseen) {
      list.insertBefore(li, firstUnseen);
    } else {
      list.appendChild(li);
    }
  }
}

function katakanaToHiragana(str) {
  return str.replace(/[\u30A1-\u30F6]/g, ch => String.fromCharCode(ch.charCodeAt(0) - 0x60));
}

function alignKanjiReadings(wordStr, wordReading) {
  // Returns one entry per character in wordStr; non-kanji characters return null.
  const result = [];
  let pos = 0;
  for (const ch of wordStr) {
    if (!kanjiData[ch]) { pos++; result.push(null); continue; }
    const k = kanjiData[ch];
    const remaining = wordReading.slice(pos);
    const candidates = [
      ...k.on.map(r => ({ match: katakanaToHiragana(r), display: r, type: 'on' })),
      ...k.kun.map(r => ({ match: r,                    display: r, type: 'kun' })),
    ].sort((a, b) => b.match.length - a.match.length);
    const hit = candidates.find(c => remaining.startsWith(c.match));
    if (hit) { pos += hit.match.length; result.push({ ch, reading: hit.display, type: hit.type }); }
    else      { pos++;                  result.push({ ch, reading: null,         type: null });     }
  }
  return result;
}

function renderKanjiInfo(container, word) {
  container.innerHTML = '';
  alignKanjiReadings(word.word, word.reading).forEach(r => {
    if (!r) return;
    const k = kanjiData[r.ch];
    const entry = document.createElement('div');
    entry.className = 'kanji-entry';
    entry.innerHTML =
      '<div class="kanji-char">' + r.ch + '</div>' +
      '<div class="kanji-detail">' +
        '<div class="kanji-readings">' + (r.reading ? '<span class="kanji-' + r.type + '">' + r.reading + '</span>' : '') + '</div>' +
        '<div class="kanji-meanings">' + k.meanings.join(', ') + '</div>' +
      '</div>';
    container.appendChild(entry);
  });
}

// Tooltip hover logic
const tip = document.getElementById('tooltip');
document.getElementById('sidebar-list').addEventListener('mouseover', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item || !item.dataset.word) return;
  const data = JSON.parse(item.dataset.word);
  document.getElementById('tip-word').textContent = data.word;
  document.getElementById('tip-reading').textContent = data.reading;
  document.getElementById('tip-pos').textContent = data.type;
  document.getElementById('tip-meaning').textContent = data.meaning;
  document.getElementById('tip-example').textContent = data.exampleJp || '';
  document.getElementById('tip-example-en').textContent = data.exampleEn || '';
  renderKanjiInfo(document.getElementById('tip-kanji-info'), data);

  const rect = item.getBoundingClientRect();
  const sidebar = document.querySelector('.sidebar');
  tip.style.left = sidebar.getBoundingClientRect().right + 'px';
  tip.style.top = rect.top + 'px';
  tip.style.transform = '';
  tip.classList.add('visible');
});
document.getElementById('sidebar-list').addEventListener('mouseout', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item) return;
  if (!item.contains(e.relatedTarget)) {
    tip.classList.remove('visible');
  }
});


function startNextRound() {
  round++;
  const redoSet = new Set(redo.map(w => w.word));
  remaining = buildRound(); // uses current redo + new picks from pool
  redo = [];
  currentWord = remaining[0];
  updateStats();

  const list = document.getElementById('sidebar-list');
  list.innerHTML = '';

  // Redo words first (red + blurred), then new words (gray + blurred)
  const redoWords = remaining.filter(w => redoSet.has(w.word));
  const newWords = remaining.filter(w => !redoSet.has(w.word));
  [...redoWords, ...newWords].forEach(word => {
    const isRedo = redoSet.has(word.word);
    const li = document.createElement('li');
    li.className = 'sidebar-item ' + (isRedo ? 'unseen-redo' : 'unseen');
    li.textContent = word.word;
    li.dataset.word = JSON.stringify(word);
    li.dataset.id = word.word;
    list.appendChild(li);
  });

  showWord();
}

const STEP_INTERVAL = 230;
let _stepTimer = null;
function startStep(fn, ...args) { fn(...args); _stepTimer = setInterval(() => fn(...args), STEP_INTERVAL); }
function stopStep() { clearInterval(_stepTimer); _stepTimer = null; }

function adjustRestart(id, delta) {
  const input = document.getElementById(id);
  const val = parseInt(input.value, 10) || 5;
  input.value = delta > 0
    ? Math.min(995, Math.floor(val / 5) * 5 + 5)
    : Math.max(5, Math.ceil(val / 5) * 5 - 5);
}

function capRestartInput(input) {
  if (input.value.length > 3) input.value = input.value.slice(0, 3);
  if (input.value === '0') input.value = '1';
}

function openRestartModal() {
  document.getElementById('restart-total-words').value = poolSize;
  document.getElementById('restart-round-size').value = roundSize;
  document.getElementById('restart-modal-backdrop').classList.remove('hidden');
}
function closeRestartModal() {
  document.getElementById('restart-modal-backdrop').classList.add('hidden');
}
function handleRestartBackdropClick(e) {
  if (e.target === document.getElementById('restart-modal-backdrop')) closeRestartModal();
}
function confirmRestart() {
  const total = Math.max(1, Math.min(parseInt(document.getElementById('restart-total-words').value, 10) || poolSize, words.length));
  const rSize = Math.max(1, Math.min(total, parseInt(document.getElementById('restart-round-size').value, 10) || roundSize));
  closeRestartModal();
  restartDrill(total, rSize);
}

function restartDrill(totalWords, newRoundSize) {
  poolSize = totalWords;
  roundSize = newRoundSize;
  pool = shuffle([...words]).slice(0, poolSize);
  round = 1;
  redo = [];
  doneCount = 0;
  drillStartedAt = Date.now();
  remaining = buildRound();
  currentWord = remaining[0];

  document.getElementById('sidebar-list').innerHTML = '';
  document.getElementById('action-prompt').style.display = '';
  document.getElementById('last-word-card').style.display = 'none';
  initSidebar();
  updateStats();
  showWord();
}

// Initialize
showWord();
updateStats();

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') { closeRestartModal(); return; }
  const prompt = document.getElementById('action-prompt');
  if (prompt.style.display === 'none') return;
  if (e.key === 'd' || e.key === 'D') reveal(true);
  if (e.key === 'a' || e.key === 'A') reveal(false);
});
