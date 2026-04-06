const els = {
  addConfirmBtn: document.getElementById('story-add-confirm'),
  addError: document.getElementById('story-add-error'),
  addModalBackdrop: document.getElementById('story-add-modal-backdrop'),
  headerAddBtn: document.querySelector('.btn-header'),
  deleteConfirmBtn: document.getElementById('story-delete-confirm'),
  deleteError: document.getElementById('story-delete-error'),
  deleteModalBackdrop: document.getElementById('story-delete-modal-backdrop'),
  deleteModalLabel: document.getElementById('story-delete-modal-label'),
  empty: document.getElementById('stories-empty'),
  list: document.getElementById('stories-list'),
  storyContentInput: document.getElementById('story-content-input'),
  storyTitleInput: document.getElementById('story-title-input'),
};
els.addModalCloseBtn = els.addModalBackdrop.querySelector('.modal-close');
els.addModalCancelBtn = els.addModalBackdrop.querySelector('.btn-cancel');
els.deleteModalCloseBtn = els.deleteModalBackdrop.querySelector('.modal-close');
els.deleteModalCancelBtn = els.deleteModalBackdrop.querySelector('.btn-cancel');

const state = {
  deletingStoryId: null,
  stories: [],
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

function esc(value) {
  return String(value).replace(/[&<>"']/g, char => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }[char]));
}

function renderStories(stories) {
  state.stories = stories;

  if (!stories.length) {
    els.empty.hidden = false;
    els.list.innerHTML = '';
    return;
  }

  els.empty.hidden = true;
  els.list.innerHTML = stories.map(story => `
    <a class="story-card-link" href="/stories/${story.id}">
      <article class="story-card">
        <button class="story-card-delete" type="button" data-story-id="${story.id}" aria-label="Delete ${esc(story.title)}">✕</button>
        <div class="story-card-meta">
          <span>${formatDate(story.createdAt)}</span>
          <span>${sentenceCountLabel(story.sentences.length)}</span>
          <span>${wordCountLabel(wordCount(story))}</span>
        </div>
        <h2 class="story-card-title">${esc(story.title)}</h2>
      </article>
    </a>
  `).join('');

  els.list.querySelectorAll('.story-card-delete').forEach(btn => {
    btn.addEventListener('click', openDeleteModal);
  });
}

function renderError() {
  els.empty.hidden = false;
  els.empty.textContent = 'Could not load stories right now.';
}

function onBackdropClick(event, closeFn) {
  if (event.target === event.currentTarget) closeFn();
}

function openAddModal() {
  els.storyTitleInput.value = '';
  els.storyContentInput.value = '';
  els.addError.classList.add('hidden');
  els.addConfirmBtn.disabled = false;
  els.addConfirmBtn.textContent = 'Add';
  els.addModalBackdrop.classList.remove('hidden');
  els.storyTitleInput.focus();
}

function closeAddModal() {
  els.addModalBackdrop.classList.add('hidden');
}

async function confirmAdd() {
  els.addConfirmBtn.disabled = true;
  els.addConfirmBtn.innerHTML = '<span class="spinner"></span>';
  els.addError.classList.add('hidden');

  try {
    const res = await fetch('/api/stories', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title: els.storyTitleInput.value,
        content: els.storyContentInput.value,
      }),
    });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    const story = await res.json();
    closeAddModal();
    renderStories([story, ...state.stories]);
  } catch (err) {
    els.addError.textContent = err.message;
    els.addError.classList.remove('hidden');
    els.addConfirmBtn.disabled = false;
    els.addConfirmBtn.textContent = 'Add';
  }
}

function openDeleteModal(event) {
  event.preventDefault();
  event.stopPropagation();
  const storyId = Number(event.currentTarget.dataset.storyId);
  const story = state.stories.find(item => item.id === storyId);
  if (!story) return;

  state.deletingStoryId = storyId;
  els.deleteModalLabel.textContent = story.title;
  els.deleteError.classList.add('hidden');
  els.deleteConfirmBtn.disabled = false;
  els.deleteConfirmBtn.textContent = 'Delete';
  els.deleteModalBackdrop.classList.remove('hidden');
}

function closeDeleteModal() {
  els.deleteModalBackdrop.classList.add('hidden');
  state.deletingStoryId = null;
}

async function confirmDelete() {
  if (!state.deletingStoryId) return;

  els.deleteConfirmBtn.disabled = true;
  els.deleteConfirmBtn.innerHTML = '<span class="spinner"></span>';
  els.deleteError.classList.add('hidden');

  try {
    const res = await fetch(`/api/stories/${state.deletingStoryId}`, { method: 'DELETE' });
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    const deletingStoryId = state.deletingStoryId;
    closeDeleteModal();
    renderStories(state.stories.filter(story => story.id !== deletingStoryId));
  } catch (err) {
    els.deleteError.textContent = err.message;
    els.deleteError.classList.remove('hidden');
    els.deleteConfirmBtn.disabled = false;
    els.deleteConfirmBtn.textContent = 'Delete';
  }
}

document.addEventListener('keydown', event => {
  if (event.key === 'Escape') {
    closeAddModal();
    closeDeleteModal();
  }
});

els.headerAddBtn.addEventListener('click', openAddModal);
els.addModalBackdrop.addEventListener('click', event => onBackdropClick(event, closeAddModal));
els.addModalCloseBtn.addEventListener('click', closeAddModal);
els.addModalCancelBtn.addEventListener('click', closeAddModal);
els.addConfirmBtn.addEventListener('click', confirmAdd);

els.deleteModalBackdrop.addEventListener('click', event => onBackdropClick(event, closeDeleteModal));
els.deleteModalCloseBtn.addEventListener('click', closeDeleteModal);
els.deleteModalCancelBtn.addEventListener('click', closeDeleteModal);
els.deleteConfirmBtn.addEventListener('click', confirmDelete);

loadStories().then(renderStories).catch(renderError);
