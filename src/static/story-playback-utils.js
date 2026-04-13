export function clampPlaybackRate(rate) {
  return Math.min(2.0, Math.max(0.5, parseFloat(rate.toFixed(2))));
}

export function speechPlaybackLangForStory(story) {
  const sentences = story?.sentences || [];
  if (sentences.length > 0 && sentences.every(sentence => sentence.orig_lang === 'en')) {
    return 'en-US';
  }
  return 'ja-JP';
}

export function splitByClause(sentence) {
  const clauses = [];
  let current = [];
  for (const word of sentence.words) {
    current.push(word);
    if (word.display.includes('、')) {
      clauses.push(current);
      current = [];
    }
  }
  if (current.length > 0) clauses.push(current);
  return clauses.filter(c => c.length > 0);
}
