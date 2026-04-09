import { getTtsVoice, getVoicevoxSettings } from './common.js';

let _els, _state, _storyId;

// ── Helpers ───────────────────────────────────────────────────────────────────
function formatDuration(ms) {
  const totalSeconds = Math.max(0, Math.round(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
}

// ── Speed stepper ─────────────────────────────────────────────────────────────
const SPEED_STEPPER_INTERVAL = 230;

function clampPlaybackRate(rate) {
  return Math.min(2.0, Math.max(0.5, parseFloat(rate.toFixed(2))));
}

async function restartPlaybackForRateChange() {
  if (_state.audioMode) {
    if (_state.audioEl) _state.audioEl.playbackRate = _state.playbackRate;
    return;
  }
  if (!window.speechSynthesis.speaking) return;
  stopSpeechPlayback();
  await startSpeechPlayback();
}

async function setPlaybackRate(nextRate) {
  const clamped = clampPlaybackRate(nextRate);
  if (clamped === _state.playbackRate) {
    _els.speedVal.textContent = clamped.toFixed(2);
    return;
  }
  _state.playbackRate = clamped;
  _els.speedVal.textContent = _state.playbackRate.toFixed(2);
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
    setPlaybackRate(_state.playbackRate + delta);
    stepTimer = setInterval(() => {
      setPlaybackRate(_state.playbackRate + delta);
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
    setPlaybackRate(_state.playbackRate + delta);
  });
}

// ── Icons ─────────────────────────────────────────────────────────────────────
const ICON_PLAY = '<path d="M8 5v14l11-7z"/>';
const ICON_STOP = '<rect x="6" y="6" width="12" height="12"/>';
const ICON_PLAY_SM = '<svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M8 5v14l11-7z"/></svg>';
const ICON_STOP_SM = '<svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><rect x="6" y="6" width="12" height="12"/></svg>';

function isSentencePlaying(idx) {
  if (_state.synthMode || _state.audioMode) return _state.audioSentenceIdx === idx && !!_state.audioEl && !_state.audioEl.paused;
  return _state.activeIdx === idx && window.speechSynthesis.speaking;
}

function updateSentencePlayBtnIcon() {
  const idx = _state.sentencePlayBtnTargetIdx;
  if (idx < 0) return;
  const playing = isSentencePlaying(idx);
  _els.sentencePlayBtn.innerHTML = playing ? ICON_STOP_SM : ICON_PLAY_SM;
  _els.sentencePlayBtn.setAttribute('aria-label', playing ? 'Stop' : 'Play from this sentence');
}

function setPlaybackPlaying(playing) {
  _els.playbackIcon.innerHTML = playing ? ICON_STOP : ICON_PLAY;
  _els.playbackBtn.setAttribute('aria-label', playing ? 'Stop reading' : 'Play story');
  updateSentencePlayBtnIcon();
}

// ── Sentence / word highlight (shared by both modes) ──────────────────────────
function setActiveIdx(idx) {
  _state.sentenceSpans[_state.activeIdx]?.classList.remove('story-sentence--active');
  _state.activeIdx = idx;
  const span = _state.sentenceSpans[_state.activeIdx];
  span?.classList.add('story-sentence--active');
  span?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  updateSentencePlayBtnIcon();
}

function clearHighlight() {
  _state.sentenceSpans[_state.activeIdx]?.classList.remove('story-sentence--active');
  _state.activeIdx = -1;
  _state.resumeOffset = 0;
  _state.lastWordAbsPos = 0;
}

// ── Floating sentence play button ─────────────────────────────────────────────
export function hideSentencePlayBtn() {
  _els.sentencePlayBtn.style.opacity = '0';
  _els.sentencePlayBtn.style.pointerEvents = 'none';
  _state.sentencePlayBtnTargetIdx = -1;
}

export function scheduleSentencePlayHide() {
  if (_state.sentencePlayHideTimer !== null) clearTimeout(_state.sentencePlayHideTimer);
  _state.sentencePlayHideTimer = setTimeout(() => {
    _state.sentencePlayHideTimer = null;
    hideSentencePlayBtn();
  }, 230);
}

export function cancelSentencePlayHide() {
  if (_state.sentencePlayHideTimer !== null) {
    clearTimeout(_state.sentencePlayHideTimer);
    _state.sentencePlayHideTimer = null;
  }
}

export function showSentencePlayBtn(idx) {
  cancelSentencePlayHide();
  _state.sentencePlayBtnTargetIdx = idx;
  const span = _state.sentenceSpans[idx];
  const firstWord = span?.querySelector('.story-word');
  const rect = (firstWord ?? span)?.getBoundingClientRect();
  if (!rect) return;
  const btnSize = _els.sentencePlayBtn.offsetHeight || 20;
  _els.sentencePlayBtn.style.opacity = '1';
  _els.sentencePlayBtn.style.pointerEvents = 'auto';
  _els.sentencePlayBtn.style.left = `${rect.left - btnSize / 1.4}px`;
  _els.sentencePlayBtn.style.top = `${rect.top - btnSize / 1.4}px`;
  updateSentencePlayBtnIcon();
}

// ── Speech-synthesis mode ─────────────────────────────────────────────────────
function highlightAt(charIndex) {
  _state.lastWordAbsPos = _state.resumeOffset + charIndex;
  const abs = _state.lastWordAbsPos;

  let sIdx = 0;
  for (let i = 1; i < _state.sentenceOffsets.length; i++) {
    if (_state.sentenceOffsets[i] <= abs) sIdx = i;
    else break;
  }
  if (sIdx !== _state.activeIdx) setActiveIdx(sIdx);
}

function stopSpeechPlayback() {
  _state.resumeOffset = _state.lastWordAbsPos;
  if (_state.currentUtterance) {
    _state.currentUtterance.onboundary = null;
    _state.currentUtterance.onend = null;
    _state.currentUtterance.onerror = null;
    _state.currentUtterance = null;
  }
  window.speechSynthesis.cancel();
  setPlaybackPlaying(false);
}

async function startSpeechPlayback() {
  if (!speechSynthesis.getVoices().length) {
    await new Promise(resolve => speechSynthesis.addEventListener('voiceschanged', resolve, { once: true }));
  }
  _state.currentUtterance = new SpeechSynthesisUtterance(_state.speechText.slice(_state.resumeOffset));
  _state.currentUtterance.lang = 'ja-JP';
  _state.currentUtterance.rate = _state.playbackRate;
  const voice = getTtsVoice('ja-JP');
  if (voice) _state.currentUtterance.voice = voice;
  _state.currentUtterance.onboundary = e => highlightAt(e.charIndex);
  _state.currentUtterance.onend = () => { _state.currentUtterance = null; clearHighlight(); setPlaybackPlaying(false); };
  _state.currentUtterance.onerror = () => { _state.currentUtterance = null; clearHighlight(); setPlaybackPlaying(false); };
  window.speechSynthesis.speak(_state.currentUtterance);
  setPlaybackPlaying(true);
}

// ── On-demand synthesis ───────────────────────────────────────────────────────
async function synthSentenceAudio(sentence) {
  const vv = getVoicevoxSettings();
  const text = sentence.words.map(w => w.displayWord).join('');
  const res = await fetch('/api/voicevox/synthesize', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      text,
      speaker: vv.speaker,
      speedScale: vv.speedScale,
      intonationScale: vv.intonationScale,
    }),
  });
  if (!res.ok) throw new Error(`synthesis failed: ${res.status}`);
  const blob = await res.blob();
  return URL.createObjectURL(blob);
}

