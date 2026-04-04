const els = {
  empty: document.getElementById('stories-empty'),
  list: document.getElementById('stories-list'),
};

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
  if (!stories.length) {
    els.empty.hidden = false;
    els.list.innerHTML = '';
    return;
  }

  els.empty.hidden = true;
  els.list.innerHTML = stories.map(story => `
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
  els.empty.hidden = false;
  els.empty.textContent = 'Could not load stories right now.';
}

loadStories().then(renderStories).catch(renderError);
