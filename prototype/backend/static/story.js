import { getTtsVoice, getVoicevoxSettings, checkVoicevoxAvailable, playDing } from './common.js';

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

// ── Provider models (same list as lexicon-add-edit.js) ───────────────────────
const PROVIDER_MODELS = [
  { key: 'anthropic', label: 'Anthropic', envKey: 'ANTHROPIC_API_KEY', models: [
    ['anthropic/claude-haiku-4-5-20251001', 'claude-haiku (fast)'],
    ['anthropic/claude-sonnet-4-6',         'claude-sonnet (better)'],
  ]},
  { key: 'openai',   label: 'OpenAI',   envKey: 'OPENAI_API_KEY',   models: [
    ['openai/gpt-4o-mini', 'gpt-4o-mini (fast)'],
    ['openai/gpt-4o',      'gpt-4o (better)'],
  ]},
  { key: 'google',   label: 'Google',   envKey: 'GOOGLE_API_KEY',   models: [
    ['google/gemini-2.0-flash', 'gemini-2.0-flash (fast)'],
    ['google/gemini-1.5-pro',   'gemini-1.5-pro (better)'],
  ]},
  { key: 'mistral',  label: 'Mistral',  envKey: 'MISTRAL_API_KEY',  models: [
    ['mistral/mistral-small-latest', 'mistral-small (fast)'],
    ['mistral/mistral-large-latest', 'mistral-large (better)'],
  ]},
  { key: 'glm',      label: 'GLM',      envKey: 'GLM_API_KEY',      models: [
    ['glm/glm-4',       'glm-4 (better)'],
    ['glm/glm-3-turbo', 'glm-3-turbo (fast)'],
  ]},
];

