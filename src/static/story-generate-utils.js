export function formatElapsedSeconds(seconds) {
  return `${Math.max(0, Math.round(seconds))}s`;
}

export function formatTokenCount(count) {
  return new Intl.NumberFormat().format(Math.max(0, count || 0));
}

export function getTranslationTarget(story, chunkPosition) {
  if (!story) return null;
  const sentences = chunkPosition
    ? (story.sentences || []).filter(s => s.chunkPosition === chunkPosition)
    : (story.sentences || []);
  return { sentences };
}

export function getTranslationCounts(story, chunkPosition) {
  const target = getTranslationTarget(story, chunkPosition);
  const sentences = Array.isArray(target?.sentences) ? target.sentences : [];
  const uniqueWordCount = new Set(
    sentences
      .filter(s => s.orig_lang === 'jp')
      .flatMap(s => (s.words || []).filter(w => w.base).map(w => w.base))
  ).size;
  return {
    sentenceCount: sentences.length,
    uniqueWordCount,
  };
}

export function getTranslationCountsText(story, chunkPosition) {
  const { sentenceCount, uniqueWordCount } = getTranslationCounts(story, chunkPosition);
  return `${sentenceCount} sentence${sentenceCount === 1 ? '' : 's'}, ${uniqueWordCount} unique word${uniqueWordCount === 1 ? '' : 's'}`;
}
