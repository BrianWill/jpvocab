import { getTtsVoice, getVoicevoxSettings, checkVoicevoxAvailable, playDing, PROVIDER_MODELS, refreshTooltip } from './common.js';
import { esc } from './lexicon-utils.js';
import { initGenerateModals, populateTranslationModelSelect } from './story-generate.js';
import { initPlayback, applyAudioState, showSentencePlayBtn, scheduleSentencePlayHide, hideSentencePlayBtn, cancelSentencePlayHide, stopPlayback } from './story-playback.js';

// ── DOM refs ──────────────────────────────────────────────────────────────────
const els = {
  genBtn: document.getElementById('story-gen-btn'),
  genCancelGenerationBtn: document.getElementById('story-gen-cancel-generation-btn'),
  genConfirmBody: document.getElementById('gen-confirm-body'),
  genModalBackdrop: document.getElementById('story-gen-modal-backdrop'),
  genModalCancel: document.getElementById('story-gen-modal-cancel'),
  genModalConfirm: document.getElementById('story-gen-modal-confirm'),
  genModalDone: document.getElementById('story-gen-modal-done'),
  genProgressBody: document.getElementById('gen-progress-body'),
  genProgressCount: document.getElementById('gen-progress-count'),
  genSentenceList: document.getElementById('gen-sentence-list'),
  genTranslationBtn: document.getElementById('story-gen-translation-btn'),
  genTranslationConfirmBody: document.getElementById('gen-translation-confirm-body'),
  genTranslationModalBackdrop: document.getElementById('story-gen-translation-modal-backdrop'),
  genTranslationModalCancel: document.getElementById('story-gen-translation-modal-cancel'),
  genTranslationModalCancelGen: document.getElementById('story-gen-translation-modal-cancel-gen'),
  genTranslationModalConfirm: document.getElementById('story-gen-translation-modal-confirm'),
  genTranslationModalDone: document.getElementById('story-gen-translation-modal-done'),
  genTranslationModelSelect: document.getElementById('story-gen-translation-model-select'),
  genTranslationProgressBody: document.getElementById('gen-translation-progress-body'),
  genTranslationProviderInfo: document.getElementById('story-gen-translation-provider-info'),
  genTranslationSpinner: document.getElementById('gen-translation-spinner'),
  genTranslationStatusText: document.getElementById('gen-translation-status-text'),
  currentTime: document.getElementById('story-current-time'),
  duration: document.getElementById('story-duration'),
  seekbar: document.getElementById('story-seekbar'),
  speedDec: document.getElementById('story-speed-dec'),
  speedInc: document.getElementById('story-speed-inc'),
  speedVal: document.getElementById('story-speed-val'),
  storyLayout: document.getElementById('story-layout'),
  storyContent: document.getElementById('story-content'),
  storyError: document.getElementById('story-error'),
  storyNotedAddAll: document.getElementById('story-noted-add-all'),
  storyNotedClose: document.getElementById('story-noted-close'),
  storyNotedCount: document.getElementById('story-noted-count'),
  storyNotedEmpty: document.getElementById('story-noted-empty'),
  storyNotedList: document.getElementById('story-noted-list'),
  storyNotedTab: document.getElementById('story-noted-tab'),
  storyTitle: document.getElementById('story-title'),
  playbackBtn: document.getElementById('story-playback-btn'),
  playbackIcon: document.getElementById('story-playback-icon'),
};
els.genModalClose = els.genModalBackdrop.querySelector('.modal-close');
els.genTranslationModalClose = els.genTranslationModalBackdrop.querySelector('.modal-close');

// Floating sentence-play button (created dynamically; positioned via JS)
{
  const btn = document.createElement('button');
  btn.className = 'sentence-play-btn';
  btn.setAttribute('aria-label', 'Play from this sentence');
  btn.style.opacity = '0';
  btn.style.pointerEvents = 'none';
  document.body.appendChild(btn);
  els.sentencePlayBtn = btn;
}

// ── State ─────────────────────────────────────────────────────────────────────
const state = {
  activeIdx: -1,
  audioEl: null,
  audioMode: false,
  audioSentenceIdx: 0,
  currentUtterance: null,
  generateController: null,
  generating: false,
  hoveredWord: null,
  notedWords: [],
  notedWordsOpen: false,
  providers: null,
  translating: false,
  translationController: null,
  updatingNotedWords: false,
  lastWordAbsPos: 0,
  resumeOffset: 0,
  seekbarDragging: false,
  sentenceCumulative: [],
  sentenceDurations: [],
  sentenceOffsets: [],
  sentenceSpans: [],
  story: null,
  totalDurationMs: 0,
  wordTokenMetas: [],
  playbackRate: 1.0,
  speechText: '',
  sentencePlayBtnTargetIdx: -1,
  sentencePlayHideTimer: null,
};

// ── Helpers ───────────────────────────────────────────────────────────────────
// Returns true if the token is punctuation/whitespace with no meaningful word content.
function isPunctuation(w) {
  return !/[\u3040-\u30FF\u4E00-\u9FFFa-zA-Z0-9]/.test(w);
}

