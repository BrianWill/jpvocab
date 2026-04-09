import { getTtsVoice, getVoicevoxSettings } from './common.js';
import { getSynthAudio } from './synth-cache.js';

let _els, _state, _storyId;

// ── Speed stepper ─────────────────────────────────────────────────────────────
const SPEED_STEPPER_INTERVAL = 230;

function clampPlaybackRate(rate) {
  return Math.min(2.0, Math.max(0.5, parseFloat(rate.toFixed(2))));
}

async function restartPlaybackForRateChange() {
  if (_state.synthMode) {
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
const ICON_LOADING_SM = '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true" class="synth-spinner"><circle cx="12" cy="12" r="9" stroke-opacity="0.25"/><path stroke-linecap="round" d="M12 3a9 9 0 0 1 9 9"/></svg>';

function isSentencePlaying(idx) {
  if (_state.synthMode) return _state.audioSentenceIdx === idx && !!_state.audioEl && !_state.audioEl.paused;
  return _state.activeIdx === idx && window.speechSynthesis.speaking;
}

function updateSentencePlayBtnIcon() {
  const idx = _state.sentencePlayBtnTargetIdx;
  if (idx < 0) return;
  if (_state.synthLoadingIdx === idx) {
    _els.sentencePlayBtn.innerHTML = ICON_LOADING_SM;
    _els.sentencePlayBtn.classList.add('sentence-play-btn--loading');
    _els.sentencePlayBtn.setAttribute('aria-label', 'Generating audio…');
    return;
  }
  _els.sentencePlayBtn.classList.remove('sentence-play-btn--loading');
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

// ── Clause splitting ──────────────────────────────────────────────────────────
// Splits a sentence into clauses by breaking after any token whose displayWord
// contains a Japanese comma (、). Returns an array of word-token arrays.
export function splitByClause(sentence) {
  const clauses = [];
  let current = [];
  for (const word of sentence.words) {
    current.push(word);
    if (word.displayWord.includes('、')) {
      clauses.push(current);
      current = [];
    }
  }
  if (current.length > 0) clauses.push(current);
  return clauses.filter(c => c.length > 0);
}

// ── On-demand synthesis ───────────────────────────────────────────────────────
function synthClauseAudio(words) {
  const vv = getVoicevoxSettings();
  const text = words.map(w => w.displayWord).join('');
  return getSynthAudio(text, vv);
}

function prefetchClause(sentenceIdx, clauseIdx) {
  if (!_state.story || sentenceIdx >= _state.story.sentences.length) return;
  const clauses = splitByClause(_state.story.sentences[sentenceIdx]);
  if (clauseIdx >= clauses.length) return;
  synthClauseAudio(clauses[clauseIdx]).catch(() => {});
}

function createAudioElement() {
  const el = new Audio();
  el.addEventListener('ended', () => {
    const sIdx = _state.audioSentenceIdx;
    const cIdx = _state.audioClauseIdx;
    if (!_state.story) { clearHighlight(); setPlaybackPlaying(false); return; }
    const sentence = _state.story.sentences[sIdx];
    if (!sentence) { clearHighlight(); setPlaybackPlaying(false); return; }
    const clauses = splitByClause(sentence);
    if (cIdx + 1 < clauses.length) {
      loadClauseAudio(sIdx, cIdx + 1);
    } else {
      loadClauseAudio(sIdx + 1, 0);
    }
  });
  el.addEventListener('pause', () => setPlaybackPlaying(false));
  el.addEventListener('play', () => setPlaybackPlaying(true));
  return el;
}

async function loadClauseAudio(sentenceIdx, clauseIdx) {
  if (sentenceIdx >= _state.sentenceSpans.length) {
    // Reached end of story.
    clearHighlight();
    setPlaybackPlaying(false);
    return;
  }
  _state.audioSentenceIdx = sentenceIdx;
  _state.audioClauseIdx = clauseIdx;
  setActiveIdx(sentenceIdx);

  const sentence = _state.story.sentences[sentenceIdx];
  const clauses = splitByClause(sentence);

  if (clauseIdx >= clauses.length) {
    // No clause at this index; advance to next sentence.
    loadClauseAudio(sentenceIdx + 1, 0);
    return;
  }

  if (_state.synthMode) {
    const gen = ++_state.synthGeneration;
    let blobUrl;
    try {
      blobUrl = await synthClauseAudio(clauses[clauseIdx]);
    } catch (_) {
      if (_state.synthGeneration === gen) {
        _state.synthLoadingIdx = -1;
        clearHighlight();
        setPlaybackPlaying(false);
      }
      return;
    }
    if (_state.synthGeneration !== gen) return; // Superseded by a newer request or stop.
    _state.synthLoadingIdx = -1;
    _state.audioEl.src = blobUrl;
    _state.audioEl.playbackRate = _state.playbackRate;
    _state.audioEl.play().catch(() => {});
    // Prefetch next clause while this one plays. If this is the last clause of
    // the sentence, split the next sentence and prefetch its first clause.
    const isLastClause = clauseIdx + 1 >= clauses.length;
    if (isLastClause) {
      prefetchClause(sentenceIdx + 1, 0);
    } else {
      prefetchClause(sentenceIdx, clauseIdx + 1);
    }
  }
}

function stopAudio() {
  _state.audioEl.pause();
  _state.synthGeneration++;    // Invalidate any in-progress synthesis so it won't auto-play.
  _state.synthLoadingIdx = -1;
  setPlaybackPlaying(false);
  // Keep highlight and clause position for resume.
}

function startAudio(sentenceIdx = _state.audioSentenceIdx, clauseIdx = _state.audioClauseIdx) {
  loadClauseAudio(sentenceIdx, clauseIdx);
  setPlaybackPlaying(true);
}

// ── Exported stop (used by generate modals as stopPlayback callback) ──────────
export function stopPlayback() {
  if (_state.synthMode) { if (_state.audioEl && !_state.audioEl.paused) stopAudio(); }
  else if (window.speechSynthesis.speaking) stopSpeechPlayback();
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
      if (state.synthMode) stopAudio();
      else stopSpeechPlayback();
      els.sentencePlayBtn.innerHTML = ICON_PLAY_SM;
      els.sentencePlayBtn.setAttribute('aria-label', 'Play from this sentence');
      return;
    }

    if (state.synthMode) {
      // Stop whatever is playing immediately, then synthesize the new sentence.
      if (state.audioEl && !state.audioEl.paused) stopAudio();
      state.synthLoadingIdx = idx;
      updateSentencePlayBtnIcon();
      setPlaybackPlaying(true);
      loadClauseAudio(idx, 0);
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
        startAudio();
      }
      return;
    }
    if (window.speechSynthesis.speaking) {
      stopSpeechPlayback();
    } else {
      await startSpeechPlayback();
    }
  });

  window.addEventListener('beforeunload', () => {
    if (state.synthMode) state.audioEl?.pause();
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
