export function storyUsesYouTubeMedia(story) {
  return (story?.mediaType || '') === 'youtube' && !!story?.mediaUrl;
}

export function storyHasLocalMedia(story) {
  const mediaType = story?.mediaType || '';
  return mediaType === 'local' && !!story?.mediaUrl;
}

export function storyMediaTypeLabel(mediaType) {
  switch (mediaType) {
    case 'youtube':
      return 'YouTube';
    case 'local':
      return 'Local media';
    default:
      return '';
  }
}

export function sentenceHasStartTime(sentence) {
  return Number.isFinite(sentence?.startTimeMs);
}

export function storyCanSeekSentenceInMedia(story, sentence) {
  return storyUsesYouTubeMedia(story) && sentenceHasStartTime(sentence);
}
