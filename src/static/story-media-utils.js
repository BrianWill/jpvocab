export function storyUsesYouTubeMedia(story) {
  return (story?.mediaType || '') === 'youtube' && !!story?.mediaUrl;
}

export function storyHasLocalMedia(story) {
  const mediaType = story?.mediaType || '';
  return (mediaType === 'local_audio' || mediaType === 'local_video') && !!story?.mediaUrl;
}

export function storyMediaTypeLabel(mediaType) {
  switch (mediaType) {
    case 'youtube':
      return 'YouTube';
    case 'local_audio':
      return 'Local audio';
    case 'local_video':
      return 'Local video';
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