// ── Playback state ────────────────────────────────────────────────────────────
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
function esc(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// Returns true if the token is punctuation/whitespace with no meaningful word content.
function isPunctuation(w) {
  return !/[\u3040-\u30FF\u4E00-\u9FFFa-zA-Z0-9]/.test(w);
}

function formatDuration(ms) {
  const totalSeconds = Math.max(0, Math.round(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
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

// ── Speed stepper ─────────────────────────────────────────────────────────────
const SPEED_STEPPER_INTERVAL = 230;

function clampPlaybackRate(rate) {
  return Math.min(2.0, Math.max(0.5, parseFloat(rate.toFixed(2))));
}

async function restartPlaybackForRateChange() {
  if (state.audioMode) {
    if (state.audioEl) state.audioEl.playbackRate = state.playbackRate;
    return;
  }

  if (!window.speechSynthesis.speaking) return;
  stopSpeechPlayback();
  await startSpeechPlayback();
}

async function setPlaybackRate(nextRate) {
  const clamped = clampPlaybackRate(nextRate);
  if (clamped === state.playbackRate) {
    els.speedVal.textContent = clamped.toFixed(2);
    return;
  }

  state.playbackRate = clamped;
  els.speedVal.textContent = state.playbackRate.toFixed(2);
  await restartPlaybackForRateChange();
}

function attachHoldRateButton(button, delta) {
  let stepTimer = null;
  let suppressClick = false;

  const stopStep = () => {
    if (stepTimer) {
      clearInterval(stepTimer);
      stepTimer = null;
    }
  };

  const startStep = () => {
    suppressClick = true;
    setPlaybackRate(state.playbackRate + delta);
    stepTimer = setInterval(() => {
      setPlaybackRate(state.playbackRate + delta);
    }, SPEED_STEPPER_INTERVAL);
  };

  button.addEventListener('pointerdown', event => {
    if (event.button !== 0) return;
    stopStep();
    startStep();
  });
  button.addEventListener('pointerup', stopStep);
  button.addEventListener('pointercancel', stopStep);
  button.addEventListener('pointerleave', stopStep);
  button.addEventListener('click', event => {
    if (suppressClick) {
      suppressClick = false;
      event.preventDefault();
      return;
    }
    setPlaybackRate(state.playbackRate + delta);
  });
}

attachHoldRateButton(els.speedDec, -0.05);
attachHoldRateButton(els.speedInc, 0.05);

// ── Icons ─────────────────────────────────────────────────────────────────────
const ICON_PLAY = '<path d="M8 5v14l11-7z"/>';
const ICON_STOP = '<rect x="6" y="6" width="12" height="12"/>';
const ICON_PLAY_SM = '<svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M8 5v14l11-7z"/></svg>';
const ICON_STOP_SM = '<svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><rect x="6" y="6" width="12" height="12"/></svg>';

function isSentencePlaying(idx) {
  if (state.audioMode) return state.audioSentenceIdx === idx && !!state.audioEl && !state.audioEl.paused;
  return state.activeIdx === idx && window.speechSynthesis.speaking;
}

function updateSentencePlayBtnIcon() {
  const idx = state.sentencePlayBtnTargetIdx;
  if (idx < 0) return;
  const playing = isSentencePlaying(idx);
  els.sentencePlayBtn.innerHTML = playing ? ICON_STOP_SM : ICON_PLAY_SM;
  els.sentencePlayBtn.setAttribute('aria-label', playing ? 'Stop' : 'Play from this sentence');
}

function setPlaybackPlaying(playing) {
  els.playbackIcon.innerHTML = playing ? ICON_STOP : ICON_PLAY;
  els.playbackBtn.setAttribute('aria-label', playing ? 'Stop reading' : 'Play story');
  updateSentencePlayBtnIcon();
}

// ── Sentence / word highlight (shared by both modes) ──────────────────────────
function setActiveIdx(idx) {
  state.sentenceSpans[state.activeIdx]?.classList.remove('story-sentence--active');
  state.activeIdx = idx;
  const span = state.sentenceSpans[state.activeIdx];
  span?.classList.add('story-sentence--active');
  span?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  updateSentencePlayBtnIcon();
}

function clearHighlight() {
  state.sentenceSpans[state.activeIdx]?.classList.remove('story-sentence--active');
  state.activeIdx = -1;
  state.resumeOffset = 0;
  state.lastWordAbsPos = 0;
}

// ── Floating sentence play button ─────────────────────────────────────────────
function hideSentencePlayBtn() {
  els.sentencePlayBtn.style.opacity = '0';
  els.sentencePlayBtn.style.pointerEvents = 'none';
  state.sentencePlayBtnTargetIdx = -1;
}

function scheduleSentencePlayHide() {
  if (state.sentencePlayHideTimer !== null) clearTimeout(state.sentencePlayHideTimer);
  state.sentencePlayHideTimer = setTimeout(() => {
    state.sentencePlayHideTimer = null;
    hideSentencePlayBtn();
  }, 230);
}

function cancelSentencePlayHide() {
  if (state.sentencePlayHideTimer !== null) {
    clearTimeout(state.sentencePlayHideTimer);
    state.sentencePlayHideTimer = null;
  }
}

function showSentencePlayBtn(idx) {
  cancelSentencePlayHide();
  state.sentencePlayBtnTargetIdx = idx;
  const span = state.sentenceSpans[idx];
  const firstWord = span?.querySelector('.story-word');
  const rect = (firstWord ?? span)?.getBoundingClientRect();
  if (!rect) return;
  const btnSize = els.sentencePlayBtn.offsetHeight || 20;
  els.sentencePlayBtn.style.opacity = '1';
  els.sentencePlayBtn.style.pointerEvents = 'auto';
  els.sentencePlayBtn.style.left = `${rect.left - btnSize / 1.4}px`;
  els.sentencePlayBtn.style.top = `${rect.top - btnSize / 1.4}px`;
  updateSentencePlayBtnIcon();
}

els.sentencePlayBtn.addEventListener('mouseenter', cancelSentencePlayHide);
els.sentencePlayBtn.addEventListener('mouseleave', scheduleSentencePlayHide);
els.sentencePlayBtn.addEventListener('click', async () => {
  const idx = state.sentencePlayBtnTargetIdx;
  if (idx < 0) return;

  if (isSentencePlaying(idx)) {
    if (state.audioMode) stopAudio();
    else stopSpeechPlayback();
    els.sentencePlayBtn.innerHTML = ICON_PLAY_SM;
    els.sentencePlayBtn.setAttribute('aria-label', 'Play from this sentence');
    return;
  }

  // (no timer management needed — button visibility is driven by hover)

  if (state.audioMode) {
    startAudio(idx, 0);
  } else {
    if (state.currentUtterance) {
      state.currentUtterance.onboundary = null;
      state.currentUtterance.onend = null;
      state.currentUtterance.onerror = null;
      state.currentUtterance = null;
      window.speechSynthesis.cancel();
    }
    state.resumeOffset = state.sentenceOffsets[idx];
    state.lastWordAbsPos = state.resumeOffset;
    await startSpeechPlayback();
  }
});

// ── Speech-synthesis mode ─────────────────────────────────────────────────────
function highlightAt(charIndex) {
  state.lastWordAbsPos = state.resumeOffset + charIndex;
  const abs = state.lastWordAbsPos;

  let sIdx = 0;
  for (let i = 1; i < state.sentenceOffsets.length; i++) {
    if (state.sentenceOffsets[i] <= abs) sIdx = i;
    else break;
  }
  if (sIdx !== state.activeIdx) setActiveIdx(sIdx);
}

function stopSpeechPlayback() {
  state.resumeOffset = state.lastWordAbsPos;
  if (state.currentUtterance) {
    state.currentUtterance.onboundary = null;
    state.currentUtterance.onend = null;
    state.currentUtterance.onerror = null;
    state.currentUtterance = null;
  }
  window.speechSynthesis.cancel();
  setPlaybackPlaying(false);
}

async function startSpeechPlayback() {
  if (!speechSynthesis.getVoices().length) {
    await new Promise(resolve => speechSynthesis.addEventListener('voiceschanged', resolve, { once: true }));
  }
  state.currentUtterance = new SpeechSynthesisUtterance(state.speechText.slice(state.resumeOffset));
  state.currentUtterance.lang = 'ja-JP';
  state.currentUtterance.rate = state.playbackRate;
  const voice = getTtsVoice('ja-JP');
  if (voice) state.currentUtterance.voice = voice;
  state.currentUtterance.onboundary = e => highlightAt(e.charIndex);
  state.currentUtterance.onend = () => { state.currentUtterance = null; clearHighlight(); setPlaybackPlaying(false); };
  state.currentUtterance.onerror = () => { state.currentUtterance = null; clearHighlight(); setPlaybackPlaying(false); };
  window.speechSynthesis.speak(state.currentUtterance);
  setPlaybackPlaying(true);
}

// ── Audio-file mode ───────────────────────────────────────────────────────────
function audioFileUrl(sentencePosition) {
  return `/static/audio/story_${STORY_ID}/sentence_${sentencePosition}.ogg`;
}

function seekbarPositionMs() {
  // Current playback position in the full story timeline (ms).
  if (!state.audioEl) return 0;
  return state.sentenceCumulative[state.audioSentenceIdx] + state.audioEl.currentTime * 1000;
}

function setCurrentTimeLabel(positionMs) {
  els.currentTime.textContent = formatDuration(positionMs);
}

function updateSeekbar() {
  if (state.seekbarDragging || state.totalDurationMs === 0) return;
  const positionMs = seekbarPositionMs();
  els.seekbar.value = Math.round(positionMs / state.totalDurationMs * 1000);
  setCurrentTimeLabel(positionMs);
}

function loadSentenceAudio(idx, startSec = 0) {
  if (idx >= state.sentenceSpans.length) {
    // Reached end of story.
    clearHighlight();
    setPlaybackPlaying(false);
    els.seekbar.value = 1000;
    setCurrentTimeLabel(state.totalDurationMs);
    return;
  }
  state.audioSentenceIdx = idx;
  setActiveIdx(idx);

  const sentence = state.story.sentences[idx];
  state.audioEl.src = audioFileUrl(sentence.position);
  state.audioEl.playbackRate = state.playbackRate;
  state.audioEl.currentTime = startSec;
  setCurrentTimeLabel(state.sentenceCumulative[idx] + startSec * 1000);
  state.audioEl.play().catch(() => {});
}

function stopAudio() {
  state.audioEl.pause();
  setPlaybackPlaying(false);
  // Keep highlight and position for resume.
}

function startAudio(idx = state.audioSentenceIdx, startSec = 0) {
  loadSentenceAudio(idx, startSec);
  setPlaybackPlaying(true);
}

function seekToAudioPosition(positionMs) {
  // Find which sentence contains positionMs.
  let idx = 0;
  for (let i = state.sentenceCumulative.length - 1; i >= 0; i--) {
    if (state.sentenceCumulative[i] <= positionMs) { idx = i; break; }
  }
  const offsetMs = positionMs - state.sentenceCumulative[idx];
  startAudio(idx, offsetMs / 1000);
}

// ── Play/stop button ──────────────────────────────────────────────────────────
els.playbackBtn.addEventListener('click', async () => {
  if (state.audioMode) {
    if (!state.audioEl.paused) {
      stopAudio();
    } else {
      startAudio(state.audioSentenceIdx, state.audioEl.currentTime);
    }
    return;
  }
  if (window.speechSynthesis.speaking) {
    stopSpeechPlayback();
  } else {
    await startSpeechPlayback();
  }
});


function escapeTooltipText(s) {
  return esc(String(s ?? '')).replace(/\n/g, '<br>');
}

function isWordNoted(baseWord) {
  return state.notedWords.some(word => word.baseWord === baseWord);
}

function buildWordTooltipHtml(word, sentenceEnglish) {
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
      esc(isWordNoted(word.baseWord) ? 'Already in noted words' : 'Click to add this word to noted words') +
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

// ── Seekbar interaction ───────────────────────────────────────────────────────
els.seekbar.addEventListener('mousedown', () => { state.seekbarDragging = true; });
els.seekbar.addEventListener('mouseup', () => {
  state.seekbarDragging = false;
  if (!state.audioMode) return;
  const posMs = els.seekbar.value / 1000 * state.totalDurationMs;
  const wasPlaying = !state.audioEl.paused;
  seekToAudioPosition(posMs);
  if (!wasPlaying) { state.audioEl.pause(); setPlaybackPlaying(false); }
});
els.seekbar.addEventListener('input', () => {
  if (!state.audioMode) return;
  const posMs = els.seekbar.value / 1000 * state.totalDurationMs;
  setCurrentTimeLabel(posMs);
});

// ── beforeunload cleanup ──────────────────────────────────────────────────────
window.addEventListener('beforeunload', () => {
  if (state.audioMode) state.audioEl?.pause();
  else stopSpeechPlayback();
});

// ── Keyboard shortcuts ────────────────────────────────────────────────────────
window.addEventListener('keydown', async e => {
  if (e.code !== 'Space') return;
  const activeEl = document.activeElement;
  const tag = activeEl?.tagName;
  if (tag === 'TEXTAREA' || tag === 'SELECT') return;
  if (tag === 'INPUT' && activeEl?.type !== 'range') return;
  e.preventDefault();
  els.playbackBtn.click();
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

// ── Generate audio ────────────────────────────────────────────────────────────
function openGenModal() {
  if (state.audioMode) { if (!state.audioEl.paused) stopAudio(); }
  else if (window.speechSynthesis.speaking) stopSpeechPlayback();
  // Always reset to confirmation state when opening.
  setModalGenerating(false);
  els.genModalBackdrop.classList.remove('hidden');
}

function closeGenModal() {
  if (state.generating) return;
  els.genModalBackdrop.classList.add('hidden');
}

function setModalGenerating(generating) {
  state.generating = generating;
  els.genConfirmBody.classList.toggle('hidden', generating);
  els.genProgressBody.classList.toggle('hidden', !generating);
  els.genModalCancel.classList.toggle('hidden', generating);
  els.genModalConfirm.classList.toggle('hidden', generating);
  els.genCancelGenerationBtn.classList.toggle('hidden', !generating);
  els.genModalDone.classList.add('hidden');
  els.genModalClose.disabled = generating;
}

function buildSentenceList() {
  els.genSentenceList.innerHTML = '';
  for (const sentence of state.story.sentences) {
    const text = sentence.words.map(w => w.displayWord).join('');
    const preview = text.length > 35 ? text.slice(0, 35) + '…' : text;

    const row = document.createElement('div');
    row.className = 'gen-sentence-row';
    row.id = `gen-row-${sentence.position}`;

    const icon = document.createElement('span');
    icon.className = 'gen-sentence-icon';
    const dot = document.createElement('span');
    dot.className = 'gen-pending-dot';
    icon.appendChild(dot);

    const previewEl = document.createElement('span');
    previewEl.className = 'gen-sentence-preview';
    previewEl.textContent = preview;

    row.appendChild(icon);
    row.appendChild(previewEl);
    els.genSentenceList.appendChild(row);
  }
}

function setRowActive(position) {
  const row = document.getElementById(`gen-row-${position}`);
  if (!row) return;
  row.classList.add('gen-sentence-row--active');
  const icon = row.querySelector('.gen-sentence-icon');
  icon.innerHTML = '<span class="spinner"></span>';
  row.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
}

function setRowDone(position) {
  const row = document.getElementById(`gen-row-${position}`);
  if (!row) return;
  row.classList.remove('gen-sentence-row--active');
  row.classList.add('gen-sentence-row--done');
  const icon = row.querySelector('.gen-sentence-icon');
  icon.innerHTML = '<span class="gen-checkmark">✓</span>';
}

function markNextRowActive(donePosition) {
  const positions = state.story.sentences.map(s => s.position);
  const idx = positions.indexOf(donePosition);
  if (idx < 0 || idx + 1 >= positions.length) return;
  const nextRow = document.getElementById(`gen-row-${positions[idx + 1]}`);
  if (nextRow && !nextRow.classList.contains('gen-sentence-row--done')) {
    setRowActive(positions[idx + 1]);
  }
}

// ── Generate translation ──────────────────────────────────────────────────────
function populateTranslationModelSelect(providers) {
  const hasProviders = PROVIDER_MODELS.some(p => providers[p.key]);
  const missingLines = PROVIDER_MODELS
    .filter(p => !providers[p.key])
    .map(p => p.label + ': set ' + p.envKey + ' to enable');
  const tip = missingLines.length ? missingLines.join('\n') + '\n— then restart the program' : null;

  let firstAvailSet = false;
  const optgroupsHtml = PROVIDER_MODELS.map(({ key, label, models }) => {
    const avail = providers[key];
    const groupLabel = avail ? label : label + ' — no API key';
    const options = models.map(([val, text], i) => {
      const sel = avail && !firstAvailSet && i === 0 ? ' selected' : '';
      if (sel) firstAvailSet = true;
      return '<option value="' + val + '"' + sel + '>' + text + '</option>';
    }).join('');
    return '<optgroup label="' + groupLabel + '"' + (avail ? '' : ' disabled') + '>' + options + '</optgroup>';
  }).join('');

  els.genTranslationModelSelect.innerHTML =
    (!hasProviders ? '<option value="" selected>no API keys configured</option>' : '') +
    optgroupsHtml;
  els.genTranslationModelSelect.disabled = !hasProviders;

  if (tip) {
    els.genTranslationProviderInfo.dataset.tooltip = tip;
    els.genTranslationProviderInfo.style.display = '';
  } else {
    els.genTranslationProviderInfo.style.display = 'none';
  }

  els.genTranslationModalConfirm.disabled = !hasProviders;
}

function setTranslationModalGenerating(generating) {
  state.translating = generating;
  els.genTranslationConfirmBody.classList.toggle('hidden', generating);
  els.genTranslationProgressBody.classList.toggle('hidden', !generating);
  els.genTranslationModalCancel.classList.toggle('hidden', generating);
  els.genTranslationModalCancelGen.classList.toggle('hidden', !generating);
  els.genTranslationModalConfirm.classList.toggle('hidden', generating);
  els.genTranslationModalDone.classList.add('hidden');
  els.genTranslationModalClose.disabled = generating;
}

function openTranslationModal() {
  setTranslationModalGenerating(false);
  els.genTranslationModalBackdrop.classList.remove('hidden');
}

function closeTranslationModal() {
  if (state.translating) return;
  els.genTranslationModalBackdrop.classList.add('hidden');
}

els.genTranslationBtn.addEventListener('click', openTranslationModal);
els.genTranslationModalClose.addEventListener('click', closeTranslationModal);
els.genTranslationModalCancel.addEventListener('click', closeTranslationModal);
els.genTranslationModalCancelGen.addEventListener('click', () => {
  if (state.translationController) state.translationController.abort();
});
els.genTranslationModalBackdrop.addEventListener('click', e => {
  if (e.target === els.genTranslationModalBackdrop) closeTranslationModal();
});

els.genTranslationModalConfirm.addEventListener('click', async () => {
  if (state.translationController) return;

  const aiModel = els.genTranslationModelSelect.value;
  if (!aiModel) return;

  state.translationController = new AbortController();
  setTranslationModalGenerating(true);
  els.genTranslationStatusText.textContent = 'Translating…';

  let allDone = false;
  let baseStatusText = 'Translating…';
  let elapsedSecs = 0;
  let elapsedTimer = null;

  const startElapsedTimer = () => {
    elapsedTimer = setInterval(() => {
      elapsedSecs++;
      els.genTranslationStatusText.textContent = `${baseStatusText} (${elapsedSecs}s)`;
    }, 1000);
  };
  const stopElapsedTimer = () => {
    if (elapsedTimer !== null) { clearInterval(elapsedTimer); elapsedTimer = null; }
  };

  try {
    const res = await fetch(`/api/stories/${STORY_ID}/generate-translation`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ai_model: aiModel }),
      signal: state.translationController.signal,
    });

    if (res.ok) {
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      while (true) {
        let value, done;
        try { ({ value, done } = await reader.read()); } catch (_) { break; }
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split('\n');
        buf = lines.pop();
        for (const line of lines) {
          if (!line.trim()) continue;
          let msg;
          try { msg = JSON.parse(line); } catch (_) { continue; }
          if (msg.status === 'translating') {
            baseStatusText =
              `Translating ${msg.sentenceCount} sentence${msg.sentenceCount !== 1 ? 's' : ''}` +
              (msg.wordCount > 0 ? ` and ${msg.wordCount} word${msg.wordCount !== 1 ? 's' : ''}` : '') +
              '…';
            els.genTranslationStatusText.textContent = baseStatusText;
            startElapsedTimer();
          } else if (msg.allDone) {
            allDone = true;
          }
        }
      }
    }
  } catch (_) {
    // Aborted or network error.
  }

  stopElapsedTimer();

  state.translationController = null;

  if (allDone) {
    playDing();
    state.translating = false;
    els.genTranslationSpinner.classList.add('hidden');
    els.genTranslationModalCancelGen.classList.add('hidden');
    els.genTranslationStatusText.textContent = baseStatusText.replace(/^Translating/, 'Translated').replace(/….*$/, '.');
    els.genTranslationModalConfirm.classList.add('hidden');
    els.genTranslationModalDone.classList.remove('hidden');
    els.genTranslationModalClose.disabled = false;
    // Reload the story to pick up new sentence translations and word glosses.
    const updated = await fetch(`/api/stories/${STORY_ID}`).then(r => r.json());
    renderStory(updated);
  } else {
    setTranslationModalGenerating(false);
    closeTranslationModal();
  }
});

els.genTranslationModalDone.addEventListener('click', closeTranslationModal);

els.genBtn.addEventListener('click', openGenModal);
els.genModalClose.addEventListener('click', closeGenModal);
els.genModalCancel.addEventListener('click', closeGenModal);
els.genModalBackdrop.addEventListener('click', e => { if (e.target === els.genModalBackdrop) closeGenModal(); });

els.genModalConfirm.addEventListener('click', async () => {
  if (state.generateController) return;

  const vv = getVoicevoxSettings();
  state.generateController = new AbortController();

  const total = state.story?.sentences.length ?? 0;
  let completed = 0;

  function updateProgressCount() {
    els.genProgressCount.textContent = `${completed} / ${total} sentences`;
  }

  buildSentenceList();
  setModalGenerating(true);
  updateProgressCount();
  if (total > 0) {
    setRowActive(state.story.sentences[0].position);
  }

  let allDone = false;
  try {
    const res = await fetch(`/api/stories/${STORY_ID}/generate-audio`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ speaker: vv.speaker, speedScale: vv.speedScale, intonationScale: vv.intonationScale }),
      signal: state.generateController.signal,
    });

    if (res.ok) {
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      outer: while (true) {
        let value, done;
        try { ({ value, done } = await reader.read()); } catch (_) { break; }
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split('\n');
        buf = lines.pop();
        for (const line of lines) {
          if (!line.trim()) continue;
          let msg;
          try { msg = JSON.parse(line); } catch (_) { continue; }
          if (msg.sentencePosition !== undefined) {
            completed++;
            updateProgressCount();
            setRowDone(msg.sentencePosition);
            markNextRowActive(msg.sentencePosition);
          } else if (msg.allDone) {
            allDone = true;
            break outer;
          }
        }
      }
    }
  } catch (_) {
    // Aborted or network error.
  }

  state.generateController = null;

  if (allDone) {
    playDing();

    // Unlock the modal so the user can close it manually; keep the progress view visible.
    state.generating = false;
    els.genCancelGenerationBtn.classList.add('hidden');
    els.genModalDone.classList.remove('hidden');
    els.genModalClose.disabled = false;
    const updated = await fetch(`/api/stories/${STORY_ID}`).then(r => r.json());
    applyAudioState(updated);
  } else {
    // Cancelled or error — close and reset immediately.
    setModalGenerating(false);
    closeGenModal();
  }
});

els.genCancelGenerationBtn.addEventListener('click', () => {
  state.generateController?.abort();
});

els.genModalDone.addEventListener('click', closeGenModal);

// ── Apply hasAudio state ──────────────────────────────────────────────────────
function applyAudioState(story) {
  state.story = story;
  if (!story.hasAudio) {
    els.seekbar.hidden = true;
    els.currentTime.hidden = true;
    els.duration.hidden = true;
    state.audioMode = false;
    return;
  }

  state.sentenceDurations = story.sentences.map(s => s.audioDurationMs ?? 0);
  state.sentenceCumulative = [];
  let cum = 0;
  for (const d of state.sentenceDurations) {
    state.sentenceCumulative.push(cum);
    cum += d;
  }
  state.totalDurationMs = cum;

  if (!state.audioEl) {
    state.audioEl = new Audio();
    state.audioEl.addEventListener('ended', () => {
      loadSentenceAudio(state.audioSentenceIdx + 1, 0);
    });
    state.audioEl.addEventListener('timeupdate', () => {
      updateSeekbar();
    });
    state.audioEl.addEventListener('pause', () => setPlaybackPlaying(false));
    state.audioEl.addEventListener('play', () => setPlaybackPlaying(true));
  }

  state.audioMode = true;
  state.audioSentenceIdx = 0;
  els.currentTime.hidden = false;
  els.seekbar.hidden = false;
  els.seekbar.value = 0;
  els.currentTime.textContent = '0:00';
  els.duration.hidden = false;
  els.duration.textContent = formatDuration(state.totalDurationMs);
}

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
          addHoveredWordToNotedWords().catch(() => {});
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

Promise.all([
  loadStory(STORY_ID),
  fetch('/api/providers').then(r => r.json()).catch(() => null),
]).then(([story, providers]) => {
  if (providers?.ai) state.providers = providers.ai;
  renderStory(story);
}).catch(renderError);
