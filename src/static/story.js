import { getTtsVoice, getVoicevoxSettings, checkVoicevoxAvailable, playDing, PROVIDER_MODELS, refreshTooltip } from './common.js';
import { esc, renderReading } from './lexicon-utils.js';
import { initGenerateModals, openTranslationModal, populateTranslationModelSelect } from './story-generate.js';
import { initStoryAddToLexicon, addWordsToLexicon } from './story-add-to-lexicon.js';
import { initPlayback, initSynthPlayback, showSentencePlayBtn, scheduleSentencePlayHide, hideSentencePlayBtn, cancelSentencePlayHide, stopPlayback } from './story-playback.js';

// ── DOM refs ──────────────────────────────────────────────────────────────────
const els = {
  genTranslationBtn: document.getElementById('story-gen-translation-btn'),
  genTranslationCopy: document.getElementById('gen-translation-copy'),
  genTranslationConfirmBody: document.getElementById('gen-translation-confirm-body'),
  genTranslationCounts: document.getElementById('gen-translation-counts'),
  genTranslationElapsed: document.getElementById('gen-translation-elapsed'),
  genTranslationModalBackdrop: document.getElementById('story-gen-translation-modal-backdrop'),
  genTranslationModalCancel: document.getElementById('story-gen-translation-modal-cancel'),
  genTranslationModalCancelGen: document.getElementById('story-gen-translation-modal-cancel-gen'),
  genTranslationModalConfirm: document.getElementById('story-gen-translation-modal-confirm'),
  genTranslationModalDone: document.getElementById('story-gen-translation-modal-done'),
  genTranslationModelSelect: document.getElementById('story-gen-translation-model-select'),
  genTranslationProgressBody: document.getElementById('gen-translation-progress-body'),
  genTranslationProviderInfo: document.getElementById('story-gen-translation-provider-info'),
  genTranslationSummary: document.getElementById('gen-translation-summary'),
  genTranslationSpinner: document.getElementById('gen-translation-spinner'),
  genTranslationStatusText: document.getElementById('gen-translation-status-text'),
  currentTime: document.getElementById('story-current-time'),
  duration: document.getElementById('story-duration'),
  seekbar: document.getElementById('story-seekbar'),
  speedDec: document.getElementById('story-speed-dec'),
  speedInc: document.getElementById('story-speed-inc'),
  speedVal: document.getElementById('story-speed-val'),
  storyLayout: document.getElementById('story-layout'),
  storyMeta: document.getElementById('story-meta'),
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
  synthMode: false,
  synthLoadingIdx: -1,
  synthGeneration: 0,
  audioSentenceIdx: 0,
  audioClauseIdx: 0,
  currentUtterance: null,
  hoveredWord: null,
  wordInfoMap: new Map(),   // base -> wordInfoResponseJSON (or null if not in lexicon)
  fetchingWords: new Set(), // bases currently in-flight
  chunkObserver: null,      // IntersectionObserver for batch-fetching chunk word info
  chunkFetchTimer: null,    // debounce timer for chunk word-info fetches
  notedWords: [],
  notedWordsOpen: false,
  providers: null,
  translating: false,
  translationChunkPosition: null,
  translationChunkLabel: '',
  translationController: null,
  translationElapsedTimer: null,
  translationStartedAt: 0,
  translationWordInfoCount: null,
  updatingNotedWords: false,
  lastWordAbsPos: 0,
  resumeOffset: 0,
  sentenceOffsets: [],
  sentenceSpans: [],
  story: null,
  wordTokenMetas: [],
  playbackRate: 1.0,
  speechText: '',
  sentencePlayBtnTargetIdx: -1,
  sentencePlayHideTimer: null,
};

// ── Helpers ───────────────────────────────────────────────────────────────────
function pluralize(count, singular, plural = singular + 's') {
  return `${count} ${count === 1 ? singular : plural}`;
}

