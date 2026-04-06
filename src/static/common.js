// ── VoiceVox bulk audio generation state ─────────────────────────────────
let _vvGenController = null; // non-null while generation is in progress
let _vvGenPending = 0;
let _vvGenTotal = 0;

// ── Settings modal step helpers ────────────────────────────────────────────
const STEPPER_INTERVAL = 230;

function adjustStepperInput(input, delta, step, min, max) {
  if (!input) return;
  const val = parseInt(input.value, 10) || min;
  input.value = delta > 0
    ? Math.min(max, Math.floor(val / step) * step + step)
    : Math.max(min, Math.ceil(val / step) * step - step);
}

function capStepperInput(input, min, max) {
  const maxLen = String(max).length;
  if (input.value.length > maxLen) input.value = input.value.slice(0, maxLen);
  if (parseInt(input.value, 10) < min) input.value = String(min);
}

export function attachNumberStepper(input, options = {}) {
  if (!input) return;

  const { onChange, onInput, step = 5, min = 5, max = 995 } = options;
  const [minusBtn, plusBtn] = input.closest('.num-stepper').querySelectorAll('.num-btn');
  let stepTimer = null;

  const stopStep = () => {
    clearInterval(stepTimer);
    stepTimer = null;
  };

  const startStep = delta => {
    adjustStepperInput(input, delta, step, min, max);
    onChange?.();
    stepTimer = setInterval(() => {
      adjustStepperInput(input, delta, step, min, max);
      onChange?.();
    }, STEPPER_INTERVAL);
  };

  minusBtn.addEventListener('mousedown', () => startStep(-step));
  minusBtn.addEventListener('mouseup', stopStep);
  minusBtn.addEventListener('mouseleave', stopStep);
  plusBtn.addEventListener('mousedown', () => startStep(step));
  plusBtn.addEventListener('mouseup', stopStep);
  plusBtn.addEventListener('mouseleave', stopStep);
  input.addEventListener('input', () => {
    capStepperInput(input, min, max);
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
    <div class="modal settings-modal">
      <div class="modal-header">
        <span>Settings</span>
        <button class="modal-close">✕</button>
      </div>
      <div class="modal-body settings-body">
        <div class="settings-col">
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
          <div class="restart-field">
            <label>New word target drills</label>
            <div class="num-stepper">
              <button class="num-btn" type="button">−</button>
              <input type="number" id="settings-new-word-target" min="1">
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
        </div>
        <div class="settings-col-divider"></div>
        <div class="settings-col" style="flex:3">
          <div class="settings-section-label">TTS voices</div>
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
          <div class="settings-section-label settings-section-label--spaced">VoiceVox</div>
          <div class="restart-field">
            <label>Voice</label>
            <div style="display:flex;align-items:center;gap:0.4rem">
              <select id="settings-vv-speaker" class="settings-tts-select"><option value="1">Loading…</option></select>
              <span class="provider-info-icon" data-tooltip="VoiceVox must be running on this &#10;machine at http://localhost:50021&#10;&#10;Download: https://voicevox.hiroshiba.jp/">?</span>
            </div>
          </div>
          <div class="restart-field">
            <label>Speed</label>
            <div class="settings-slider-row">
              <input type="range" id="settings-vv-speed" min="0.5" max="2.0" step="0.05" value="1.0">
              <span id="settings-vv-speed-val" class="settings-slider-val">1.00</span>
            </div>
          </div>
          <div class="restart-field">
            <label>Intonation</label>
            <div class="settings-slider-row">
              <input type="range" id="settings-vv-intonation" min="0.0" max="2.0" step="0.05" value="1.5">
              <span id="settings-vv-intonation-val" class="settings-slider-val">1.50</span>
            </div>
          </div>
          <div class="restart-field">
            <label></label>
            <div style="display:flex;gap:0.5rem;align-items:center">
              <div id="settings-vv-gen-all-wrap">
                <button type="button" id="settings-vv-gen-all" class="btn-save" disabled>Generate audio for all words</button>
                <button type="button" id="settings-vv-gen-cancel" class="btn-danger hidden" style="white-space:nowrap"><span class="spinner"></span><span id="settings-vv-gen-cancel-label" style="margin-left:0.4rem"></span></button>
              </div>
              <button type="button" id="settings-vv-preview" class="btn-cancel" style="white-space:nowrap">▶ Preview</button>
            </div>
          </div>
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

// Returns the current VoiceVox settings from localStorage.
export function getVoicevoxSettings() {
  return {
    speaker: localStorage.getItem('vv-speaker') !== null
      ? parseInt(localStorage.getItem('vv-speaker'), 10)
      : (getDefaultVoicevoxSpeakerId() ?? 1),
    speedScale: parseFloat(localStorage.getItem('vv-speed') ?? '1.0'),
    intonationScale: parseFloat(localStorage.getItem('vv-intonation') ?? '1.5'),
  };
}

// Gender map keyed by VoiceVox speaker_uuid. 'F' = female, 'M' = male, absent = unknown.
const VOICEVOX_GENDER = {
  '7ffcb7ce-00ec-4bdc-82cd-45a8889e43ff': 'F', // 四国めたん
  '388f246b-8c41-4ac1-8e2d-5d79f3ff56d9': 'F', // ずんだもん
  '35b2c544-660e-401e-b503-0e14c635303a': 'F', // 春日部つむぎ
  '3474ee95-c274-47f9-aa1a-8322163d96f1': 'F', // 雨晴はう
  'b1a81618-b27b-40d2-b0ea-27a9ad408c4b': 'F', // 波音リツ
  'c30dc15a-0992-4f8d-8bb8-ad3b314e6a6f': 'M', // 玄野武宏
  '4f51116a-d9ee-4516-925d-21f183e2afad': 'M', // 青山龍星
  '8eaad775-3119-417e-8cf4-2a10bfd592c8': 'F', // 冥鳴ひまり
  '481fb609-6446-4870-9f46-90c4dd623403': 'F', // 九州そら
  '9f3ee141-26ad-437e-97bd-d22298d02ad2': 'F', // もち子さん
  '67d5d8da-acd7-4207-bb10-b5542d3a663b': 'F', // WhiteCUL
  '044830d2-f23b-44d6-ac0d-b5d733caa900': 'M', // No.7
  '468b8e94-9da4-4f7a-8715-a22a48844f9e': 'M', // ちび式じい
  '0693554c-338e-4790-8982-b9c6d476dc69': 'F', // 櫻歌ミコ
  'a8cc6d22-aad0-4ab8-bf1e-2f843924164a': 'F', // 小夜/SAYO
  '882a636f-3bac-431a-966d-c5e6bba9f949': 'F', // ナースロボ＿タイプＴ
  '471e39d2-fb11-4c8c-8d89-4b322d2498e0': 'M', // †聖騎士 紅桜†
  '0acebdee-a4a5-4e12-a695-e19609728e30': 'M', // 雀松朱司
  '7d1e7ba7-f957-40e5-a3fc-da49f769ab65': 'M', // 麒ヶ島宗麟
  'ba5d2428-f7e0-4c20-ac41-9dd56e9178b4': 'F', // 春歌ナナ
  '00a5c10c-d3bd-459f-83fd-43180b521a44': 'F', // 猫使アル
  'c20a2254-0349-4470-9fc8-e5c0f8cf3404': 'M', // 猫使ビィ
  '1f18ffc3-47ea-4ce0-9829-0576d03a7ec8': 'F', // 中国うさぎ
  '04dbd989-32d0-40b4-9e71-17c920f2a8a9': 'F', // 栗田まろん
  'dda44ade-5f9c-4a3a-9d2c-2a976c7476d9': 'F', // あいえるたん
  '287aa49f-e56b-4530-a469-855776c84a8d': 'F', // 満別花丸
  '97a4af4b-086e-4efd-b125-7ae2da85e697': 'F', // 琴詠ニア
  '0156da66-4300-474a-a398-49eb2e8dd853': 'F', // ぞん子
  '4614a7de-9829-465d-9791-97eb8a5f9b86': 'M', // 中部つるぎ
  '3b91e034-e028-4acb-a08d-fbdcd207ea63': 'M', // 離途
  '0b466290-f9b6-4718-8d37-6c0c81e824ac': 'F', // 黒沢冴白
  '462cd6b4-c088-42b0-b357-3816e24f112e': 'F', // ユーレイちゃん
  '80802b2d-8c75-4429-978b-515105017010': 'F', // 東北ずん子
  '1bd6b32b-d650-4072-bbe5-1d0ef4aaa28b': 'F', // 東北きりたん
  'ab4c31a3-8769-422a-b412-708f5ae637e8': 'F', // 東北イタコ
  '3be49e15-34bb-48a0-9e2f-9b80c96e9905': 'F', // あんこもん
};

const VOICEVOX_DEFAULT_SPEAKER_UUID = '4f51116a-d9ee-4516-925d-21f183e2afad'; // 青山龍星

let _voicevoxSpeakers = null; // cached speaker list; null = not yet fetched, [] = unavailable

// Fetches the speaker list once and caches it. Returns true if VoiceVox is available.
export async function checkVoicevoxAvailable() {
  if (_voicevoxSpeakers !== null) return _voicevoxSpeakers.length > 0;
  try {
    const resp = await fetch('/api/voicevox/speakers');
    _voicevoxSpeakers = await resp.json();
  } catch (_) {
    _voicevoxSpeakers = [];
  }
  return _voicevoxSpeakers.length > 0;
}

// Returns the ノーマル style ID for the default speaker (青山龍星) from the cached list,
// or null if the list isn't loaded or the speaker isn't found.
function getDefaultVoicevoxSpeakerId() {
  if (!_voicevoxSpeakers?.length) return null;
  for (const sp of _voicevoxSpeakers) {
    if (sp.speaker_uuid === VOICEVOX_DEFAULT_SPEAKER_UUID) {
      const normal = sp.styles.find(s => s.name === 'ノーマル');
      if (normal) return normal.id;
    }
  }
  return null;
}

let _ffmpegAvailableCache = null;
// Checks once whether ffmpeg is available on the server. Returns a boolean.
export async function checkFfmpegAvailable() {
  if (_ffmpegAvailableCache !== null) return _ffmpegAvailableCache;
  try {
    const resp = await fetch('/api/ffmpeg/available');
    const data = await resp.json();
    _ffmpegAvailableCache = data.available === true;
  } catch (_) {
    _ffmpegAvailableCache = false;
  }
  return _ffmpegAvailableCache;
}

async function populateVoicevoxSpeakers() {
  const sel = document.getElementById('settings-vv-speaker');
  if (!sel) return;
  await checkVoicevoxAvailable();
  const available = _voicevoxSpeakers.length > 0;
  document.getElementById('settings-vv-preview')?.toggleAttribute('disabled', !available);
  document.getElementById('settings-vv-gen-all')?.toggleAttribute('disabled', !available);
  if (!available) {
    sel.innerHTML = '<option value="1">VoiceVox unavailable</option>';
    return;
  }
  const savedRaw = localStorage.getItem('vv-speaker');

  // Keep only speakers that have a ノーマル style, grouped by gender.
  const groups = { F: [], M: [], '': [] };
  for (const sp of _voicevoxSpeakers) {
    const normal = sp.styles.find(s => s.name === 'ノーマル');
    if (!normal) continue;
    const gender = VOICEVOX_GENDER[sp.speaker_uuid] ?? '';
    groups[gender].push({ id: normal.id, name: sp.name });
  }

  const selectedId = savedRaw !== null
    ? parseInt(savedRaw, 10)
    : (getDefaultVoicevoxSpeakerId() ?? groups.M[0]?.id ?? groups.F[0]?.id ?? 1);

  const makeOptions = list =>
    list.map(({ id, name }) =>
      `<option value="${id}"${id === selectedId ? ' selected' : ''}>${name}</option>`
    ).join('');

  sel.innerHTML =
    (groups.F.length ? `<optgroup label="Female">${makeOptions(groups.F)}</optgroup>` : '') +
    (groups.M.length ? `<optgroup label="Male">${makeOptions(groups.M)}</optgroup>` : '') +
    (groups[''].length ? `<optgroup label="Other">${makeOptions(groups[''])}</optgroup>` : '');
}

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
export function playWordAudio(word, rate = 1) {
  speechSynthesis.cancel();
  if (_currentAudio) { _currentAudio.pause(); }
  const audio = new Audio(`/static/audio/${encodeURIComponent(word.word)}.ogg`);
  audio.playbackRate = rate;
  audio.addEventListener('error', () => playTts(word.word, 'ja-JP', WORD_TTS_RATE * rate));
  _currentAudio = audio;
  audio.play().catch(() => {});
}

// playSentenceAudio plays a word's generated sentence audio file if available, else falls back to TTS.
export function playSentenceAudio(word, rate = 1) {
  speechSynthesis.cancel();
  if (_currentAudio) { _currentAudio.pause(); }
  const audio = new Audio(`/static/audio/${encodeURIComponent(word.word)}_sentence.ogg`);
  audio.playbackRate = rate;
  audio.addEventListener('error', () => { if (word.exampleJp) playTts(word.exampleJp, 'ja-JP', 0.75 * rate); });
  _currentAudio = audio;
  audio.play().catch(() => {});
}

function updateGenAllBtn() {
  const genBtn = document.getElementById('settings-vv-gen-all');
  const cancelBtn = document.getElementById('settings-vv-gen-cancel');
  if (!genBtn || !cancelBtn) return;
  const generating = !!_vvGenController;
  genBtn.classList.toggle('hidden', generating);
  cancelBtn.classList.toggle('hidden', !generating);
  if (generating) {
    document.getElementById('settings-vv-gen-cancel-label').textContent =
      'Cancel generation (' + _vvGenPending + ' remaining)';
  }
}

async function runGenerateAllAudio() {
  if (_vvGenController) return;
  const words = await fetch('/api/words').then(r => r.json());
  if (!words.length) return;
  _vvGenController = new AbortController();
  _vvGenTotal = words.length;
  _vvGenPending = words.length;
  updateGenAllBtn();
  for (const w of words) {
    if (_vvGenController.signal.aborted) break;
    try {
      const vv = getVoicevoxSettings();
      await fetch('/api/words/' + w.id + '/generate-audio', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ word: w.word, speaker: vv.speaker, speedScale: vv.speedScale, intonationScale: vv.intonationScale }),
        signal: _vvGenController.signal,
      });
    } catch (_) {
      if (_vvGenController?.signal.aborted) break;
    }
    _vvGenPending = Math.max(0, _vvGenPending - 1);
    updateGenAllBtn();
  }
  _vvGenController = null;
  _vvGenPending = 0;
  _vvGenTotal = 0;
  updateGenAllBtn();
}

