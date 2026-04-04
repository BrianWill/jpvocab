async function loadStories() {
  const res = await fetch('/api/stories');
  if (!res.ok) throw new Error('failed to load stories');
  return res.json();
}

function formatDate(dateTime) {
  return new Date(dateTime.replace(' ', 'T')).toLocaleDateString('en-GB', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

function sentenceCountLabel(n) {
  return n === 1 ? '1 sentence' : `${n} sentences`;
}

function wordCount(story) {
  return story.sentences.reduce((total, sentence) => total + sentence.words.length, 0);
}

function wordCountLabel(n) {
  return n === 1 ? '1 word' : `${n} words`;
}

function renderStories(stories) {
  const list = document.getElementById('stories-list');
  const empty = document.getElementById('stories-empty');

  if (!stories.length) {
    empty.hidden = false;
    list.innerHTML = '';
    return;
  }

  empty.hidden = true;
  list.innerHTML = stories.map(story => `
    <a class="story-card-link" href="/stories/${story.id}">
    <article class="story-card">
      <div class="story-card-meta">
        <span>${formatDate(story.createdAt)}</span>
        <span>${sentenceCountLabel(story.sentences.length)}</span>
        <span>${wordCountLabel(wordCount(story))}</span>
      </div>
      <h2 class="story-card-title">${story.title}</h2>
    </article></a>
  `).join('');
}

function renderError() {
  const empty = document.getElementById('stories-empty');
  empty.hidden = false;
  empty.textContent = 'Could not load stories right now.';
}

loadStories().then(renderStories).catch(renderError);
