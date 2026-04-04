import { getTtsVoice, getVoicevoxSettings, checkVoicevoxAvailable } from './common.js';

// ── DOM refs ──────────────────────────────────────────────────────────────────
const ttsBtn   = document.getElementById('story-tts-btn');
const ttsIcon  = document.getElementById('story-tts-icon');
const speedVal = document.getElementById('story-speed-val');
const seekbar  = document.getElementById('story-seekbar');
const genBtn   = document.getElementById('story-gen-btn');
const genModal = document.getElementById('story-gen-modal-backdrop');

// ── Playback state ────────────────────────────────────────────────────────────
let ttsText = '';
let ttsRate = 1.0;
let sentenceSpans = [];
let sentenceOffsets = [];  // char start of each sentence in ttsText (TTS mode)
let wordSpans = [];        // { absStart, span } for every word
let activeIdx = -1;
let activeWordSpan = null;
let resumeOffset = 0;
let lastWordAbsPos = 0;
let currentUtterance = null;

// Audio-file mode (used when story.hasAudio is true)
let audioMode = false;
let sentenceDurations = [];  // ms per sentence, parallel to story.sentences
let sentenceCumulative = []; // cumulative ms offsets, parallel (sentenceCumulative[i] = start of sentence i)
let totalDurationMs = 0;
let audioSentenceIdx = 0;    // which sentence file is currently loaded/playing
let audioEl = null;          // single reused <audio> element
let seekbarDragging = false;

// ── Story ID ──────────────────────────────────────────────────────────────────
function storyIdFromPath() {
  const parts = window.location.pathname.split('/').filter(Boolean);
  return parts[parts.length - 1];
}
const STORY_ID = storyIdFromPath();

// ── Speed stepper ─────────────────────────────────────────────────────────────
document.getElementById('story-speed-dec').addEventListener('click', () => {
  ttsRate = Math.max(0.5, parseFloat((ttsRate - 0.05).toFixed(2)));
  speedVal.value = ttsRate.toFixed(2);
});
document.getElementById('story-speed-inc').addEventListener('click', () => {
  ttsRate = Math.min(2.0, parseFloat((ttsRate + 0.05).toFixed(2)));
  speedVal.value = ttsRate.toFixed(2);
});

// ── Icons ─────────────────────────────────────────────────────────────────────
const ICON_PLAY = '<path d="M8 5v14l11-7z"/>';
const ICON_STOP = '<rect x="6" y="6" width="12" height="12"/>';

function setTtsPlaying(playing) {
  ttsIcon.innerHTML = playing ? ICON_STOP : ICON_PLAY;
  ttsBtn.setAttribute('aria-label', playing ? 'Stop reading' : 'Play story');
}

// ── Sentence / word highlight (shared by both modes) ──────────────────────────
function setActiveIdx(idx) {
  sentenceSpans[activeIdx]?.classList.remove('story-sentence--active');
  activeIdx = idx;
  const span = sentenceSpans[activeIdx];
  span?.classList.add('story-sentence--active');
  span?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
}

function clearHighlight() {
  sentenceSpans[activeIdx]?.classList.remove('story-sentence--active');
  activeIdx = -1;
  activeWordSpan?.classList.remove('story-word--active');
  activeWordSpan = null;
  resumeOffset = 0;
  lastWordAbsPos = 0;
}

// ── TTS mode ──────────────────────────────────────────────────────────────────
function highlightAt(charIndex) {
  lastWordAbsPos = resumeOffset + charIndex;
  const abs = lastWordAbsPos;

  let sIdx = 0;
  for (let i = 1; i < sentenceOffsets.length; i++) {
    if (sentenceOffsets[i] <= abs) sIdx = i;
    else break;
  }
  if (sIdx !== activeIdx) setActiveIdx(sIdx);

  let wIdx = 0;
  for (let i = 1; i < wordSpans.length; i++) {
    if (wordSpans[i].absStart <= abs) wIdx = i;
    else break;
  }
  const wordSpan = wordSpans[wIdx]?.span ?? null;
  if (wordSpan !== activeWordSpan) {
    activeWordSpan?.classList.remove('story-word--active');
    activeWordSpan = wordSpan;
    wordSpan?.classList.add('story-word--active');
  }
}

