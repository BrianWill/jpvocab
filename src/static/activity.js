import { populateWordTooltip, positionAnchoredWordTooltip, playJapaneseText, WORD_TTS_RATE } from './common.js';
import { renderReading } from './lexicon-utils.js';

const els = {
  activityBody: document.querySelector('.activity-body'),
  calendar: document.getElementById('calendar'),
  calendarWrap: document.querySelector('.calendar-wrap'),
  dayLabels: document.getElementById('day-labels'),
  dayModalBackdrop: document.getElementById('day-modal-backdrop'),
  dayModalBody: document.getElementById('day-modal-body'),
  dayModalTitle: document.getElementById('day-modal-title'),
  statsSection: document.getElementById('stats-section'),
  wordTooltip: document.getElementById('activity-word-tooltip'),
};
els.dayModal = els.dayModalBackdrop.querySelector('.modal');
els.dayModalCloseBtn = els.dayModalBackdrop.querySelector('.modal-close');

const state = {
  activityData: null,
  historyStart: null,
  kanjiMap: {},
  stats: null,
  today: null,
  weeksLoaded: 0,
  wordMap: {},
};

// ── Stats utilities ───────────────────────────────────────────────────────────

// Returns average count of `field` per day over the given window.
// days = number of calendar days ending today; null = all time (from historyStart).
// Days with no entry in activityData count as zero — the denominator is the full window.
function computeAvg(field, days) {
  let startStr, denom;
  if (days === null) {
    startStr = state.historyStart;
    const ms = new Date(state.today + 'T00:00:00') - new Date(state.historyStart + 'T00:00:00');
    denom = Math.round(ms / 86400000) + 1;
  } else {
    const start = addDays(new Date(state.today + 'T00:00:00'), -(days - 1));
    startStr = toDateStr(start);
    denom = days;
  }
  let total = 0;
  for (const [date, data] of Object.entries(state.activityData)) {
    if (date < startStr) continue;
    total += data[field].length;
  }
  return (total / denom).toFixed(1);
}

// ── Calendar utilities ────────────────────────────────────────────────────────

function weekSunday(dateStr) {
  const d = new Date(dateStr + 'T00:00:00');
  d.setDate(d.getDate() - d.getDay()); // getDay(): 0=Sun, 1=Mon, …
  return d;
}

function toDateStr(d) { return d.toISOString().slice(0, 10); }

function addDays(d, n) {
  const r = new Date(d);
  r.setDate(r.getDate() + n);
  return r;
}

const INITIAL_WEEKS = 5;
const LOAD_BATCH = 4;

function appendWeeks(count) {
  const sun = weekSunday(state.today);
  let added = 0;
  let exhausted = false;

  while (added < count) {
    const start = addDays(sun, -state.weeksLoaded * 7);
    if (toDateStr(start) < state.historyStart) { exhausted = true; break; }
    const weekDays = Array.from({length: 7}, (_, j) => toDateStr(addDays(start, j)));
    const row = document.createElement('div');
    row.className = 'week-row';
    weekDays.forEach(dateStr => row.appendChild(buildDayCell(dateStr)));
    els.calendar.appendChild(row);
    state.weeksLoaded++;
    added++;
  }

  return exhausted;
}

function formatDateFull(dateStr) {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('en-GB', {
    weekday: 'long', day: 'numeric', month: 'long', year: 'numeric'
  });
}

function dayLabel(dateStr) {
  const d = new Date(dateStr + 'T00:00:00');
  if (d.getDay() === 0) return d.toLocaleDateString('en-GB', {month: 'short', day: 'numeric'});
  return String(d.getDate());
}

