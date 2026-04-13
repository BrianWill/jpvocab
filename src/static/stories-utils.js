export function storyTimestamp(story) {
  const raw = story?.createdAt;
  if (!raw) return 0;
  const parsed = Date.parse(raw.includes('T') ? raw : raw.replace(' ', 'T'));
  return Number.isNaN(parsed) ? 0 : parsed;
}

export function formatStoryDate(dateTime) {
  return new Date(dateTime.includes('T') ? dateTime : dateTime.replace(' ', 'T')).toLocaleDateString('en-GB', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export function formatStoryTimestamp(dateTime) {
  return new Date(dateTime.includes('T') ? dateTime : dateTime.replace(' ', 'T')).toLocaleString('en-GB', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function sentenceCountLabel(n) {
  return n === 1 ? '1 sentence' : `${n} sentences`;
}

export function wordCountLabel(n) {
  return n === 1 ? '1 unique lexicon word' : `${n} unique lexicon words`;
}

export function escStoryHtml(value) {
  return String(value).replace(/[&<>"']/g, char => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }[char]));
}

export function sortStories(stories) {
  return [...stories].sort((a, b) => {
    const timeDiff = storyTimestamp(b) - storyTimestamp(a);
    if (timeDiff !== 0) return timeDiff;
    return (b.id ?? 0) - (a.id ?? 0);
  });
}