function stopTts() {
  resumeOffset = lastWordAbsPos;
  if (currentUtterance) {
    currentUtterance.onboundary = null;
    currentUtterance.onend = null;
    currentUtterance.onerror = null;
    currentUtterance = null;
  }
  window.speechSynthesis.cancel();
  setTtsPlaying(false);
}

async function startTts() {
  if (!speechSynthesis.getVoices().length) {
    await new Promise(resolve => speechSynthesis.addEventListener('voiceschanged', resolve, { once: true }));
  }
  currentUtterance = new SpeechSynthesisUtterance(ttsText.slice(resumeOffset));
  currentUtterance.lang = 'ja-JP';
  currentUtterance.rate = ttsRate;
  const voice = getTtsVoice('ja-JP');
  if (voice) currentUtterance.voice = voice;
  currentUtterance.onboundary = e => highlightAt(e.charIndex);
  currentUtterance.onend = () => { currentUtterance = null; clearHighlight(); setTtsPlaying(false); };
  currentUtterance.onerror = () => { currentUtterance = null; clearHighlight(); setTtsPlaying(false); };
  window.speechSynthesis.speak(currentUtterance);
  setTtsPlaying(true);
}

// ── Audio-file mode ───────────────────────────────────────────────────────────
function audioFileUrl(sentencePosition) {
  return `/static/audio/story_${STORY_ID}/sentence_${sentencePosition}.ogg`;
}

function seekbarPositionMs() {
  // Current playback position in the full story timeline (ms).
  if (!audioEl) return 0;
  return sentenceCumulative[audioSentenceIdx] + audioEl.currentTime * 1000;
}

function updateSeekbar() {
  if (seekbarDragging || totalDurationMs === 0) return;
  seekbar.value = Math.round(seekbarPositionMs() / totalDurationMs * 1000);
}

function loadSentenceAudio(idx, startSec = 0) {
  if (idx >= sentenceSpans.length) {
    // Reached end of story.
    clearHighlight();
    activeWordSpan?.classList.remove('story-word--active');
    activeWordSpan = null;
    setTtsPlaying(false);
    seekbar.value = 1000;
    return;
  }
  audioSentenceIdx = idx;
  setActiveIdx(idx);
  activeWordSpan?.classList.remove('story-word--active');
  activeWordSpan = null;

  const sentence = _story.sentences[idx];
  audioEl.src = audioFileUrl(sentence.position);
  audioEl.playbackRate = ttsRate;
  audioEl.currentTime = startSec;
  audioEl.play().catch(() => {});
}

function stopAudio() {
  audioEl.pause();
  setTtsPlaying(false);
  // Keep highlight and position for resume.
}

function startAudio(idx = audioSentenceIdx, startSec = 0) {
  loadSentenceAudio(idx, startSec);
  setTtsPlaying(true);
}

function seekToAudioPosition(positionMs) {
  // Find which sentence contains positionMs.
  let idx = 0;
  for (let i = sentenceCumulative.length - 1; i >= 0; i--) {
    if (sentenceCumulative[i] <= positionMs) { idx = i; break; }
  }
  const offsetMs = positionMs - sentenceCumulative[idx];
  startAudio(idx, offsetMs / 1000);
}