function renderStats() {
  const { stats } = state;
  const wordTotal = stats.drillsCleared + stats.drillsClose + stats.drillsMid + stats.drillsFar;
  const pct = n => (n / wordTotal * 100).toFixed(1);

  const avgDrilled7  = computeAvg('drilled',  7);
  const avgDrilled30 = computeAvg('drilled', 30);
  const avgDrilledAll = computeAvg('drilled', null);
  const avgCleared7  = computeAvg('cleared',  7);
  const avgCleared30 = computeAvg('cleared', 30);
  const avgClearedAll = computeAvg('cleared', null);
  const avgAdded7  = computeAvg('added',  7);
  const avgAdded30 = computeAvg('added', 30);
  const avgAddedAll = computeAvg('added', null);

  els.statsSection.innerHTML = `
    <div class="stat-grid">
      <div class="stat-card" data-tooltip="Total words in your vocabulary">
        <div class="stat-value">${stats.lexiconSize}</div>
        <div class="stat-label">Words in lexicon</div>
      </div>
      <div class="stat-card" data-tooltip="Words whose total drill count is below the word's target count&#10;Eligible to be drawn in drills">
        <div class="stat-value">${stats.activeWords}</div>
        <div class="stat-label">Active words</div>
      </div>
      <div class="stat-card" data-tooltip="Words that have reached their target&#10;drill count at least once">
        <div class="stat-value">${stats.clearedLifetime}</div>
        <div class="stat-label">Cleared (lifetime)</div>
      </div>
      <div class="stat-card" data-tooltip="Words cleared per day (last 30 days)&#10;Over last 7 days: ${avgCleared7}&#10;Over last 30 days: ${avgCleared30}&#10;All time: ${avgClearedAll}">
        <div class="stat-value">${avgCleared30}</div>
        <div class="stat-label">Avg cleared per day</div>
      </div>
      <div class="stat-card" data-tooltip="Words drilled per day (last 30 days)&#10;Over last 7 days: ${avgDrilled7}&#10;Over last 30 days: ${avgDrilled30}&#10;All time: ${avgDrilledAll}">
        <div class="stat-value">${avgDrilled30}</div>
        <div class="stat-label">Avg drilled per day</div>
      </div>
      <div class="stat-card" data-tooltip="Words added per day (last 30 days)&#10;Over last 7 days: ${avgAdded7}&#10;Over last 30 days: ${avgAdded30}&#10;All time: ${avgAddedAll}">
        <div class="stat-value">${avgAdded30}</div>
        <div class="stat-label">Avg added per day</div>
      </div>
    </div>
    <div class="drill-progress">
      <div class="drill-progress-label">Words by drills remaining to target</div>
      <div class="drill-progress-track">
        <div class="drill-progress-seg seg-cleared" style="width:${pct(stats.drillsCleared)}%"></div>
        <div class="drill-progress-seg seg-close"   style="width:${pct(stats.drillsClose)}%"></div>
        <div class="drill-progress-seg seg-mid"     style="width:${pct(stats.drillsMid)}%"></div>
        <div class="drill-progress-seg seg-far"     style="width:${pct(stats.drillsFar)}%"></div>
      </div>
      <div class="drill-progress-legend">
        <span class="legend-item legend-cleared" data-tooltip="Reached target drill count">&#9632; ${stats.drillsCleared} 🍏</span>
        <span class="legend-item legend-close" data-tooltip="4 drills or fewer remaining to target">&#9632; ${stats.drillsClose} 🌳</span>
        <span class="legend-item legend-mid" data-tooltip="8 drills or fewer remaining to target">&#9632; ${stats.drillsMid} 🌿</span>
        <span class="legend-item legend-far" data-tooltip="more than 8 drills remaining to target">&#9632; ${stats.drillsFar} 🌱</span>
      </div>
    </div>`;
}

// ── Rendering ─────────────────────────────────────────────────────────────────

function renderCalendar() {
  ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'].forEach(name => {
    const div = document.createElement('div');
    div.className = 'day-label';
    div.textContent = name;
    els.dayLabels.appendChild(div);
  });

  appendWeeks(INITIAL_WEEKS);

  const endBar = document.createElement('div');
  endBar.className = 'calendar-end hidden';
  endBar.textContent = 'Beginning of history';
  els.calendarWrap.appendChild(endBar);

  let exhausted = false;

  els.activityBody.addEventListener('scroll', () => {
    if (exhausted) return;
    const { scrollTop, scrollHeight, clientHeight } = els.activityBody;
    if (scrollTop + clientHeight >= scrollHeight - 200) {
      exhausted = appendWeeks(LOAD_BATCH);
      if (exhausted) endBar.classList.remove('hidden');
    }
  });
}

function buildDayCell(dateStr) {
  const cell = document.createElement('div');
  cell.className = 'day-cell';

  const isFuture = dateStr > state.today;
  const isToday = dateStr === state.today;
  const data = state.activityData[dateStr];

  if (isFuture) cell.classList.add('future');
  if (isToday) cell.classList.add('today');

  const numEl = document.createElement('div');
  numEl.className = 'day-number';
  numEl.textContent = dayLabel(dateStr);
  cell.appendChild(numEl);

  if (data) {
    cell.classList.add('has-activity');
    cell.addEventListener('click', () => openDayModal(dateStr));

    const badges = document.createElement('div');
    badges.className = 'day-badges';

    if (data.drilled.length) {
      const knew = data.drilled.filter(e => e.knew).length;
      const missed = data.drilled.length - knew;
      if (knew)   badges.appendChild(makeBadge('drilled-knew',   knew   + ' drilled ✓', null));
      if (missed) badges.appendChild(makeBadge('drilled-missed', missed + ' drilled ✗', 'Wrong at least once (may also have been answered correctly the same day)'));
    }
    if (data.added.length)   badges.appendChild(makeBadge('added',   data.added.length + ' added'));
    if (data.cleared.length) badges.appendChild(makeBadge('cleared', data.cleared.length + ' cleared'));

    cell.appendChild(badges);
  }

  return cell;
}

