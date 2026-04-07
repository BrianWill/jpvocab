const els = {
  magazine: document.querySelector('.magazine'),
  kanjiStrip: document.querySelector('.kanji-strip'),
};

function updateFillers() {
  // Remove existing fillers before recalculating.
  els.magazine.querySelectorAll('.feature-card--filler').forEach(el => el.remove());

  const cards = [...els.magazine.querySelectorAll('.feature-card')];
  if (cards.length === 0) return;

  // Count columns by counting distinct left-edge positions of the cards.
  // This works at any zoom level or breakpoint without inspecting CSS.
  const leftPositions = new Set(cards.map(c => Math.round(c.getBoundingClientRect().left)));
  const cols = leftPositions.size;

  // No fillers needed when cols === 1 (single-column layout) or the row is already full.
  if (cols <= 1) return;
  const remainder = cards.length % cols;
  if (remainder === 0) return;

  const fillerCount = cols - remainder;
  for (let i = 0; i < fillerCount; i++) {
    const filler = document.createElement('div');
    filler.className = 'feature-card feature-card--filler';
    filler.setAttribute('aria-hidden', 'true');
    els.magazine.insertBefore(filler, els.kanjiStrip);
  }
}

updateFillers();
new ResizeObserver(updateFillers).observe(els.magazine);
