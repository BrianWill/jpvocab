// ── Settings modal step helpers ────────────────────────────────────────────
const STEPPER_INTERVAL = 230;

function adjustStepperInput(input, delta) {
  if (!input) return;
  const val = parseInt(input.value, 10) || 5;
  input.value = delta > 0
    ? Math.min(995, Math.floor(val / 5) * 5 + 5)
    : Math.max(5, Math.ceil(val / 5) * 5 - 5);
}

function capStepperInput(input) {
  if (input.value.length > 3) input.value = input.value.slice(0, 3);
  if (input.value === '0') input.value = '1';
}

export function attachNumberStepper(input, options = {}) {
  if (!input) return;

  const { onChange, onInput } = options;
  const [minusBtn, plusBtn] = input.closest('.num-stepper').querySelectorAll('.num-btn');
  let stepTimer = null;

  const stopStep = () => {
    clearInterval(stepTimer);
    stepTimer = null;
  };

  const startStep = delta => {
    adjustStepperInput(input, delta);
    onChange?.();
    stepTimer = setInterval(() => {
      adjustStepperInput(input, delta);
      onChange?.();
    }, STEPPER_INTERVAL);
  };

  minusBtn.addEventListener('mousedown', () => startStep(-5));
  minusBtn.addEventListener('mouseup', stopStep);
  minusBtn.addEventListener('mouseleave', stopStep);
  plusBtn.addEventListener('mousedown', () => startStep(5));
  plusBtn.addEventListener('mouseup', stopStep);
  plusBtn.addEventListener('mouseleave', stopStep);
  input.addEventListener('input', () => {
    capStepperInput(input);
    onInput?.();
  });
}

// ── Settings modal ─────────────────────────────────────────────────────────
export const DRILL_FILTER_KEYS = ['katakana', 'verbs', 'nouns', 'other'];

function injectSettingsModal() {
  if (document.getElementById('settings-modal-backdrop')) return;
  const el = document.createElement('div');
  el.id = 'settings-modal-backdrop';
  el.className = 'modal-backdrop hidden';
  el.innerHTML = `
    <div class="modal">
      <div class="modal-header">
        <span>Settings</span>
        <button class="modal-close">✕</button>
      </div>
      <div class="modal-body">
        <div class="settings-section-label">Drill defaults</div>
        <div class="restart-field">
          <label>Max total words</label>
          <div class="num-stepper">
            <button class="num-btn" type="button">−</button>
            <input type="number" id="settings-total-words" min="1">
            <button class="num-btn" type="button">+</button>
          </div>
        </div>
        <div class="restart-field">
          <label>Words per round</label>
          <div class="num-stepper">
            <button class="num-btn" type="button">−</button>
            <input type="number" id="settings-round-size" min="1">
            <button class="num-btn" type="button">+</button>
          </div>
        </div>
        <div class="restart-field restart-field-filter">
          <label>Word type</label>
          <div class="filter-chips">
            <button type="button" class="filter-chip" data-setting-filter="katakana">Katakana</button>
            <button type="button" class="filter-chip" data-setting-filter="verbs">Verbs</button>
            <button type="button" class="filter-chip" data-setting-filter="nouns">Nouns</button>
            <button type="button" class="filter-chip" data-setting-filter="other">Other</button>
          </div>
        </div>
        <div class="settings-section-label settings-section-label--spaced">TTS voices</div>
        <div class="restart-field">
          <label>Auto-play word in drill</label>
          <input type="checkbox" id="settings-tts-autoplay" class="settings-tts-autoplay">
        </div>
        <div class="restart-field">
          <label>Japanese voice</label>
          <select id="settings-tts-jp" class="settings-tts-select"></select>
        </div>
        <div class="restart-field">
          <label>English voice</label>
          <select id="settings-tts-en" class="settings-tts-select"></select>
        </div>
      </div>
      <div class="modal-footer">
        <button class="btn-cancel" id="settings-cancel-btn">Cancel</button>
        <button class="btn-save" id="settings-save-btn">Save</button>
      </div>
    </div>`;
  document.body.appendChild(el);
}

function populateTtsSelects() {
  const voices = speechSynthesis.getVoices();
  const fill = (selId, langPrefix, storageKey) => {
    const sel = document.getElementById(selId);
    if (!sel) return;
    const saved = localStorage.getItem(storageKey) ?? '';
    sel.innerHTML = '<option value="">Default</option>';
    voices.filter(v => v.lang.startsWith(langPrefix)).forEach(v => {
      const opt = document.createElement('option');
      opt.value = v.voiceURI;
      opt.textContent = v.name + (v.localService ? '' : ' ☁');
      opt.selected = v.voiceURI === saved;
      sel.appendChild(opt);
    });
  };
  fill('settings-tts-jp', 'ja', 'tts-voice-ja');
  fill('settings-tts-en', 'en', 'tts-voice-en');
}