function makeBadge(type, text, tooltip) {
  const div = document.createElement('div');
  div.className = 'day-badge badge-' + type;
  if (tooltip) div.dataset.tooltip = tooltip;
  const dot = document.createElement('span');
  dot.className = 'badge-dot';
  const label = document.createTextNode(text);
  div.appendChild(dot);
  div.appendChild(label);
  return div;
}

// ── Modal ─────────────────────────────────────────────────────────────────────

function openDayModal(dateStr) {
  const data = state.activityData[dateStr];
  els.dayModalTitle.textContent = formatDateFull(dateStr);
  els.dayModalBody.innerHTML = '';

  if (data.drilled.length) {
    const knew = data.drilled.filter(e => e.knew).length;
    const missed = data.drilled.length - knew;
    const note = missed > 0 ? '✗ = wrong at least once this day (may also have been answered correctly the same day)' : null;
    els.dayModalBody.appendChild(buildSection(
      'Drilled — <span class="drilled-knew-count">' + knew + ' ✓</span>  <span class="drilled-missed-count">' + missed + ' ✗</span>',
      data.drilled,
      'drilled',
      note
    ));
  }
  if (data.added.length)   els.dayModalBody.appendChild(buildSection('Added',   data.added,   'added'));
  if (data.cleared.length) els.dayModalBody.appendChild(buildSection('Cleared', data.cleared, 'cleared'));

  els.dayModalBackdrop.classList.remove('hidden');
}

function buildSection(title, words, type, note) {
  const section = document.createElement('div');
  const titleEl = document.createElement('div');
  titleEl.className = 'day-section-title';
  titleEl.innerHTML = title;
  section.appendChild(titleEl);
  if (note) {
    const noteEl = document.createElement('div');
    noteEl.className = 'day-section-note';
    noteEl.textContent = note;
    section.appendChild(noteEl);
  }

  const list = document.createElement('div');
  list.className = 'day-word-list';

  words.forEach(entry => {
    const fullWord = state.wordMap[entry.word] || null;
    const item = document.createElement('div');
    item.className = 'day-word-item';
    if (type === 'drilled') item.classList.add(entry.knew ? 'knew' : 'missed');
    if (fullWord) item.dataset.wordInfo = JSON.stringify(fullWord);
    item.innerHTML =
      '<span class="day-word-jp">' + entry.word + '</span>' +
      '<span class="day-word-reading">' + entry.reading + '</span>' +
      '<span class="day-word-meaning">' + entry.meaning + '</span>';
    item.querySelector('.day-word-jp').addEventListener('click', () =>
      playJapaneseText(entry.word, WORD_TTS_RATE, { preferSynthesis: true, fallbackToBrowserTts: true })
    );
    list.appendChild(item);
  });

  section.appendChild(list);
  return section;
}

function closeDayModal() {
  els.dayModalBackdrop.classList.add('hidden');
}

document.addEventListener('keydown', e => { if (e.key === 'Escape') closeDayModal(); });

els.dayModalBackdrop.addEventListener('click', e => {
  if (e.target === els.dayModalBackdrop) closeDayModal();
});
els.dayModalCloseBtn.addEventListener('click', closeDayModal);

// ── Tooltip ───────────────────────────────────────────────────────────────────

function showWordTooltip(item) {
  if (!item.dataset.wordInfo) return;
  const data = JSON.parse(item.dataset.wordInfo);
  populateWordTooltip(els.wordTooltip, data, state.kanjiMap, renderReading);
  positionWordTooltip(item);
  els.wordTooltip.classList.add('visible');
}

function positionWordTooltip(item) {
  const itemRect = item.getBoundingClientRect();
  const modalRect = els.dayModal.getBoundingClientRect();
  const overlap = 162;
  positionAnchoredWordTooltip(els.wordTooltip, {
    anchorRect: itemRect,
    left: modalRect.right - overlap,
  });
}

document.addEventListener('mouseover', e => {
  const wordItem = e.target.closest('.day-word-item[data-word-info]');
  if (wordItem) showWordTooltip(wordItem);
});

document.addEventListener('mouseout', e => {
  const wordItem = e.target.closest('.day-word-item[data-word-info]');
  if (wordItem && !wordItem.contains(e.relatedTarget)) els.wordTooltip.classList.remove('visible');
});

async function init() {
  const [statsRes, calRes, wordsRes, kanjiRes] = await Promise.all([
    fetch('/api/activity/stats'),
    fetch('/api/activity/calendar'),
    fetch('/api/words'),
    fetch('/api/kanji'),
  ]);
  state.stats = await statsRes.json();
  const cal = await calRes.json();
  const words = await wordsRes.json();
  const kanjiList = await kanjiRes.json();
  state.today = cal.today;
  state.historyStart = cal.historyStart;
  state.activityData = cal.days;
  state.wordMap = {};
  words.forEach(word => { state.wordMap[word.word] = word; });
  state.kanjiMap = {};
  kanjiList.forEach(kanji => { state.kanjiMap[kanji.id] = kanji; });
  renderStats();
  renderCalendar();
}

init();
