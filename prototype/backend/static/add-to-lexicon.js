export function sortAddResultRows(container) {
  if (!container) return;
  const rows = Array.from(container.children);
  rows.sort((a, b) => {
    const aLexicon = a.dataset.reason === 'already in lexicon' ? 1 : 0;
    const bLexicon = b.dataset.reason === 'already in lexicon' ? 1 : 0;
    return aLexicon - bLexicon;
  });
  rows.forEach(row => container.appendChild(row));
}