const TTS_DEFAULTS = { ja: 'Kyoko', en: 'Daniel' };

export function getTtsVoice(lang) {
  const isJa = lang.startsWith('ja');
  const key = isJa ? 'tts-voice-ja' : 'tts-voice-en';
  const voices = speechSynthesis.getVoices();
  const uri = localStorage.getItem(key);
  if (uri) return voices.find(v => v.voiceURI === uri) ?? null;
  const preferredName = TTS_DEFAULTS[isJa ? 'ja' : 'en'];
  return voices.find(v => v.name === preferredName) ?? null;
}

export const WORD_TTS_RATE = 0.85;

export function isTtsAutoplayEnabled() {
  return localStorage.getItem('tts-autoplay') !== 'off';
}

function waitForVoices() {
  return new Promise(resolve => {
    const voices = speechSynthesis.getVoices();
    if (voices.length > 0) { resolve(); return; }
    speechSynthesis.addEventListener('voiceschanged', () => resolve(), { once: true });
  });
}

let _currentAudio = null;

export async function playTts(text, lang, rate = 1) {
  if (_currentAudio) {
    _currentAudio.pause();
    _currentAudio = null;
  }
  await waitForVoices();
  const utt = new SpeechSynthesisUtterance(text);
  utt.lang = lang;
  utt.rate = rate;
  const voice = getTtsVoice(lang);
  if (voice) utt.voice = voice;
  speechSynthesis.cancel();
  speechSynthesis.speak(utt);
}

// playWordAudio plays a word's generated audio file if available, else falls back to TTS.
export function playWordAudio(word) {
  if (word.hasWordAudio) {
    speechSynthesis.cancel();
    if (_currentAudio) { _currentAudio.pause(); }
    _currentAudio = new Audio(`/static/audio/${encodeURIComponent(word.word)}.wav`);
    _currentAudio.play();
  } else {
    playTts(word.word, 'ja-JP', WORD_TTS_RATE);
  }
}

// playSentenceAudio plays a word's generated sentence audio file if available, else falls back to TTS.
export function playSentenceAudio(word) {
  if (word.hasSentenceAudio) {
    speechSynthesis.cancel();
    if (_currentAudio) { _currentAudio.pause(); }
    _currentAudio = new Audio(`/static/audio/${encodeURIComponent(word.word)}_sentence.wav`);
    _currentAudio.play();
  } else if (word.exampleJp) {
    playTts(word.exampleJp, 'ja-JP', 0.75);
  }
}

function initializeSettings() {
  const settingsBtn = document.getElementById('settings-btn');
  const settingsModal = document.getElementById('settings-modal-backdrop');
  if (!settingsBtn || !settingsModal) return;

  const saveBtn = document.getElementById('settings-save-btn');
  const closeModal = () => settingsModal.classList.add('hidden');

  const setDirty = () => saveBtn?.classList.add('btn-save--dirty');
  const clearDirty = () => saveBtn?.classList.remove('btn-save--dirty');

  // Open: fetch current settings, populate, and reset dirty state
  settingsBtn.addEventListener('click', async () => {
    const resp = await fetch('/api/settings/drill');
    const settings = await resp.json();

    const totalInput = document.getElementById('settings-total-words');
    const roundInput = document.getElementById('settings-round-size');
    if (totalInput) totalInput.value = settings.maxWords;
    if (roundInput) roundInput.value = settings.roundSize;

    settingsModal.querySelectorAll('.filter-chip[data-setting-filter]').forEach(btn => {
      btn.classList.toggle('active', settings.wordTypes.includes(btn.dataset.settingFilter));
    });

    populateTtsSelects();
    const autoplayEl = document.getElementById('settings-tts-autoplay');
    if (autoplayEl) autoplayEl.checked = localStorage.getItem('tts-autoplay') !== 'off';
    clearDirty();
    settingsModal.classList.remove('hidden');
  });

  // Close
  settingsModal.querySelector('.modal-close')?.addEventListener('click', closeModal);
  document.getElementById('settings-cancel-btn')?.addEventListener('click', closeModal);
  settingsModal.addEventListener('click', (e) => {
    if (e.target === settingsModal) closeModal();
  });

  // Save
  saveBtn?.addEventListener('click', async () => {
    const totalVal = parseInt(document.getElementById('settings-total-words')?.value, 10);
    const roundVal = parseInt(document.getElementById('settings-round-size')?.value, 10);
    const wordTypes = DRILL_FILTER_KEYS.filter(f =>
      settingsModal.querySelector(`[data-setting-filter="${f}"]`)?.classList.contains('active')
    );
    if (wordTypes.length === 0) return;

    await fetch('/api/settings/drill', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        maxWords: Math.max(1, isNaN(totalVal) ? 100 : Math.min(995, totalVal)),
        roundSize: isNaN(roundVal) ? 10 : Math.max(1, Math.min(995, roundVal)),
        wordTypes,
      }),
    });

    const jpVoice = document.getElementById('settings-tts-jp')?.value ?? '';
    const enVoice = document.getElementById('settings-tts-en')?.value ?? '';
    if (jpVoice) localStorage.setItem('tts-voice-ja', jpVoice);
    else localStorage.removeItem('tts-voice-ja');
    if (enVoice) localStorage.setItem('tts-voice-en', enVoice);
    else localStorage.removeItem('tts-voice-en');
    const autoplay = document.getElementById('settings-tts-autoplay')?.checked ?? true;
    if (autoplay) localStorage.removeItem('tts-autoplay');
    else localStorage.setItem('tts-autoplay', 'off');

    closeModal();
  });

  // Filter chip toggles — mark dirty on change
  settingsModal.querySelectorAll('.filter-chip[data-setting-filter]').forEach(btn => {
    btn.addEventListener('click', () => { btn.classList.toggle('active'); setDirty(); });
  });

  // Stepper buttons — mark dirty on adjust; inputs mark dirty on manual edit
  const totalInput = document.getElementById('settings-total-words');
  if (totalInput) {
    attachNumberStepper(totalInput, {
      onChange: setDirty,
      onInput: setDirty,
    });
  }

  const roundInput = document.getElementById('settings-round-size');
  if (roundInput) {
    attachNumberStepper(roundInput, {
      onChange: setDirty,
      onInput: setDirty,
    });
  }

  const previewVoice = (voiceURI, lang, sample) => {
    const voice = voiceURI
      ? speechSynthesis.getVoices().find(v => v.voiceURI === voiceURI)
      : null;
    const utt = new SpeechSynthesisUtterance(sample);
    utt.lang = lang;
    if (voice) utt.voice = voice;
    speechSynthesis.cancel();
    speechSynthesis.speak(utt);
  };

  document.getElementById('settings-tts-autoplay')?.addEventListener('change', setDirty);
  document.getElementById('settings-tts-jp')?.addEventListener('change', e => {
    setDirty();
    previewVoice(e.target.value, 'ja-JP', 'こんにちは、よろしくお願いします。');
  });
  document.getElementById('settings-tts-en')?.addEventListener('change', e => {
    setDirty();
    previewVoice(e.target.value, 'en-US', 'This is a sample of the selected voice.');
  });
}

