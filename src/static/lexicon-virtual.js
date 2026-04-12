export function computeVisibleRange({ scrollTop, viewportHeight, itemHeight, totalItems, overscan = 0 }) {
  if (totalItems <= 0 || itemHeight <= 0 || viewportHeight <= 0) {
    return { start: 0, end: 0 };
  }
  const firstVisible = Math.floor(Math.max(0, scrollTop) / itemHeight);
  const visibleCount = Math.max(1, Math.ceil(viewportHeight / itemHeight));
  const start = Math.max(0, firstVisible - overscan);
  const end = Math.min(totalItems, firstVisible + visibleCount + overscan);
  return { start, end };
}

export function mergeWordPage(wordSlots, offset, items, total) {
  const next = Array.from({ length: total }, (_, i) => (i < wordSlots.length ? wordSlots[i] ?? null : null));
  for (let i = 0; i < items.length; i++) {
    const idx = offset + i;
    if (idx >= 0 && idx < total) next[idx] = items[i];
  }
  return next;
}

export function removeWordAtIndex(wordSlots, index) {
  if (index < 0 || index >= wordSlots.length) return wordSlots.slice();
  return wordSlots.slice(0, index).concat(wordSlots.slice(index + 1));
}
