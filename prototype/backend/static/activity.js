let TODAY, HISTORY_START, activityData, stats;

// ── Stats utilities ───────────────────────────────────────────────────────────

// Returns average count of `field` per day over the given window.
// days = number of calendar days ending today; null = all time (from HISTORY_START).
// Days with no entry in activityData count as zero — the denominator is the full window.
function computeAvg(field, days) {
  let startStr, denom;
  if (days === null) {
    startStr = HISTORY_START;
    const ms = new Date(TODAY + 'T00:00:00') - new Date(HISTORY_START + 'T00:00:00');
    denom = Math.round(ms / 86400000) + 1;
  } else {
    const start = addDays(new Date(TODAY + 'T00:00:00'), -(days - 1));
    startStr = toDateStr(start);
    denom = days;
  }
  let total = 0;
  for (const [date, data] of Object.entries(activityData)) {
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
let weeksLoaded = 0;

function appendWeeks(count) {
  const cal = document.getElementById('calendar');
  const sun = weekSunday(TODAY);
  let added = 0;
  let exhausted = false;

  while (added < count) {
    const start = addDays(sun, -weeksLoaded * 7);
    if (toDateStr(start) < HISTORY_START) { exhausted = true; break; }
    const weekDays = Array.from({length: 7}, (_, j) => toDateStr(addDays(start, j)));
    const row = document.createElement('div');
    row.className = 'week-row';
    weekDays.forEach(dateStr => row.appendChild(buildDayCell(dateStr)));
    cal.appendChild(row);
    weeksLoaded++;
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
  const el = document.getElementById('stats-section');

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

  el.innerHTML = `
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
  const labelsEl = document.getElementById('day-labels');
  ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'].forEach(name => {
    const div = document.createElement('div');
    div.className = 'day-label';
    div.textContent = name;
    labelsEl.appendChild(div);
  });

  appendWeeks(INITIAL_WEEKS);

  const wrap = document.querySelector('.calendar-wrap');

  const endBar = document.createElement('div');
  endBar.className = 'calendar-end hidden';
  endBar.textContent = 'Beginning of history';

  wrap.appendChild(endBar);

  const activityBody = document.querySelector('.activity-body');
  let exhausted = false;

  activityBody.addEventListener('scroll', () => {
    if (exhausted) return;
    const { scrollTop, scrollHeight, clientHeight } = activityBody;
    if (scrollTop + clientHeight >= scrollHeight - 200) {
      exhausted = appendWeeks(LOAD_BATCH);
      if (exhausted) endBar.classList.remove('hidden');
    }
  });
}

function buildDayCell(dateStr) {
  const cell = document.createElement('div');
  cell.className = 'day-cell';

  const isFuture = dateStr > TODAY;
  const isToday = dateStr === TODAY;
  const data = activityData[dateStr];

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
  const data = activityData[dateStr];
  document.getElementById('day-modal-title').textContent = formatDateFull(dateStr);
  const body = document.getElementById('day-modal-body');
  body.innerHTML = '';

  if (data.drilled.length) {
    const knew = data.drilled.filter(e => e.knew).length;
    const missed = data.drilled.length - knew;
    const note = missed > 0 ? '✗ = wrong at least once this day (may also have been answered correctly the same day)' : null;
    body.appendChild(buildSection(
      'Drilled — <span class="drilled-knew-count">' + knew + ' ✓</span>  <span class="drilled-missed-count">' + missed + ' ✗</span>',
      data.drilled,
      'drilled',
      note
    ));
  }
  if (data.added.length)   body.appendChild(buildSection('Added',   data.added,   'added'));
  if (data.cleared.length) body.appendChild(buildSection('Cleared', data.cleared, 'cleared'));

  document.getElementById('day-modal-backdrop').classList.remove('hidden');
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
    const item = document.createElement('div');
    item.className = 'day-word-item';
    if (type === 'drilled') item.classList.add(entry.knew ? 'knew' : 'missed');
    item.innerHTML =
      '<span class="day-word-jp">' + entry.word + '</span>' +
      '<span class="day-word-reading">' + entry.reading + '</span>' +
      '<span class="day-word-meaning">' + entry.meaning + '</span>';
    list.appendChild(item);
  });

  section.appendChild(list);
  return section;
}

function closeDayModal() {
  document.getElementById('day-modal-backdrop').classList.add('hidden');
}

function handleDayBackdropClick(e) {
  if (e.target === document.getElementById('day-modal-backdrop')) closeDayModal();
}

document.addEventListener('keydown', e => { if (e.key === 'Escape') closeDayModal(); });

// --- Static element event listeners ---
const dayModalBackdrop = document.getElementById('day-modal-backdrop');
dayModalBackdrop.addEventListener('click', handleDayBackdropClick);
dayModalBackdrop.querySelector('.modal-close').addEventListener('click', closeDayModal);

// ── Tooltip ───────────────────────────────────────────────────────────────────

const actTooltip = document.createElement('div');
actTooltip.className = 'lex-tooltip';
document.body.appendChild(actTooltip);

document.addEventListener('mouseover', e => {
  const target = e.target.closest('[data-tooltip]');
  if (!target) return;
  actTooltip.textContent = target.dataset.tooltip;
  actTooltip.classList.add('visible');
});

document.addEventListener('mousemove', e => {
  if (!actTooltip.classList.contains('visible')) return;
  const x = e.clientX + 14;
  actTooltip.style.top  = (e.clientY + 14) + 'px';
  actTooltip.style.left = (x + actTooltip.offsetWidth > window.innerWidth)
    ? (e.clientX - actTooltip.offsetWidth) + 'px'
    : x + 'px';
});

document.addEventListener('mouseout', e => {
  if (!e.target.closest('[data-tooltip]')) return;
  actTooltip.classList.remove('visible');
});

async function init() {
  const [statsRes, calRes] = await Promise.all([
    fetch('/api/activity/stats'),
    fetch('/api/activity/calendar'),
  ]);
  stats = await statsRes.json();
  const cal = await calRes.json();
  TODAY = cal.today;
  HISTORY_START = cal.historyStart;
  activityData = cal.days;
  renderStats();
  renderCalendar();
}

init();
