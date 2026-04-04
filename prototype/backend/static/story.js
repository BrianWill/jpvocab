function storyIdFromPath() {
  const parts = window.location.pathname.split('/').filter(Boolean);
  return parts[parts.length - 1];
}

async function loadStory(id) {
  const res = await fetch(`/api/stories/${id}`);
  if (!res.ok) throw new Error('failed to load story');
  return res.json();
}

function sentenceText(sentence) {
  return sentence.words.map(word => word.displayWord).join('');
}

function renderStory(story) {
  document.title = `${story.title} | Story`;
  document.getElementById('story-title').textContent = story.title;

  const content = document.getElementById('story-content');
  content.innerHTML = '';

  let currentParagraph = null;
  for (const sentence of story.sentences) {
    if (!currentParagraph || sentence.isParagraphStart) {
      currentParagraph = document.createElement('p');
      currentParagraph.className = 'story-paragraph';
      content.appendChild(currentParagraph);
    }

    const span = document.createElement('span');
    span.className = 'story-sentence';
    span.textContent = sentenceText(sentence) + ' ';
    currentParagraph.appendChild(span);
  }
}

function renderError() {
  document.getElementById('story-error').hidden = false;
}

loadStory(storyIdFromPath()).then(renderStory).catch(renderError);