// ── Play/stop button ──────────────────────────────────────────────────────────
ttsBtn.addEventListener('click', async () => {
  if (audioMode) {
    if (!audioEl.paused) {
      stopAudio();
    } else {
      audioEl.playbackRate = ttsRate;
      audioEl.play().catch(() => {});
      setTtsPlaying(true);
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
  if (audioMode) {
    // Map absPos (char offset in ttsText) → sentence index → audio seek.
    let sIdx = 0;
    for (let i = 1; i < sentenceOffsets.length; i++) {
      if (sentenceOffsets[i] <= absPos) sIdx = i;
      else break;
    }
    startAudio(sIdx, 0);
    return;
  }
  if (currentUtterance) {
    currentUtterance.onboundary = null;
    currentUtterance.onend = null;
    currentUtterance.onerror = null;
    currentUtterance = null;
    window.speechSynthesis.cancel();
  }
  resumeOffset = absPos;
  lastWordAbsPos = absPos;
  await startTts();
}

// ── Seekbar interaction ───────────────────────────────────────────────────────
seekbar.addEventListener('mousedown', () => { seekbarDragging = true; });
seekbar.addEventListener('mouseup', () => {
  seekbarDragging = false;
  if (!audioMode) return;
  const posMs = seekbar.value / 1000 * totalDurationMs;
  const wasPlaying = !audioEl.paused;
  seekToAudioPosition(posMs);
  if (!wasPlaying) { audioEl.pause(); setTtsPlaying(false); }
});
seekbar.addEventListener('input', () => {
  // Update time display while dragging (visual only; seek happens on mouseup).
});

// ── beforeunload cleanup ──────────────────────────────────────────────────────
window.addEventListener('beforeunload', () => {
  if (audioMode) audioEl?.pause();
  else stopTts();
});

// ── Generate audio ────────────────────────────────────────────────────────────
let _generateController = null;
let _generating = false;

function openGenModal() {
  // Always reset to confirmation state when opening.
  setModalGenerating(false);
  genModal.classList.remove('hidden');
}

function closeGenModal() {
  if (_generating) return;
  genModal.classList.add('hidden');
}

function setModalGenerating(generating) {
  _generating = generating;
  document.getElementById('gen-confirm-body').classList.toggle('hidden', generating);
  document.getElementById('gen-progress-body').classList.toggle('hidden', !generating);
  document.getElementById('story-gen-modal-cancel').classList.toggle('hidden', generating);
  document.getElementById('story-gen-modal-confirm').classList.toggle('hidden', generating);
  document.getElementById('story-gen-cancel-generation-btn').classList.toggle('hidden', !generating);
  document.getElementById('story-gen-modal-done').classList.add('hidden');
  document.querySelector('#story-gen-modal-backdrop .modal-close').disabled = generating;
}

function buildSentenceList() {
  const list = document.getElementById('gen-sentence-list');
  list.innerHTML = '';
  for (const sentence of _story.sentences) {
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
    list.appendChild(row);
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
  const positions = _story.sentences.map(s => s.position);
  const idx = positions.indexOf(donePosition);
  if (idx < 0 || idx + 1 >= positions.length) return;
  const nextRow = document.getElementById(`gen-row-${positions[idx + 1]}`);
  if (nextRow && !nextRow.classList.contains('gen-sentence-row--done')) {
    setRowActive(positions[idx + 1]);
  }
}

genBtn.addEventListener('click', openGenModal);
document.querySelector('#story-gen-modal-backdrop .modal-close').addEventListener('click', closeGenModal);
document.getElementById('story-gen-modal-cancel').addEventListener('click', closeGenModal);
genModal.addEventListener('click', e => { if (e.target === genModal) closeGenModal(); });

document.getElementById('story-gen-modal-confirm').addEventListener('click', async () => {
  if (_generateController) return;

  const vv = getVoicevoxSettings();
  _generateController = new AbortController();

  buildSentenceList();
  setModalGenerating(true);
  if (_story?.sentences.length > 0) {
    setRowActive(_story.sentences[0].position);
  }

  let allDone = false;
  try {
    const res = await fetch(`/api/stories/${STORY_ID}/generate-audio`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ speaker: vv.speaker, speedScale: vv.speedScale, intonationScale: vv.intonationScale }),
      signal: _generateController.signal,
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

  _generateController = null;

  if (allDone) {
    // Unlock the modal so the user can close it manually; keep the progress view visible.
    _generating = false;
    document.getElementById('story-gen-cancel-generation-btn').classList.add('hidden');
    document.getElementById('story-gen-modal-done').classList.remove('hidden');
    document.querySelector('#story-gen-modal-backdrop .modal-close').disabled = false;
    const updated = await fetch(`/api/stories/${STORY_ID}`).then(r => r.json());
    applyAudioState(updated);
  } else {
    // Cancelled or error — close and reset immediately.
    setModalGenerating(false);
    closeGenModal();
  }
});

document.getElementById('story-gen-cancel-generation-btn').addEventListener('click', () => {
  _generateController?.abort();
});

document.getElementById('story-gen-modal-done').addEventListener('click', closeGenModal);

// ── Apply hasAudio state ──────────────────────────────────────────────────────
let _story = null;

function applyAudioState(story) {
  _story = story;
  if (!story.hasAudio) {
    seekbar.hidden = true;
    audioMode = false;
    return;
  }

  sentenceDurations = story.sentences.map(s => s.audioDurationMs ?? 0);
  sentenceCumulative = [];
  let cum = 0;
  for (const d of sentenceDurations) {
    sentenceCumulative.push(cum);
    cum += d;
  }
  totalDurationMs = cum;

  if (!audioEl) {
    audioEl = new Audio();
    audioEl.addEventListener('ended', () => {
      loadSentenceAudio(audioSentenceIdx + 1, 0);
    });
    audioEl.addEventListener('timeupdate', () => {
      updateSeekbar();
    });
    audioEl.addEventListener('pause', () => setTtsPlaying(false));
    audioEl.addEventListener('play', () => setTtsPlaying(true));
  }

  audioMode = true;
  audioSentenceIdx = 0;
  seekbar.hidden = false;
  seekbar.value = 0;
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
  _story = story;
  document.title = `${story.title} | Story`;
  document.getElementById('story-title').textContent = story.title;

  const SEPARATOR = '　';
  sentenceSpans = [];
  sentenceOffsets = [];
  wordSpans = [];
  const textParts = [];
  let offset = 0;
  for (const sentence of story.sentences) {
    const text = sentenceText(sentence);
    sentenceOffsets.push(offset);
    textParts.push(text);
    offset += text.length + SEPARATOR.length;
  }
  ttsText = textParts.join(SEPARATOR);
  ttsBtn.disabled = false;

  const content = document.getElementById('story-content');
  content.innerHTML = '';
  let currentParagraph = null;
  for (let i = 0; i < story.sentences.length; i++) {
    const sentence = story.sentences[i];
    if (!currentParagraph || sentence.isParagraphStart) {
      currentParagraph = document.createElement('p');
      currentParagraph.className = 'story-paragraph';
      content.appendChild(currentParagraph);
    }
    const sentenceSpan = document.createElement('span');
    sentenceSpan.className = 'story-sentence';

    let wordOffset = sentenceOffsets[i];
    for (const word of sentence.words) {
      const wordSpan = document.createElement('span');
      wordSpan.className = 'story-word';
      wordSpan.textContent = word.displayWord;
      const capturedOffset = wordOffset;
      wordSpan.addEventListener('click', () => seekToWord(capturedOffset));
      sentenceSpan.appendChild(wordSpan);
      wordSpans.push({ absStart: wordOffset, span: wordSpan });
      wordOffset += word.displayWord.length;
    }
    sentenceSpan.appendChild(document.createTextNode(' '));
    currentParagraph.appendChild(sentenceSpan);
    sentenceSpans.push(sentenceSpan);
  }

  // Enable generate button if VoiceVox is available.
  checkVoicevoxAvailable().then(available => {
    genBtn.disabled = !available;
  });

  applyAudioState(story);
}

function renderError() {
  document.getElementById('story-error').hidden = false;
}

loadStory(STORY_ID).then(renderStory).catch(renderError);