function pluralize(count, singular, plural = singular + 's') {
  return `${count} ${count === 1 ? singular : plural}`;
}

// ── Story ID ──────────────────────────────────────────────────────────────────
function storyIdFromPath() {
  const parts = window.location.pathname.split('/').filter(Boolean);
  return parts[parts.length - 1];
}
const STORY_ID = storyIdFromPath();

// ── Noted words ───────────────────────────────────────────────────────────────
function escapeTooltipText(s) {
  return esc(String(s ?? '')).replace(/\n/g, '<br>');
}

function isWordNoted(baseWord) {
  return state.notedWords.some(word => word.baseWord === baseWord);
}

function buildWordTooltipHtml(word, sentenceEnglish, isNoted = isWordNoted(word.baseWord)) {
  const ispunct = isPunctuation(word.displayWord);
  const wordTranslation = ispunct ? '' : (word.english || '');
  let html = '';
  if (sentenceEnglish) html += escapeTooltipText(sentenceEnglish);
  if (!ispunct && wordTranslation) {
    if (sentenceEnglish) html += '<br><br>';
    html += '<strong><span class="tooltip-word-label">' + esc(word.displayWord) + ':</span></strong> ' + escapeTooltipText(wordTranslation);
  }
  if (!ispunct) {
    if (html) html += '<br><br>';
    html += '<span class="tooltip-word-note">' +
      esc(isNoted ? 'Click to remove from noted words' : 'Click to add this word to noted words') +
      '</span>';
  }
  return html;
}

function updateWordTokenUI() {
  for (const meta of state.wordTokenMetas) {
    meta.el.dataset.tooltipHtml = buildWordTooltipHtml(meta.word, meta.sentenceEnglishText);
    meta.el.dataset.tooltipClass = 'tooltip-translation';
    meta.el.classList.toggle('story-word--noted', isWordNoted(meta.word.baseWord));
  }
}

function setNotedWordsOpen(open) {
  state.notedWordsOpen = !!open;
  els.storyLayout.classList.toggle('story-layout--noted-open', state.notedWordsOpen);
  els.storyNotedTab.textContent = `Noted Words (${state.notedWords.length})`;
  cancelSentencePlayHide();
  hideSentencePlayBtn();
}

function renderNotedWords(autoOpen = false) {
  els.storyNotedCount.textContent = pluralize(state.notedWords.length, 'word');
  els.storyNotedList.innerHTML = state.notedWords.map(word => `
    <div class="story-noted-item">
      <div class="story-noted-item-main">
        <p class="story-noted-item-word">${esc(word.baseWord || word.displayWord)}</p>
        ${word.displayWord && word.baseWord && word.displayWord !== word.baseWord ? `<p class="story-noted-item-base">Seen in story as: ${esc(word.displayWord)}</p>` : ''}
        ${word.english ? `<p class="story-noted-item-meaning">${esc(word.english)}</p>` : ''}
      </div>
      <button class="story-noted-item-remove" type="button" data-base-word="${esc(word.baseWord)}" aria-label="Remove ${esc(word.baseWord || word.displayWord)}">✕</button>
    </div>
  `).join('');
  els.storyNotedEmpty.hidden = state.notedWords.length > 0;
  if (autoOpen) setNotedWordsOpen(true);
  else setNotedWordsOpen(state.notedWordsOpen);
  updateWordTokenUI();
}

async function addHoveredWordToNotedWords() {
  if (!state.hoveredWord || state.updatingNotedWords || isWordNoted(state.hoveredWord.baseWord)) return;
  state.updatingNotedWords = true;
  try {
    const res = await fetch(`/api/stories/${STORY_ID}/noted-words`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        baseWord: state.hoveredWord.baseWord,
        displayWord: state.hoveredWord.displayWord,
      }),
    });
    if (!res.ok) throw new Error('failed to add noted word');
    const data = await res.json();
    state.notedWords = Array.isArray(data.notedWords) ? data.notedWords : [];
    renderNotedWords(true);
  } finally {
    state.updatingNotedWords = false;
  }
}

async function removeNotedWord(baseWord) {
  if (!baseWord || state.updatingNotedWords) return;
  state.updatingNotedWords = true;
  try {
    const res = await fetch(`/api/stories/${STORY_ID}/noted-words`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ baseWord }),
    });
    if (!res.ok) throw new Error('failed to remove noted word');
    const data = await res.json();
    state.notedWords = Array.isArray(data.notedWords) ? data.notedWords : [];
    renderNotedWords();
  } finally {
    state.updatingNotedWords = false;
  }
}

els.storyNotedTab.addEventListener('click', () => {
  setNotedWordsOpen(true);
});

els.storyNotedClose.addEventListener('click', () => {
  setNotedWordsOpen(false);
});