function storyMetaLabel(story) {
  return [
    pluralize(story.sentenceCount || 0, 'sentence'),
    pluralize(story.lexiconWordCount || 0, 'lexicon word'),
  ].join(' | ');
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

// wordInfo is the object returned by GET /api/word-info?base=... (or null if not in lexicon).
function buildWordTooltipHtml(word, sentenceEnglish, wordInfo, isNoted = isWordNoted(word.base)) {
  if (!wordInfo) return sentenceEnglish ? escapeTooltipText(sentenceEnglish) : '';

  const wordDisplay = word.base || word.display;
  const wordReading = wordInfo.reading
    ? renderReading(wordInfo.reading, wordDisplay, wordInfo.kanjiData, wordInfo.pitchAccent)
    : '';

  // Kanji panel
  const kanjiEntries = (wordInfo.kanjiData || []).map(entry => {
    if (!entry.character) return '';
    const isOn = /[\u30A0-\u30FF]/.test(entry.reading);
    return '<div class="kanji-entry">' +
      '<div class="kanji-char">' + esc(entry.character) + '</div>' +
      '<div class="kanji-detail">' +
        '<div class="kanji-readings"><span class="kanji-' + (isOn ? 'on' : 'kun') + '">' + esc(entry.reading) + '</span></div>' +
        '<div class="kanji-meanings">' + esc((entry.meanings || []).join(', ')) + '</div>' +
      '</div>' +
    '</div>';
  }).filter(Boolean).join('');
  const kanjiHtml = kanjiEntries
    ? '<div class="word-tooltip-kanji">' + kanjiEntries + '</div>'
    : '';

  let footerHtml;
  if (wordInfo.tracked) {
    const remaining = Math.max(0, (wordInfo.drillTarget || 0) - (wordInfo.drillCount || 0));
    footerHtml = '<div class="tooltip-word-note"><span>Word in lexicon: <span class="tooltip-drill-remaining">' +
      esc(String(remaining)) + '</span> drill' + (remaining === 1 ? '' : 's') + ' remaining</span>' +
      '<span class="tooltip-hotkey-hint">(- / + to adjust)</span></div>';
  } else {
    footerHtml = '<div class="tooltip-word-note">' +
      esc(isNoted ? 'Click to remove from noted words' : 'Click to add this word to noted words') +
      '</div>';
  }

  return (sentenceEnglish ? '<div class="story-tip-sentence">' + escapeTooltipText(sentenceEnglish) + '</div>' : '') +
    '<div class="tooltip-cols">' +
      '<div class="tooltip-main">' +
        '<div class="tooltip-word">' + esc(wordDisplay) + '</div>' +
        (wordReading ? '<div class="tooltip-reading">' + wordReading + '</div>' : '') +
        (wordInfo.type ? '<div class="tooltip-pos">' + esc(wordInfo.type) + '</div>' : '') +
        (wordInfo.english ? '<div class="tooltip-meaning">' + esc(wordInfo.english) + '</div>' : '') +
      '</div>' +
      kanjiHtml +
    '</div>' +
    footerHtml;
}

function updateWordTokensForBase(base) {
  const wordInfo = state.wordInfoMap.get(base) || null;
  for (const meta of state.wordTokenMetas) {
    if (meta.word.base !== base) continue;
    const isStoryWord = !!wordInfo;
    meta.el.dataset.tooltipHtml = buildWordTooltipHtml(meta.word, meta.sentenceEnglishText, wordInfo);
    meta.el.dataset.tooltipClass = isStoryWord ? 'story-word-tooltip' : (meta.sentenceEnglishText ? 'tooltip-translation' : '');
    meta.el.classList.toggle('story-word--translated', isStoryWord && !!wordInfo.english);
    meta.el.classList.toggle('story-word--in-lexicon', isStoryWord && !!wordInfo.tracked);
    meta.el.classList.toggle('story-word--noted', isStoryWord && isWordNoted(base) && !wordInfo.tracked);
  }
}

function updateWordTokenUI() {
  for (const meta of state.wordTokenMetas) {
    const wordInfo = state.wordInfoMap.get(meta.word.base) || null;
    const isStoryWord = !!wordInfo;
    meta.el.dataset.tooltipHtml = buildWordTooltipHtml(meta.word, meta.sentenceEnglishText, wordInfo);
    meta.el.dataset.tooltipClass = isStoryWord ? 'story-word-tooltip' : (meta.sentenceEnglishText ? 'tooltip-translation' : '');
    meta.el.classList.toggle('story-word--noted', isStoryWord && isWordNoted(meta.word.base) && !wordInfo.tracked);
    meta.el.classList.toggle('story-word--in-lexicon', isStoryWord && !!wordInfo.tracked);
  }
}

async function fetchWordInfoBatch(bases) {
  const toFetch = bases.filter(b => b && !state.wordInfoMap.has(b) && !state.fetchingWords.has(b));
  if (toFetch.length === 0) return;
  for (const b of toFetch) state.fetchingWords.add(b);
  try {
    const res = await fetch('/api/word-info-batch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ bases: toFetch }),
    });
    if (!res.ok) return; // leave out of map so next scroll retries
    const data = await res.json();
    const words = data.words || {};
    for (const base of toFetch) {
      const info = words[base];
      state.wordInfoMap.set(base, info?.wordId ? info : null);
      updateWordTokensForBase(base);
    }
  } catch (_) {
    // leave out of map for retry
  } finally {
    for (const b of toFetch) state.fetchingWords.delete(b);
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
  els.storyNotedAddAll.disabled = state.notedWords.length === 0 || state.updatingNotedWords;
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
  if (!state.hoveredWord || state.updatingNotedWords || isWordNoted(state.hoveredWord.base)) return;
  state.updatingNotedWords = true;
  try {
    const res = await fetch(`/api/stories/${STORY_ID}/noted-words`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        baseWord: state.hoveredWord.base,
        displayWord: state.hoveredWord.display,
      }),
    });
    if (!res.ok) throw new Error('failed to add noted word');
    const data = await res.json();
    state.notedWords = Array.isArray(data.notedWords) ? data.notedWords : [];
  } finally {
    state.updatingNotedWords = false;
    renderNotedWords(true);
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
  } finally {
    state.updatingNotedWords = false;
    renderNotedWords();
  }
}

