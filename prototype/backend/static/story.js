import { getTtsVoice, getVoicevoxSettings, checkVoicevoxAvailable } from './common.js';

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
  seekbar: document.getElementById('story-seekbar'),
  speedDec: document.getElementById('story-speed-dec'),
  speedInc: document.getElementById('story-speed-inc'),
  speedVal: document.getElementById('story-speed-val'),
  storyContent: document.getElementById('story-content'),
  storyError: document.getElementById('story-error'),
  storyTitle: document.getElementById('story-title'),
  ttsBtn: document.getElementById('story-tts-btn'),
  ttsIcon: document.getElementById('story-tts-icon'),
};
els.genModalClose = els.genModalBackdrop.querySelector('.modal-close');

// ── Playback state ────────────────────────────────────────────────────────────
const state = {
  activeIdx: -1,
  audioEl: null,
  audioMode: false,
  audioSentenceIdx: 0,
  currentUtterance: null,
  generateController: null,
  generating: false,
  lastWordAbsPos: 0,
  resumeOffset: 0,
  seekbarDragging: false,
  sentenceCumulative: [],
  sentenceDurations: [],
  sentenceOffsets: [],
  sentenceSpans: [],
  story: null,
  totalDurationMs: 0,
  ttsRate: 1.0,
  ttsText: '',
};

// ── Story ID ──────────────────────────────────────────────────────────────────
function storyIdFromPath() {
  const parts = window.location.pathname.split('/').filter(Boolean);
  return parts[parts.length - 1];
}
const STORY_ID = storyIdFromPath();

// ── Speed stepper ─────────────────────────────────────────────────────────────
els.speedDec.addEventListener('click', () => {
  state.ttsRate = Math.max(0.5, parseFloat((state.ttsRate - 0.05).toFixed(2)));
  els.speedVal.value = state.ttsRate.toFixed(2);
});
els.speedInc.addEventListener('click', () => {
  state.ttsRate = Math.min(2.0, parseFloat((state.ttsRate + 0.05).toFixed(2)));
  els.speedVal.value = state.ttsRate.toFixed(2);
});

// ── Icons ─────────────────────────────────────────────────────────────────────
const ICON_PLAY = '<path d="M8 5v14l11-7z"/>';
const ICON_STOP = '<rect x="6" y="6" width="12" height="12"/>';

function setTtsPlaying(playing) {
  els.ttsIcon.innerHTML = playing ? ICON_STOP : ICON_PLAY;
  els.ttsBtn.setAttribute('aria-label', playing ? 'Stop reading' : 'Play story');
}

// ── Sentence / word highlight (shared by both modes) ──────────────────────────
function setActiveIdx(idx) {
  state.sentenceSpans[state.activeIdx]?.classList.remove('story-sentence--active');
  state.activeIdx = idx;
  const span = state.sentenceSpans[state.activeIdx];
  span?.classList.add('story-sentence--active');
  span?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
}

function clearHighlight() {
  state.sentenceSpans[state.activeIdx]?.classList.remove('story-sentence--active');
  state.activeIdx = -1;
  state.resumeOffset = 0;
  state.lastWordAbsPos = 0;
}

// ── TTS mode ──────────────────────────────────────────────────────────────────
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

function stopTts() {
  state.resumeOffset = state.lastWordAbsPos;
  if (state.currentUtterance) {
    state.currentUtterance.onboundary = null;
    state.currentUtterance.onend = null;
    state.currentUtterance.onerror = null;
    state.currentUtterance = null;
  }
  window.speechSynthesis.cancel();
  setTtsPlaying(false);
}