injectSettingsModal();
initializeSettings();

export function renderWordTooltipKanji(container, word, kanjiMap) {
  container.innerHTML = '';
  if (!word.kanjiData || word.kanjiData.length === 0) return;
  word.kanjiData.forEach(entry => {
    const kanji = kanjiMap[entry.id];
    if (!kanji) return;
    const isOn = /[\u30A0-\u30FF]/.test(entry.reading);
    const div = document.createElement('div');
    div.className = 'kanji-entry';
    div.innerHTML =
      '<div class="kanji-char">' + kanji.character + '</div>' +
      '<div class="kanji-detail">' +
        '<div class="kanji-readings"><span class="kanji-' + (isOn ? 'on' : 'kun') + '">' + entry.reading + '</span></div>' +
        '<div class="kanji-meanings">' + kanji.meanings.join(', ') + '</div>' +
      '</div>';
    container.appendChild(div);
  });
}

export function populateWordTooltip(tooltipEl, word, kanjiMap, renderReading) {
  tooltipEl.querySelector('[data-word-tooltip="word"]').textContent = word.word;
  tooltipEl.querySelector('[data-word-tooltip="reading"]').innerHTML =
    renderReading(word.reading, word.word, word.kanjiData);
  tooltipEl.querySelector('[data-word-tooltip="pos"]').textContent = word.type || '';
  tooltipEl.querySelector('[data-word-tooltip="meaning"]').textContent = word.meaning || '';
  tooltipEl.querySelector('[data-word-tooltip="example"]').textContent = word.exampleJp || '';
  tooltipEl.querySelector('[data-word-tooltip="example-en"]').textContent = word.exampleEn || '';

  const imgEl = tooltipEl.querySelector('[data-word-tooltip="image"]');
  if (word.imagePath) {
    imgEl.src = '/static/' + word.imagePath;
    imgEl.style.display = '';
  } else {
    imgEl.style.display = 'none';
  }

  renderWordTooltipKanji(
    tooltipEl.querySelector('[data-word-tooltip="kanji"]'),
    word,
    kanjiMap
  );
}

export function positionAnchoredWordTooltip(tooltipEl, options) {
  const { anchorRect, left } = options;
  tooltipEl.style.visibility = 'hidden';
  tooltipEl.classList.add('visible');

  const tooltipHeight = tooltipEl.offsetHeight;
  const maxTop = Math.max(8, window.innerHeight - tooltipHeight - 8);
  const top = Math.max(8, Math.min(anchorRect.top, maxTop));

  tooltipEl.style.left = left + 'px';
  tooltipEl.style.top = top + 'px';
  tooltipEl.style.visibility = '';
}