function initializeSettings() {
  const settingsBtn = document.getElementById('settings-btn');
  const settingsModal = document.getElementById('settings-modal-backdrop');
  if (!settingsBtn || !settingsModal) return;

  const saveBtn = document.getElementById('settings-save-btn');
  const closeModal = () => {
    if (_vvGenController) return;
    settingsModal.classList.add('hidden');
  };

  const setDirty = () => saveBtn?.classList.add('btn-save--dirty');
  const clearDirty = () => saveBtn?.classList.remove('btn-save--dirty');

  // Open: fetch current settings, populate, and reset dirty state
  settingsBtn.addEventListener('click', async () => {
    const resp = await fetch('/api/settings/drill');
    const settings = await resp.json();

    const totalInput = document.getElementById('settings-total-words');
    const roundInput = document.getElementById('settings-round-size');
    const newWordTargetInput = document.getElementById('settings-new-word-target');
    if (totalInput) totalInput.value = settings.maxWords;
    if (roundInput) roundInput.value = settings.roundSize;
    if (newWordTargetInput) newWordTargetInput.value = settings.newWordTarget;

    settingsModal.querySelectorAll('.filter-chip[data-setting-filter]').forEach(btn => {
      btn.classList.toggle('active', settings.wordTypes.includes(btn.dataset.settingFilter));
    });

    populateTtsSelects();
    const autoplayEl = document.getElementById('settings-tts-autoplay');
    if (autoplayEl) autoplayEl.checked = localStorage.getItem('tts-autoplay') !== 'off';

    await populateVoicevoxSpeakers();
    const vvSpeed = document.getElementById('settings-vv-speed');
    const vvIntonation = document.getElementById('settings-vv-intonation');
    if (vvSpeed) {
      vvSpeed.value = localStorage.getItem('vv-speed') ?? '1.0';
      document.getElementById('settings-vv-speed-val').textContent = parseFloat(vvSpeed.value).toFixed(2);
    }
    if (vvIntonation) {
      vvIntonation.value = localStorage.getItem('vv-intonation') ?? '1.5';
      document.getElementById('settings-vv-intonation-val').textContent = parseFloat(vvIntonation.value).toFixed(2);
    }

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
    const newWordTargetVal = parseInt(document.getElementById('settings-new-word-target')?.value, 10);
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
        newWordTarget: isNaN(newWordTargetVal) ? 8 : Math.max(1, Math.min(999, newWordTargetVal)),
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

    const vvSpeakerEl = document.getElementById('settings-vv-speaker');
    const vvSpeedEl = document.getElementById('settings-vv-speed');
    const vvIntonationEl = document.getElementById('settings-vv-intonation');
    if (vvSpeakerEl) localStorage.setItem('vv-speaker', vvSpeakerEl.value);
    if (vvSpeedEl) localStorage.setItem('vv-speed', vvSpeedEl.value);
    if (vvIntonationEl) localStorage.setItem('vv-intonation', vvIntonationEl.value);

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

  const newWordTargetInput = document.getElementById('settings-new-word-target');
  if (newWordTargetInput) {
    attachNumberStepper(newWordTargetInput, {
      onChange: setDirty,
      onInput: setDirty,
      step: 1,
      min: 1,
      max: 999,
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

  document.getElementById('settings-vv-speed')?.addEventListener('input', e => {
    document.getElementById('settings-vv-speed-val').textContent = parseFloat(e.target.value).toFixed(2);
    setDirty();
  });
  document.getElementById('settings-vv-intonation')?.addEventListener('input', e => {
    document.getElementById('settings-vv-intonation-val').textContent = parseFloat(e.target.value).toFixed(2);
    setDirty();
  });
  document.getElementById('settings-vv-speaker')?.addEventListener('change', setDirty);

  document.getElementById('settings-vv-gen-all')?.addEventListener('click', runGenerateAllAudio);
  document.getElementById('settings-vv-gen-cancel')?.addEventListener('click', () => _vvGenController?.abort());

  document.getElementById('settings-vv-preview')?.addEventListener('click', async () => {
    const btn = document.getElementById('settings-vv-preview');
    const speaker = parseInt(document.getElementById('settings-vv-speaker')?.value ?? '1', 10);
    const speedScale = parseFloat(document.getElementById('settings-vv-speed')?.value ?? '1.0');
    const intonationScale = parseFloat(document.getElementById('settings-vv-intonation')?.value ?? '1.5');
    btn.disabled = true;
    try {
      const resp = await fetch('/api/voicevox/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ speaker, speedScale, intonationScale }),
      });
      if (!resp.ok) throw new Error(await resp.text());
      const blob = await resp.blob();
      const audioUrl = URL.createObjectURL(blob);
      if (_currentAudio) { _currentAudio.pause(); }
      _currentAudio = new Audio(audioUrl);
      _currentAudio.addEventListener('ended', () => URL.revokeObjectURL(audioUrl));
      _currentAudio.play();
    } finally {
      btn.disabled = false;
    }
  });
}

injectSettingsModal();
initializeSettings();

// ── Shared data-tooltip hover system (all pages) ──────────────────────────────
const _hoverTooltip = document.createElement('div');
_hoverTooltip.className = 'lex-tooltip';
document.body.appendChild(_hoverTooltip);

let _activeTooltipEl = null;
let _tooltipExtraClass = null;

document.addEventListener('mouseover', e => {
  const el = e.target.closest('[data-tooltip], [data-tooltip-html]');
  if (_tooltipExtraClass) {
    _hoverTooltip.classList.remove(_tooltipExtraClass);
    _tooltipExtraClass = null;
  }
  _activeTooltipEl = el ?? null;
  if (!el) { _hoverTooltip.classList.remove('visible'); return; }
  if (el.dataset.tooltipHtml !== undefined) {
    _hoverTooltip.innerHTML = el.dataset.tooltipHtml;
  } else {
    _hoverTooltip.textContent = el.dataset.tooltip;
  }
  if (el.dataset.tooltipClass) {
    _tooltipExtraClass = el.dataset.tooltipClass;
    _hoverTooltip.classList.add(_tooltipExtraClass);
  }
  _hoverTooltip.style.left = (e.clientX + 14) + 'px';
  _hoverTooltip.style.top = (e.clientY + 18) + 'px';
  _hoverTooltip.classList.add('visible');
});
document.addEventListener('mousemove', e => {
  if (!_hoverTooltip.classList.contains('visible')) return;
  const pad = 8;
  const w = _hoverTooltip.offsetWidth;
  const h = _hoverTooltip.offsetHeight;
  let left = e.clientX + 14;
  if (left + w > window.innerWidth - pad) left = window.innerWidth - w - pad;
  let top = e.clientY + 18;
  top = Math.max(pad, Math.min(top, window.innerHeight - h - pad));
  _hoverTooltip.style.left = left + 'px';
  _hoverTooltip.style.top = top + 'px';
});

export function refreshTooltip(el) {
  if (_activeTooltipEl !== el) return;
  if (el.dataset.tooltipHtml !== undefined) {
    _hoverTooltip.innerHTML = el.dataset.tooltipHtml;
  } else {
    _hoverTooltip.textContent = el.dataset.tooltip;
  }
}

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

export const PROVIDER_MODELS = [
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


export function playDing() {
  try {
    const ctx = new AudioContext();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.frequency.setValueAtTime(880, ctx.currentTime);
    osc.frequency.exponentialRampToValueAtTime(440, ctx.currentTime + 0.3);
    gain.gain.setValueAtTime(0.4, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.6);
    osc.start(ctx.currentTime);
    osc.stop(ctx.currentTime + 0.6);
  } catch (_) {}
}