async function startTts() {
  if (!speechSynthesis.getVoices().length) {
    await new Promise(resolve => speechSynthesis.addEventListener('voiceschanged', resolve, { once: true }));
  }
  state.currentUtterance = new SpeechSynthesisUtterance(state.ttsText.slice(state.resumeOffset));
  state.currentUtterance.lang = 'ja-JP';
  state.currentUtterance.rate = state.ttsRate;
  const voice = getTtsVoice('ja-JP');
  if (voice) state.currentUtterance.voice = voice;
  state.currentUtterance.onboundary = e => highlightAt(e.charIndex);
  state.currentUtterance.onend = () => { state.currentUtterance = null; clearHighlight(); setTtsPlaying(false); };
  state.currentUtterance.onerror = () => { state.currentUtterance = null; clearHighlight(); setTtsPlaying(false); };
  window.speechSynthesis.speak(state.currentUtterance);
  setTtsPlaying(true);
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

function updateSeekbar() {
  if (state.seekbarDragging || state.totalDurationMs === 0) return;
  els.seekbar.value = Math.round(seekbarPositionMs() / state.totalDurationMs * 1000);
}

function loadSentenceAudio(idx, startSec = 0) {
  if (idx >= state.sentenceSpans.length) {
    // Reached end of story.
    clearHighlight();
    setTtsPlaying(false);
    els.seekbar.value = 1000;
    return;
  }
  state.audioSentenceIdx = idx;
  setActiveIdx(idx);

  const sentence = state.story.sentences[idx];
  state.audioEl.src = audioFileUrl(sentence.position);
  state.audioEl.playbackRate = state.ttsRate;
  state.audioEl.currentTime = startSec;
  state.audioEl.play().catch(() => {});
}

function stopAudio() {
  state.audioEl.pause();
  setTtsPlaying(false);
  // Keep highlight and position for resume.
}

function startAudio(idx = state.audioSentenceIdx, startSec = 0) {
  loadSentenceAudio(idx, startSec);
  setTtsPlaying(true);
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
els.ttsBtn.addEventListener('click', async () => {
  if (state.audioMode) {
    if (!state.audioEl.paused) {
      stopAudio();
    } else {
      startAudio(state.audioSentenceIdx, state.audioEl.currentTime);
    }
    return;
  }
  if (window.speechSynthesis.speaking) {
    stopTts();
  } else {
    await startTts();
  }
});

// ── Word click — seek to that sentence (both modes) ───────────────────────────
async function seekToWord(absPos) {
  let sIdx = 0;
  for (let i = 1; i < state.sentenceOffsets.length; i++) {
    if (state.sentenceOffsets[i] <= absPos) sIdx = i;
    else break;
  }

  if (state.audioMode) {
    if (sIdx === state.activeIdx && !state.audioEl.paused) { stopAudio(); return; }
    startAudio(sIdx, 0);
    return;
  }

  if (sIdx === state.activeIdx && window.speechSynthesis.speaking) { stopTts(); return; }
  if (state.currentUtterance) {
    state.currentUtterance.onboundary = null;
    state.currentUtterance.onend = null;
    state.currentUtterance.onerror = null;
    state.currentUtterance = null;
    window.speechSynthesis.cancel();
  }
  state.resumeOffset = absPos;
  state.lastWordAbsPos = absPos;
  await startTts();
}

// ── Seekbar interaction ───────────────────────────────────────────────────────
els.seekbar.addEventListener('mousedown', () => { state.seekbarDragging = true; });
els.seekbar.addEventListener('mouseup', () => {
  state.seekbarDragging = false;
  if (!state.audioMode) return;
  const posMs = els.seekbar.value / 1000 * state.totalDurationMs;
  const wasPlaying = !state.audioEl.paused;
  seekToAudioPosition(posMs);
  if (!wasPlaying) { state.audioEl.pause(); setTtsPlaying(false); }
});
els.seekbar.addEventListener('input', () => {
  // Update time display while dragging (visual only; seek happens on mouseup).
});

// ── beforeunload cleanup ──────────────────────────────────────────────────────
window.addEventListener('beforeunload', () => {
  if (state.audioMode) state.audioEl?.pause();
  else stopTts();
});

// ── Keyboard shortcuts ────────────────────────────────────────────────────────
window.addEventListener('keydown', async e => {
  if (e.code !== 'Space') return;
  const tag = document.activeElement?.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
  e.preventDefault();
  els.ttsBtn.click();
});

// ── Generate audio ────────────────────────────────────────────────────────────
function openGenModal() {
  if (state.audioMode) { if (!state.audioEl.paused) stopAudio(); }
  else if (window.speechSynthesis.speaking) stopTts();
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
    state.audioEl.addEventListener('pause', () => setTtsPlaying(false));
    state.audioEl.addEventListener('play', () => setTtsPlaying(true));
  }

  state.audioMode = true;
  state.audioSentenceIdx = 0;
  els.seekbar.hidden = false;
  els.seekbar.value = 0;
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
  state.ttsText = textParts.join(SEPARATOR);
  els.ttsBtn.disabled = false;

  els.storyContent.innerHTML = '';
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

    let wordOffset = state.sentenceOffsets[i];
    for (const word of sentence.words) {
      const wordSpan = document.createElement('span');
      wordSpan.className = 'story-word';
      wordSpan.textContent = word.displayWord;
      const capturedOffset = wordOffset;
      wordSpan.addEventListener('click', () => seekToWord(capturedOffset));
      sentenceSpan.appendChild(wordSpan);
      wordOffset += word.displayWord.length;
    }
    currentParagraph.appendChild(sentenceSpan);
    currentParagraph.appendChild(document.createTextNode(' '));
    state.sentenceSpans.push(sentenceSpan);
  }

  // Enable generate button if VoiceVox is available.
  checkVoicevoxAvailable().then(available => {
    els.genBtn.disabled = !available;
  });

  applyAudioState(story);
}

function renderError() {
  els.storyError.hidden = false;
}

loadStory(STORY_ID).then(renderStory).catch(renderError);