document.addEventListener('keydown', e => {
  if (e.target.closest('input, textarea, select, [contenteditable]')) return;
  const word = state.hoveredWord;
  if (!word?.base) return;
  const wordInfo = state.wordInfoMap.get(word.base);
  if (!wordInfo?.tracked || !wordInfo.wordId) return;
  const delta = (e.key === '-') ? -1 : (e.key === '+' || e.key === '=') ? 1 : 0;
  if (delta === 0) return;
  e.preventDefault();
  const newTarget = Math.min(999, Math.max(wordInfo.drillCount || 0, (wordInfo.drillTarget || 0) + delta));
  if (newTarget === wordInfo.drillTarget) return;
  wordInfo.drillTarget = newTarget;
  // Refresh tooltip on the hovered span.
  const meta = state.wordTokenMetas.find(m => m.word === word);
  if (meta) {
    meta.el.dataset.tooltipHtml = buildWordTooltipHtml(word, meta.sentenceEnglishText, wordInfo);
    refreshTooltip(meta.el);
  }
  fetch('/api/words/' + wordInfo.wordId + '/target', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target: newTarget }),
  }).catch(() => {});
});

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

els.storyNotedAddAll.addEventListener('click', async () => {
  if (state.notedWords.length === 0 || state.updatingNotedWords) return;
  els.storyNotedAddAll.disabled = true;
  try {
    await addWordsToLexicon(state.notedWords.map(word => word.baseWord || word.displayWord));
  } catch (_) {
    els.storyNotedAddAll.disabled = false;
    return;
  }
  // Re-fetch story to get updated inLexicon flags and remove newly-lexiconed noted words.
  try {
    // Evict cached info for the noted words before fetching — their tracked status just changed.
    const notedBases = state.notedWords.map(w => w.baseWord).filter(Boolean);
    for (const base of notedBases) state.wordInfoMap.delete(base);
    const updated = await loadStory(STORY_ID);
    state.notedWords = Array.isArray(updated.notedWords) ? updated.notedWords : [];
    fetchWordInfoBatch(notedBases).catch(() => {});
  } catch (_) {}
  renderNotedWords();
});

