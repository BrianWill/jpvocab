export function esc(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function timeAgo(dateStr) {
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

export function getSortedWords(words, key, dir) {
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