function createAudioElement() {
  const el = new Audio();
  el.addEventListener('ended', () => { loadSentenceAudio(_state.audioSentenceIdx + 1, 0); });
  el.addEventListener('timeupdate', () => { updateSeekbar(); });
  el.addEventListener('pause', () => setPlaybackPlaying(false));
  el.addEventListener('play', () => setPlaybackPlaying(true));
  return el;
}

// ── Audio-file mode ───────────────────────────────────────────────────────────
function audioFileUrl(sentencePosition) {
  return `/static/audio/story_${_storyId}/sentence_${sentencePosition}.ogg`;
}

function seekbarPositionMs() {
  if (!_state.audioEl) return 0;
  return _state.sentenceCumulative[_state.audioSentenceIdx] + _state.audioEl.currentTime * 1000;
}

function setCurrentTimeLabel(positionMs) {
  _els.currentTime.textContent = formatDuration(positionMs);
}

function updateSeekbar() {
  if (_state.seekbarDragging || _state.totalDurationMs === 0) return;
  const positionMs = seekbarPositionMs();
  _els.seekbar.value = Math.round(positionMs / _state.totalDurationMs * 1000);
  setCurrentTimeLabel(positionMs);
}

async function loadSentenceAudio(idx, startSec = 0) {
  if (idx >= _state.sentenceSpans.length) {
    // Reached end of story.
    clearHighlight();
    setPlaybackPlaying(false);
    if (!_state.synthMode) {
      _els.seekbar.value = 1000;
      setCurrentTimeLabel(_state.totalDurationMs);
    }
    return;
  }
  _state.audioSentenceIdx = idx;
  setActiveIdx(idx);

  const sentence = _state.story.sentences[idx];

  if (_state.synthMode) {
    const oldSrc = _state.audioEl.src;
    let blobUrl;
    try {
      blobUrl = await synthSentenceAudio(sentence);
    } catch (_) {
      clearHighlight();
      setPlaybackPlaying(false);
      return;
    }
    _state.audioEl.src = blobUrl;
    if (oldSrc.startsWith('blob:')) URL.revokeObjectURL(oldSrc);
    _state.audioEl.playbackRate = _state.playbackRate;
    _state.audioEl.play().catch(() => {});
  } else {
    _state.audioEl.src = audioFileUrl(sentence.position);
    _state.audioEl.playbackRate = _state.playbackRate;
    _state.audioEl.currentTime = startSec;
    setCurrentTimeLabel(_state.sentenceCumulative[idx] + startSec * 1000);
    _state.audioEl.play().catch(() => {});
  }
}

