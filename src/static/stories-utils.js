import { escapeHtml } from './html-utils.js';
import { normalizeDateInput, pluralize } from './format-utils.js';

export function storyTimestamp(story) {
  const raw = story?.createdAt;
  if (!raw) return 0;
  const parsed = normalizeDateInput(raw).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
}

export function formatStoryDate(dateTime) {
  return normalizeDateInput(dateTime).toLocaleDateString('en-GB', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export function formatStoryTimestamp(dateTime) {
  return normalizeDateInput(dateTime).toLocaleString('en-GB', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function sentenceCountLabel(n) {
  return pluralize(n, 'sentence');
}

export function wordCountLabel(n) {
  return pluralize(n, 'unique lexicon word');
}

export function escStoryHtml(value) {
  return escapeHtml(value, { escapeApostrophe: true });
}

export function sortStories(stories) {
  return [...stories].sort((a, b) => {
    const timeDiff = storyTimestamp(b) - storyTimestamp(a);
    if (timeDiff !== 0) return timeDiff;
    return (b.id ?? 0) - (a.id ?? 0);
  });
}
