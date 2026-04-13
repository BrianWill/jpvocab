export function weekSunday(dateStr) {
  const d = new Date(dateStr + 'T00:00:00');
  d.setDate(d.getDate() - d.getDay());
  return d;
}

export function toDateStr(d) {
  return d.toISOString().slice(0, 10);
}

export function addDays(d, n) {
  const r = new Date(d);
  r.setDate(r.getDate() + n);
  return r;
}

export function computeAvgForActivity(activityData, historyStart, today, field, days) {
  let startStr;
  let denom;
  if (days === null) {
    startStr = historyStart;
    const ms = new Date(today + 'T00:00:00') - new Date(historyStart + 'T00:00:00');
    denom = Math.round(ms / 86400000) + 1;
  } else {
    const start = addDays(new Date(today + 'T00:00:00'), -(days - 1));
    startStr = toDateStr(start);
    denom = days;
  }
  let total = 0;
  for (const [date, data] of Object.entries(activityData)) {
    if (date < startStr) continue;
    total += (data[field] || []).length;
  }
  return (total / denom).toFixed(1);
}

export function formatDateFull(dateStr) {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('en-GB', {
    weekday: 'long', day: 'numeric', month: 'long', year: 'numeric',
  });
}

export function dayLabel(dateStr) {
  const d = new Date(dateStr + 'T00:00:00');
  if (d.getDay() === 0) return d.toLocaleDateString('en-GB', { month: 'short', day: 'numeric' });
  return String(d.getDate());
}