function stopAudio() {
  _state.audioEl.pause();
  setPlaybackPlaying(false);
  // Keep highlight and position for resume.
}

function startAudio(idx = _state.audioSentenceIdx, startSec = 0) {
  loadSentenceAudio(idx, startSec);
  setPlaybackPlaying(true);
}

function seekToAudioPosition(positionMs) {
  let idx = 0;
  for (let i = _state.sentenceCumulative.length - 1; i >= 0; i--) {
    if (_state.sentenceCumulative[i] <= positionMs) { idx = i; break; }
  }
  const offsetMs = positionMs - _state.sentenceCumulative[idx];
  startAudio(idx, offsetMs / 1000);
}

// ── Exported stop (used by generate modals as stopPlayback callback) ──────────
export function stopPlayback() {
  if (_state.synthMode || _state.audioMode) { if (_state.audioEl && !_state.audioEl.paused) stopAudio(); }
  else if (window.speechSynthesis.speaking) stopSpeechPlayback();
}

// ── Apply hasAudio state ──────────────────────────────────────────────────────
export function applyAudioState(story) {
  _state.story = story;
  if (!story.hasAudio) {
    _els.seekbar.hidden = true;
    _els.currentTime.hidden = true;
    _els.duration.hidden = true;
    _state.audioMode = false;
    return;
  }

  _state.sentenceDurations = story.sentences.map(s => s.audioDurationMs ?? 0);
  _state.sentenceCumulative = [];
  let cum = 0;
  for (const d of _state.sentenceDurations) {
    _state.sentenceCumulative.push(cum);
    cum += d;
  }
  _state.totalDurationMs = cum;

  if (!_state.audioEl) {
    _state.audioEl = createAudioElement();
  }

  _state.audioMode = true;
  _state.audioSentenceIdx = 0;
  _els.currentTime.hidden = false;
  _els.seekbar.hidden = false;
  _els.seekbar.value = 0;
  _els.currentTime.textContent = '0:00';
  _els.duration.hidden = false;
  _els.duration.textContent = formatDuration(_state.totalDurationMs);
}

// ── Synth-mode init ───────────────────────────────────────────────────────────
// Called when VoiceVox is confirmed available. Overrides pre-generated file mode.
export function initSynthPlayback() {
  _state.synthMode = true;
  _state.audioMode = false;
  _els.seekbar.hidden = true;
  _els.currentTime.hidden = true;
  _els.duration.hidden = true;
  if (!_state.audioEl) {
    _state.audioEl = createAudioElement();
  }
}

// ── Init ──────────────────────────────────────────────────────────────────────
export function initPlayback(els, state, storyId) {
  _els = els;
  _state = state;
  _storyId = storyId;

  attachHoldRateButton(els.speedDec, -0.05);
  attachHoldRateButton(els.speedInc, 0.05);

  els.sentencePlayBtn.addEventListener('mouseenter', cancelSentencePlayHide);
  els.sentencePlayBtn.addEventListener('mouseleave', scheduleSentencePlayHide);
  els.sentencePlayBtn.addEventListener('click', async () => {
    const idx = state.sentencePlayBtnTargetIdx;
    if (idx < 0) return;

    if (isSentencePlaying(idx)) {
      if (state.synthMode || state.audioMode) stopAudio();
      else stopSpeechPlayback();
      els.sentencePlayBtn.innerHTML = ICON_PLAY_SM;
      els.sentencePlayBtn.setAttribute('aria-label', 'Play from this sentence');
      return;
    }

    if (state.synthMode || state.audioMode) {
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

  els.playbackBtn.addEventListener('click', async () => {
    if (state.synthMode) {
      if (state.audioEl && !state.audioEl.paused) {
        stopAudio();
      } else {
        startAudio(state.audioSentenceIdx, 0);
      }
      return;
    }
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

  window.addEventListener('beforeunload', () => {
    if (state.synthMode || state.audioMode) state.audioEl?.pause();
    else stopSpeechPlayback();
  });

  window.addEventListener('keydown', async e => {
    if (e.code !== 'Space') return;
    const activeEl = document.activeElement;
    const tag = activeEl?.tagName;
    if (tag === 'TEXTAREA' || tag === 'SELECT') return;
    if (tag === 'INPUT' && activeEl?.type !== 'range') return;
    e.preventDefault();
    els.playbackBtn.click();
  });
}