els.storyNotedList.addEventListener('click', event => {
  const btn = event.target.closest('[data-base-word]');
  if (!btn) return;
  removeNotedWord(btn.dataset.baseWord).catch(() => {});
});

// ── Render ────────────────────────────────────────────────────────────────────
async function loadStory(id) {
  const res = await fetch(`/api/stories/${id}`);
  if (!res.ok) throw new Error('failed to load story');
  return res.json();
}

function sentenceText(sentence) {
  return sentence.words.map(word => word.displayWord).join('');
}

function renderStory(story) {
  state.story = story;
  state.hoveredWord = null;
  state.notedWords = Array.isArray(story.notedWords) ? story.notedWords : [];
  state.notedWordsOpen = false;
  document.title = `${story.title} | Story`;
  els.storyTitle.textContent = story.title;

  const SEPARATOR = '　';
  state.sentenceSpans = [];
  state.sentenceOffsets = [];
  const textParts = [];
  let offset = 0;
  for (const sentence of story.sentences) {
    const text = sentenceText(sentence);
    state.sentenceOffsets.push(offset);
    textParts.push(text);
    offset += text.length + SEPARATOR.length;
  }
  state.speechText = textParts.join(SEPARATOR);
  els.playbackBtn.disabled = false;

  els.storyContent.innerHTML = '';
  state.wordTokenMetas = [];
  let currentParagraph = null;
  for (let i = 0; i < story.sentences.length; i++) {
    const sentence = story.sentences[i];
    if (!currentParagraph || sentence.isParagraphStart) {
      currentParagraph = document.createElement('p');
      currentParagraph.className = 'story-paragraph';
      els.storyContent.appendChild(currentParagraph);
    }
    const sentenceSpan = document.createElement('span');
    sentenceSpan.className = 'story-sentence';
    if (sentence.englishText) {
      sentenceSpan.dataset.tooltip = sentence.englishText;
      sentenceSpan.dataset.tooltipClass = 'tooltip-translation';
    }

    for (const word of sentence.words) {
      const wordSpan = document.createElement('span');
      wordSpan.className = 'story-word';
      wordSpan.textContent = word.displayWord;
      const ispunct = isPunctuation(word.displayWord);
      if (!ispunct || sentence.englishText) {
        wordSpan.dataset.tooltipHtml = buildWordTooltipHtml(word, sentence.englishText);
        wordSpan.dataset.tooltipClass = 'tooltip-translation';
      }
      if (!ispunct && word.english) wordSpan.classList.add('story-word--translated');
      if (!ispunct) {
        wordSpan.addEventListener('mouseenter', () => {
          state.hoveredWord = word;
        });
        wordSpan.addEventListener('mouseleave', () => {
          if (state.hoveredWord === word) state.hoveredWord = null;
        });
        wordSpan.addEventListener('click', () => {
          state.hoveredWord = word;
          const currentlyNoted = isWordNoted(word.baseWord);
          wordSpan.dataset.tooltipHtml = buildWordTooltipHtml(word, sentence.englishText, !currentlyNoted);
          refreshTooltip(wordSpan);
          if (currentlyNoted) {
            removeNotedWord(word.baseWord).catch(() => {});
          } else {
            addHoveredWordToNotedWords().catch(() => {});
          }
        });
      }
      state.wordTokenMetas.push({
        el: wordSpan,
        sentenceEnglishText: sentence.englishText || '',
        word,
      });
      sentenceSpan.appendChild(wordSpan);
    }
    sentenceSpan.addEventListener('mouseenter', () => showSentencePlayBtn(i));
    sentenceSpan.addEventListener('mouseleave', scheduleSentencePlayHide);
    currentParagraph.appendChild(sentenceSpan);
    currentParagraph.appendChild(document.createTextNode(' '));
    state.sentenceSpans.push(sentenceSpan);
  }

  const endMark = document.createElement('div');
  endMark.className = 'story-end-mark';
  endMark.textContent = '※';
  els.storyContent.appendChild(endMark);
  renderNotedWords();

  // Enable generate audio button if VoiceVox is available.
  checkVoicevoxAvailable().then(available => {
    els.genBtn.disabled = !available;
  });

  // Enable generate translation button if AI providers are available.
  if (state.providers) {
    populateTranslationModelSelect(state.providers);
    const hasAny = PROVIDER_MODELS.some(p => state.providers[p.key]);
    els.genTranslationBtn.disabled = !hasAny;
  }

  applyAudioState(story);
}

function renderError() {
  els.storyError.hidden = false;
}

// ── Init ──────────────────────────────────────────────────────────────────────
initPlayback(els, state, STORY_ID);

initGenerateModals(els, state, {
  storyId: STORY_ID,
  onAudioDone: applyAudioState,
  onTranslationDone: renderStory,
  stopPlayback,
});

Promise.all([
  loadStory(STORY_ID),
  fetch('/api/providers').then(r => r.json()).catch(() => null),
]).then(([story, providers]) => {
  if (providers?.ai) state.providers = providers.ai;
  renderStory(story);
}).catch(renderError);