els.storyContent.addEventListener('click', event => {
  const btn = event.target.closest('.story-chunk-translate-btn');
  if (!btn) return;
  openTranslationModal(Number(btn.dataset.chunkPosition), btn.dataset.chunkLabel || '');
});

// ── Render ────────────────────────────────────────────────────────────────────
async function loadStory(id) {
  const res = await fetch(`/api/stories/${id}`);
  if (!res.ok) throw new Error('failed to load story');
  return res.json();
}

function sentenceText(sentence) {
  return sentence.words.map(word => word.display).join('');
}

function renderStory(story) {
  if (state.chunkObserver) {
    state.chunkObserver.disconnect();
    state.chunkObserver = null;
  }
  clearTimeout(state.chunkFetchTimer);
  state.chunkFetchTimer = null;
  state.story = story;
  state.hoveredWord = null;
  state.notedWords = Array.isArray(story.notedWords) ? story.notedWords : [];
  state.notedWordsOpen = false;
  document.title = `${story.title} | Story`;
  els.storyTitle.textContent = story.title;
  els.storyMeta.textContent = storyMetaLabel(story);

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

  const hasProviders = !!state.providers && PROVIDER_MODELS.some(p => state.providers[p.key]);

  els.storyContent.innerHTML = '';
  state.wordTokenMetas = [];
  let globalSentenceIdx = 0;
  const chunkPositions = [...new Set((story.sentences || []).map(s => s.chunkPosition))].sort((a, b) => a - b);
  for (const chunkPos of chunkPositions) {
    const chunkSentences = (story.sentences || []).filter(s => s.chunkPosition === chunkPos);
    const translated = chunkSentences.some(s => !!s.englishText);
    const translateTooltip = translated ? 'Retranslate section' : 'Translate section';
    const chunkSection = document.createElement('section');
    chunkSection.className = 'story-chunk';
    chunkSection.dataset.chunkPosition = String(chunkPos);

    const chunkHeader = document.createElement('div');
    chunkHeader.className = 'story-chunk-header';
    chunkHeader.innerHTML = `
      <div class="story-chunk-header-spacer" aria-hidden="true"></div>
      <button class="btn-save story-gen-btn story-chunk-translate-btn" type="button" data-chunk-position="${chunkPos}" data-chunk-label="this section" data-tooltip="${translateTooltip}" aria-label="${translateTooltip}" ${hasProviders ? '' : 'disabled'}>文A</button>
    `;
    chunkSection.appendChild(chunkHeader);

    const chunkBody = document.createElement('div');
    chunkBody.className = 'story-chunk-body';
    let currentParagraph = null;
    for (const sentence of chunkSentences) {
      const sentenceIdx = globalSentenceIdx;
      if (!currentParagraph || sentence.isParagraphStart) {
        currentParagraph = document.createElement('p');
        currentParagraph.className = 'story-paragraph';
        chunkBody.appendChild(currentParagraph);
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
        wordSpan.textContent = word.display;
        const wordInfo = state.wordInfoMap.get(word.base) || null;
        const isStoryWord = !!wordInfo;
        if (word.base || sentence.englishText) {
          wordSpan.dataset.tooltipHtml = buildWordTooltipHtml(word, sentence.englishText, wordInfo);
          wordSpan.dataset.tooltipClass = isStoryWord ? 'story-word-tooltip' : (sentence.englishText ? 'tooltip-translation' : '');
        }
        if (isStoryWord && wordInfo.english) wordSpan.classList.add('story-word--translated');
        if (isStoryWord && wordInfo.tracked) wordSpan.classList.add('story-word--in-lexicon');
        if (word.base) {
          wordSpan.addEventListener('mouseenter', () => {
            state.hoveredWord = word;
          });
          wordSpan.addEventListener('mouseleave', () => {
            if (state.hoveredWord === word) state.hoveredWord = null;
          });
          wordSpan.addEventListener('click', () => {
            state.hoveredWord = word;
            const wi = state.wordInfoMap.get(word.base) || null;
            if (wi?.tracked) return;
            const currentlyNoted = isWordNoted(word.base);
            wordSpan.dataset.tooltipHtml = buildWordTooltipHtml(word, sentence.englishText, wi, !currentlyNoted);
            refreshTooltip(wordSpan);
            if (currentlyNoted) {
              removeNotedWord(word.base).catch(() => {});
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
      sentenceSpan.addEventListener('mouseenter', () => showSentencePlayBtn(sentenceIdx));
      sentenceSpan.addEventListener('mouseleave', scheduleSentencePlayHide);
      currentParagraph.appendChild(sentenceSpan);
      currentParagraph.appendChild(document.createTextNode(' '));
      state.sentenceSpans.push(sentenceSpan);
      globalSentenceIdx++;
    }
    chunkSection.appendChild(chunkBody);
    els.storyContent.appendChild(chunkSection);
  }

  const endMark = document.createElement('div');
  endMark.className = 'story-end-mark';
  endMark.textContent = '※';
  els.storyContent.appendChild(endMark);

  const intersectingChunks = new Set();
  state.chunkObserver = new IntersectionObserver(entries => {
    for (const entry of entries) {
      const chunkPos = Number(entry.target.dataset.chunkPosition);
      if (entry.isIntersecting) {
        intersectingChunks.add(chunkPos);
      } else {
        intersectingChunks.delete(chunkPos);
      }
    }
    clearTimeout(state.chunkFetchTimer);
    state.chunkFetchTimer = setTimeout(() => {
      // Only fetch bases for chunks still in view when the timer fires.
      const bases = [...new Set(
        [...intersectingChunks].flatMap(chunkPos =>
          (story.sentences || []).filter(s => s.chunkPosition === chunkPos)
            .flatMap(s => (s.words || []).map(w => w.base).filter(Boolean))
        )
      )];
      for (const section of chunkSections) {
        if (intersectingChunks.has(Number(section.dataset.chunkPosition))) {
          state.chunkObserver.unobserve(section);
        }
      }
      intersectingChunks.clear();
      fetchWordInfoBatch(bases).catch(() => {});
    }, 150);
  }, { rootMargin: '300px' });
  const chunkSections = [...els.storyContent.querySelectorAll('.story-chunk')];
  for (const section of chunkSections) {
    state.chunkObserver.observe(section);
  }
  // Eagerly fetch the first few chunks without waiting for the observer.
  const EAGER_CHUNK_COUNT = 3;
  const eagerBases = [...new Set(
    chunkPositions.slice(0, EAGER_CHUNK_COUNT).flatMap(chunkPos =>
      (story.sentences || []).filter(s => s.chunkPosition === chunkPos)
        .flatMap(s => (s.words || []).map(w => w.base).filter(Boolean))
    )
  )];
  fetchWordInfoBatch(eagerBases).catch(() => {});

  renderNotedWords();

  // Enable synth-mode playback if VoiceVox is available.
  checkVoicevoxAvailable().then(available => {
    if (available) initSynthPlayback();
  });

  // Enable generate translation button if AI providers are available.
  if (state.providers) {
    populateTranslationModelSelect(state.providers);
  }

}

function renderError() {
  els.storyError.hidden = false;
}

// ── Init ──────────────────────────────────────────────────────────────────────
initPlayback(els, state, STORY_ID);
initStoryAddToLexicon().catch(() => {});

initGenerateModals(els, state, {
  storyId: STORY_ID,
  onTranslationDone: (story, chunkPosition) => {
    renderStory(story);
    if (chunkPosition != null) {
      const chunkSentences = (story.sentences || []).filter(s => s.chunkPosition === chunkPosition);
      const bases = [...new Set(chunkSentences.flatMap(s => (s.words || []).map(w => w.base).filter(Boolean)))];
      fetchWordInfoBatch(bases).catch(() => {});
    }
  },
  stopPlayback,
});

Promise.all([
  loadStory(STORY_ID),
  fetch('/api/providers').then(r => r.json()).catch(() => null),
]).then(([story, providers]) => {
  if (providers?.ai) state.providers = providers.ai;
  renderStory(story);
}).catch(renderError);
