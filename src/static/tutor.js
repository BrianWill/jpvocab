import { PROVIDER_MODELS, playTts, WORD_TTS_RATE, checkVoicevoxAvailable, getVoicevoxSettings } from './common.js';

// ── VoiceVox / TTS playback ────────────────────────────────────────────────

let _tutorAudio = null;

// Stops any currently playing tutor audio (VoiceVox or Web Speech).
function stopAudio() {
  if (_tutorAudio) {
    _tutorAudio.pause();
    URL.revokeObjectURL(_tutorAudio.src);
    _tutorAudio = null;
  }
  speechSynthesis.cancel();
}

// Plays Japanese text via VoiceVox if available, otherwise falls back to Web Speech TTS.
async function playJp(text) {
  const vvAvailable = await checkVoicevoxAvailable();
  if (vvAvailable) {
    const vv = getVoicevoxSettings();
    try {
      const resp = await fetch('/api/voicevox/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text, speaker: vv.speaker, speedScale: vv.speedScale, intonationScale: vv.intonationScale }),
      });
      if (!resp.ok) throw new Error('voicevox unavailable');
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      stopAudio();
      _tutorAudio = new Audio(url);
      _tutorAudio.addEventListener('ended', () => { URL.revokeObjectURL(url); _tutorAudio = null; });
      _tutorAudio.play();
      return;
    } catch (_) { /* fall through to Web Speech */ }
  }
  playTts(text, 'ja-JP', WORD_TTS_RATE);
}

// Topics suitable for N5–N3 learners. Each entry has a Japanese topic phrase and
// an English label used to build the opening greeting.

const els = {
  modeSelect:       document.getElementById('tutor-mode-select'),
  modelSelect:      document.getElementById('tutor-model-select'),
  providerInfo:     document.getElementById('tutor-provider-info'),
  btnNewChat:       document.getElementById('btn-new-chat'),
  btnDebug:         document.getElementById('btn-debug-toggle'),
  btnAddPrompt:     document.getElementById('btn-add-prompt'),
  btnDeletePrompt:  document.getElementById('btn-delete-prompt'),
  messages:         document.getElementById('tutor-messages'),
  form:             document.getElementById('tutor-form'),
  input:            document.getElementById('tutor-input'),
  btnMic:           document.getElementById('btn-tutor-mic'),
  btnSend:          document.getElementById('btn-tutor-send'),
  promptModal:      document.getElementById('prompt-modal'),
  promptForm:       document.getElementById('prompt-form'),
  promptLabelInput: document.getElementById('prompt-label-input'),
  promptSystemInput:document.getElementById('prompt-system-input'),
  promptGreetInput: document.getElementById('prompt-greeting-input'),
  promptLangInput:  document.getElementById('prompt-lang-input'),
  promptModalError: document.getElementById('prompt-modal-error'),
  btnCancelPrompt:  document.getElementById('btn-cancel-prompt'),
  btnSavePrompt:    document.getElementById('btn-save-prompt'),
};

const state = {
  providers:       null,
  prompts:         [],     // tutorPromptJSON[] loaded from /api/tutor/prompts
  history:         [],     // { role: 'user'|'assistant', content: string }[]
  sending:         false,
  debugMode:       false,
  systemPrompt:    null,   // cached for the current mode
  listening:       false,
  waitingForStart: false,  // true after startNewChat; cleared once the bot sends its first real message
  pendingGreeting: null,   // greeting string shown while waitingForStart, never sent to AI
};

// ── Provider / model select ────────────────────────────────────────────────

function populateModelSelect() {
  const providers = state.providers || {};
  const hasProviders = PROVIDER_MODELS.some(p => providers[p.key]);
  const missingLines = PROVIDER_MODELS
    .filter(p => !providers[p.key])
    .map(p => p.label + ': set ' + p.envKey + ' to enable');
  const tip = missingLines.length ? missingLines.join('\n') + '\n— then restart the program' : null;

  let firstAvailSet = false;
  const optgroupsHtml = PROVIDER_MODELS.map(({ key, label, models }) => {
    const avail = providers[key];
    const groupLabel = avail ? label : label + ' — no API key';
    const options = models.map(([val, text]) => {
      const sel = avail && !firstAvailSet ? ' selected' : '';
      if (sel) firstAvailSet = true;
      return '<option value="' + val + '"' + sel + '>' + text + '</option>';
    }).join('');
    return '<optgroup label="' + groupLabel + '"' + (avail ? '' : ' disabled') + '>' + options + '</optgroup>';
  }).join('');

  els.modelSelect.innerHTML =
    (!hasProviders ? '<option value="">no API keys configured</option>' : '') +
    optgroupsHtml;
  els.modelSelect.disabled = !hasProviders;

  if (tip) {
    els.providerInfo.dataset.tooltip = tip;
    els.providerInfo.style.display = '';
  } else {
    els.providerInfo.style.display = 'none';
  }

  setSendDisabled(!hasProviders || state.sending);
}

// ── Segment rendering ──────────────────────────────────────────────────────

function escHtml(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// Parse the AI's JSON segment array from a content string.
// Falls back to [{en: content}] if parsing fails.
function parseSegments(content) {
  try {
    const parsed = JSON.parse(content);
    if (Array.isArray(parsed) && parsed.length > 0) return parsed;
  } catch (_) {}
  return [{ en: content }];
}

// Parse the AI's single JSON object from a content string.
// Falls back to {en: content} if parsing fails.
function parseResponse(content) {
  try {
    const parsed = JSON.parse(content);
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return parsed;
  } catch (_) {}
  return { en: content };
}

// Fields with dedicated rendering; anything else falls through to generic labeled display.
const KNOWN_RESP_FIELDS = new Set(['jp', 'en', 'note', 'correction', 'question']);

// Build the inner HTML for an assistant message bubble from a response object.
// Renders in a fixed semantic order:
//   1. jp (with en as hover tooltip if present; en is never a standalone segment)
//   2. question
//   3. all other unknown fields (generic labeled)
//   4. note (always last)
// Correction is rendered separately outside the bubble (see appendMessage).
function responseToHTML(resp) {
  const parts = [];

  // 1. jp first; en is tooltip-only
  if (resp.jp) {
    if (resp.en) {
      parts.push('<div class="tutor-seg tutor-seg--jp" data-tooltip="' + escHtml(resp.en) + '">' + escHtml(resp.jp) + '</div>');
    } else {
      parts.push('<div class="tutor-seg">' + escHtml(resp.jp) + '</div>');
    }
  } else if (resp.en) {
    // en only (no jp to attach tooltip to — display standalone)
    parts.push('<div class="tutor-seg tutor-seg--en">' + escHtml(resp.en) + '</div>');
  }

  // 2. question immediately after jp
  if (resp.question) {
    parts.push('<div class="tutor-seg">' + escHtml(resp.question) + '</div>');
  }

  // 3. any other unknown fields: generic labeled display
  for (const [key, val] of Object.entries(resp)) {
    if (!KNOWN_RESP_FIELDS.has(key) && val) {
      parts.push(
        '<div class="tutor-seg tutor-seg--extra">' +
          '<span class="tutor-seg-key">' + escHtml(key) + '</span>' +
          escHtml(val) +
        '</div>'
      );
    }
  }

  // 4. note last
  if (resp.note) {
    parts.push('<div class="tutor-seg tutor-seg--note">' + escHtml(resp.note) + '</div>');
  }

  return parts.join('');
}

// ── Normal chat rendering ──────────────────────────────────────────────────

function appendMessage(role, content) {
  const row = document.createElement('div');
  row.className = 'tutor-message tutor-message--' + role;
  const bubble = document.createElement('div');
  bubble.className = 'tutor-bubble';
  if (role === 'assistant') {
    const resp = parseResponse(content);
    bubble.innerHTML = responseToHTML(resp);
    if (resp.jp) {
      const btn = document.createElement('button');
      btn.className = 'tutor-play-btn';
      btn.setAttribute('aria-label', 'Play Japanese');
      btn.textContent = '▶';
      btn.addEventListener('click', () => playJp(resp.jp));
      row.appendChild(btn);
      playJp(resp.jp);
    }
    if (resp.correction) {
      // Fade the preceding user message
      const userRows = els.messages.querySelectorAll('.tutor-message--user');
      if (userRows.length > 0) userRows[userRows.length - 1].classList.add('tutor-message--faded');
      // Insert correction row (right-aligned, styled differently from user bubble)
      const corrRow = document.createElement('div');
      corrRow.className = 'tutor-message tutor-message--user';
      const corrBubble = document.createElement('div');
      corrBubble.className = 'tutor-bubble tutor-bubble--correction';
      corrBubble.textContent = resp.correction;
      corrRow.appendChild(corrBubble);
      els.messages.appendChild(corrRow);
    }
  } else {
    bubble.textContent = content;
  }
  row.appendChild(bubble);
  els.messages.appendChild(row);
  els.messages.scrollTop = els.messages.scrollHeight;
  return row;
}

function appendLoadingDots() {
  const row = document.createElement('div');
  row.className = 'tutor-message tutor-message--assistant';
  row.innerHTML = '<div class="tutor-bubble tutor-bubble--loading">' +
    '<span class="tutor-dots"><span>.</span><span>.</span><span>.</span></span></div>';
  els.messages.appendChild(row);
  els.messages.scrollTop = els.messages.scrollHeight;
  return row;
}

// ── Debug rendering ────────────────────────────────────────────────────────

// Returns syntax-highlighted HTML for a JSON string, or null if not valid JSON.
function highlightJson(raw) {
  let src;
  try { src = JSON.stringify(JSON.parse(raw), null, 2); } catch (_) { return null; }
  return src.replace(
    /("(?:\\.|[^"\\])*")(:\s*)?|(true|false|null)|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|([\{\}\[\],:])/g,
    (match, str, colon, kw, num, punct) => {
      if (str !== undefined) {
        const cls = colon !== undefined ? 'json-key' : 'json-str';
        return '<span class="' + cls + '">' + escHtml(str) + '</span>' + (colon ? colon : '');
      }
      if (kw    !== undefined) return '<span class="json-kw">'    + kw    + '</span>';
      if (num   !== undefined) return '<span class="json-num">'   + num   + '</span>';
      if (punct !== undefined) return '<span class="json-punct">' + punct + '</span>';
      return escHtml(match);
    }
  );
}

function appendDebugBlock(role, content) {
  const block = document.createElement('div');
  block.className = 'tutor-debug-block tutor-debug-block--' + role;
  const label = document.createElement('div');
  label.className = 'tutor-debug-label';
  label.textContent = '[' + role.toUpperCase() + ']';
  const pre = document.createElement('pre');
  pre.className = 'tutor-debug-content';
  const highlighted = highlightJson(content);
  if (highlighted !== null) {
    pre.innerHTML = highlighted;
  } else {
    pre.textContent = content;
  }
  block.appendChild(label);
  block.appendChild(pre);
  els.messages.appendChild(block);
  els.messages.scrollTop = els.messages.scrollHeight;
  return block;
}

async function renderAllMessages() {
  els.messages.innerHTML = '';

  if (state.waitingForStart) {
    if (state.pendingGreeting) appendMessage('assistant', state.pendingGreeting);
    els.input.placeholder = 'Press Shift+Enter to begin';
    return;
  }
  els.input.placeholder = 'Type a message…';

  if (state.debugMode) {
    if (!state.systemPrompt) {
      try {
        const r = await fetch('/api/tutor/system-prompt?mode=' + encodeURIComponent(els.modeSelect.value));
        state.systemPrompt = r.ok ? await r.text() : '(failed to load)';
      } catch (_) { state.systemPrompt = '(failed to load)'; }
    }
    appendDebugBlock('system', state.systemPrompt);
    for (const msg of state.history) appendDebugBlock(msg.role, msg.content);
  } else {
    for (const msg of state.history) appendMessage(msg.role, msg.content);
  }
}

// ── Chat state ─────────────────────────────────────────────────────────────

function currentPrompt() {
  const id = parseInt(els.modeSelect.value, 10);
  return state.prompts.find(p => p.id === id) || state.prompts[0] || null;
}

function updateDeleteBtnVisibility() {
  const p = currentPrompt();
  els.btnDeletePrompt.style.display = (p && p.can_remove) ? '' : 'none';
}

function startNewChat() {
  stopAudio();
  state.history = [];
  state.systemPrompt = null; // invalidate cache; mode may have changed
  state.waitingForStart = true;
  const prompt = currentPrompt();
  if (!prompt) return;
  state.pendingGreeting = prompt.greeting || '{}';
  renderAllMessages();
  // Tell the server to forget the old session so navigation restores this fresh chat.
  fetch('/api/tutor/session', { method: 'DELETE' });
}

// Sends an empty message list to let the AI generate its own opening turn.
// Called when the user hits Enter on the "waiting for start" screen.
async function kickoffChat() {
  stopAudio();
  state.waitingForStart = false;
  state.pendingGreeting = null;
  els.input.placeholder = 'Type a message…';
  const aiModel = els.modelSelect.value;
  if (!aiModel) return;

  els.messages.innerHTML = '';
  state.sending = true;
  setSendDisabled(true);
  const loadingRow = appendLoadingDots();

  try {
    const resp = await fetch('/api/tutor/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ai_model:   aiModel,
        tutor_mode: els.modeSelect.value,
        messages:   [],
      }),
    });
    if (!resp.ok) throw new Error((await resp.text()).trim() || resp.statusText);
    const data    = await resp.json();
    const response = data.response || { en: '(empty response)' };
    const rawJson  = JSON.stringify(response);
    loadingRow.remove();
    state.history.push({ role: 'assistant', content: rawJson });
    if (state.debugMode) appendDebugBlock('assistant', rawJson);
    else appendMessage('assistant', rawJson);
  } catch (err) {
    loadingRow.remove();
    appendMessage('assistant', 'Error: ' + err.message);
  } finally {
    state.sending = false;
    setSendDisabled(false);
    els.input.focus();
  }
}

function setSendDisabled(disabled) {
  const hasProviders = PROVIDER_MODELS.some(p => (state.providers || {})[p.key]);
  els.btnSend.disabled = disabled || !hasProviders;
  els.input.disabled   = disabled || !hasProviders;
}

async function sendMessage(text) {
  if (state.sending || !text.trim()) return;
  const aiModel = els.modelSelect.value;
  if (!aiModel) return;

  state.history.push({ role: 'user', content: text });
  if (state.debugMode) appendDebugBlock('user', text);
  else appendMessage('user', text);

  state.sending = true;
  setSendDisabled(true);

  const loadingRow = appendLoadingDots();

  try {
    const resp = await fetch('/api/tutor/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ai_model:   aiModel,
        tutor_mode: els.modeSelect.value,
        messages:   state.history,
      }),
    });

    if (!resp.ok) {
      const errText = await resp.text();
      throw new Error(errText.trim() || resp.statusText);
    }

    const data    = await resp.json();
    const response = data.response || { en: '(empty response)' };
    // Re-serialize back to a JSON string so history content stays consistent
    // with the raw AI output the backend stored in tutorSession.
    const rawJson = JSON.stringify(response);

    stopAudio();
    loadingRow.remove();
    state.history.push({ role: 'assistant', content: rawJson });
    if (state.debugMode) appendDebugBlock('assistant', rawJson);
    else appendMessage('assistant', rawJson);
  } catch (err) {
    loadingRow.remove();
    const errContent = 'Error: ' + err.message;
    if (state.debugMode) appendDebugBlock('assistant', errContent);
    else appendMessage('assistant', errContent);
    // Remove the user message from history so it can be retried
    state.history.pop();
  } finally {
    state.sending = false;
    setSendDisabled(false);
    els.input.focus();
    autoResize(els.input);
  }
}

// ── Voice input ────────────────────────────────────────────────────────────

// Returns the STT recognition language for the current prompt's lang_input value.
// "ja" and "mix" use Japanese; "en" uses English.
function sttLangForCurrentPrompt() {
  const lang = currentPrompt()?.lang_input;
  return (lang === 'en') ? 'en-US' : 'ja-JP';
}

function initMic() {
  const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
  if (!SR) { els.btnMic.style.display = 'none'; return; }

  let recognition = null;
  // Tracks text already in the input before listening started, so interim
  // results can be appended cleanly without doubling up committed text.
  let baseText = '';

  function stopListening() {
    recognition?.stop();
  }

  function startListening() {
    baseText = els.input.value;
    recognition = new SR();
    recognition.lang = sttLangForCurrentPrompt();
    recognition.interimResults = true;
    recognition.maxAlternatives = 1;

    recognition.addEventListener('start', () => {
      state.listening = true;
      els.btnMic.classList.add('btn-tutor-mic--active');
    });

    recognition.addEventListener('result', e => {
      let interim = '';
      let final = '';
      for (const result of e.results) {
        if (result.isFinal) final += result[0].transcript;
        else interim += result[0].transcript;
      }
      els.input.value = baseText + final + interim;
      // Commit finalized text into baseText so the next interim doesn't erase it.
      if (final) baseText += final;
      autoResize(els.input);
    });

    recognition.addEventListener('end', () => {
      state.listening = false;
      els.btnMic.classList.remove('btn-tutor-mic--active');
      recognition = null;
    });

    recognition.addEventListener('error', e => {
      if (e.error !== 'aborted') console.warn('Speech recognition error:', e.error);
    });

    recognition.start();
  }

  els.btnMic.addEventListener('click', () => {
    if (state.listening) stopListening();
    else startListening();
  });
}

// ── Input auto-resize ──────────────────────────────────────────────────────

function autoResize(textarea) {
  textarea.style.height = 'auto';
  textarea.style.height = Math.min(textarea.scrollHeight, 160) + 'px';
}

// ── Init ───────────────────────────────────────────────────────────────────

function restoreSession(session) {
  if (!session.messages || session.messages.length === 0) {
    startNewChat();
    return;
  }
  // Restore mode and model selects
  if (session.tutor_mode) els.modeSelect.value = session.tutor_mode;
  if (session.ai_model)   els.modelSelect.value = session.ai_model;

  // Rebuild the chat from saved history
  state.history = session.messages;
  renderAllMessages();
}

function populateModeSelect() {
  const saved = els.modeSelect.value;
  els.modeSelect.innerHTML = state.prompts.map(p =>
    '<option value="' + p.id + '">' + escHtml(p.label) + '</option>'
  ).join('');
  // Restore previously selected value if it still exists
  if (saved && state.prompts.some(p => String(p.id) === saved)) {
    els.modeSelect.value = saved;
  }
  updateDeleteBtnVisibility();
}

async function loadPrompts() {
  try {
    const resp = await fetch('/api/tutor/prompts');
    if (resp.ok) state.prompts = await resp.json();
  } catch (_) {}
  populateModeSelect();
}

function openAddPromptModal() {
  els.promptLabelInput.value = '';
  els.promptSystemInput.value = '';
  els.promptGreetInput.value = '';
  els.promptLangInput.value = 'ja';
  els.promptModalError.style.display = 'none';
  els.btnSavePrompt.disabled = false;
  els.promptModal.showModal();
}

async function saveCustomPrompt() {
  const label = els.promptLabelInput.value.trim();
  const systemPrompt = els.promptSystemInput.value.trim();
  const greeting = els.promptGreetInput.value.trim() || '{}';
  const langInput = els.promptLangInput.value;

  if (!label || !systemPrompt) {
    els.promptModalError.textContent = 'Name and Instructions are required.';
    els.promptModalError.style.display = '';
    return;
  }

  els.btnSavePrompt.disabled = true;
  els.promptModalError.style.display = 'none';

  try {
    const resp = await fetch('/api/tutor/prompts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label, system_prompt: systemPrompt, greeting, lang_input: langInput }),
    });
    if (!resp.ok) {
      const msg = await resp.text();
      throw new Error(msg || resp.statusText);
    }
    const newPrompt = await resp.json();
    els.promptModal.close();
    state.prompts.push(newPrompt);
    populateModeSelect();
    // Switch to the newly created prompt and start a fresh chat
    els.modeSelect.value = String(newPrompt.id);
    updateDeleteBtnVisibility();
    startNewChat();
  } catch (err) {
    els.promptModalError.textContent = 'Error: ' + err.message;
    els.promptModalError.style.display = '';
    els.btnSavePrompt.disabled = false;
  }
}

async function deleteCurrentPrompt() {
  const prompt = currentPrompt();
  if (!prompt || !prompt.can_remove) return;

  // Two-click confirmation: first click arms, second click (within 3s) deletes.
  if (els.btnDeletePrompt.dataset.armed !== 'true') {
    els.btnDeletePrompt.dataset.armed = 'true';
    els.btnDeletePrompt.textContent = 'Remove?';
    setTimeout(() => {
      els.btnDeletePrompt.dataset.armed = '';
      els.btnDeletePrompt.textContent = '✕';
    }, 3000);
    return;
  }

  els.btnDeletePrompt.dataset.armed = '';
  els.btnDeletePrompt.textContent = '✕';

  try {
    const resp = await fetch('/api/tutor/prompts/' + prompt.id, { method: 'DELETE' });
    if (!resp.ok) return;
    state.prompts = state.prompts.filter(p => p.id !== prompt.id);
    populateModeSelect();
    startNewChat();
  } catch (_) {}
}

async function init() {
  // Fetch prompts, providers, saved session, and VoiceVox availability in parallel
  const [promptsResp, providersResp, sessionResp] = await Promise.allSettled([
    fetch('/api/tutor/prompts').then(r => r.json()),
    fetch('/api/providers').then(r => r.json()),
    fetch('/api/tutor/session').then(r => r.json()),
  ]);
  checkVoicevoxAvailable(); // warm cache; no need to await

  state.prompts   = promptsResp.status === 'fulfilled'   ? (promptsResp.value || [])    : [];
  state.providers = providersResp.status === 'fulfilled' ? (providersResp.value.ai || {}) : {};
  const session   = sessionResp.status === 'fulfilled'   ? sessionResp.value : {};

  populateModeSelect();
  populateModelSelect();

  // Restore saved session, or start a fresh greeting
  restoreSession(session);

  els.modeSelect.addEventListener('change', () => { updateDeleteBtnVisibility(); startNewChat(); });
  els.btnNewChat.addEventListener('click', startNewChat);
  els.btnAddPrompt.addEventListener('click', openAddPromptModal);
  els.btnDeletePrompt.addEventListener('click', deleteCurrentPrompt);
  els.btnCancelPrompt.addEventListener('click', () => els.promptModal.close());
  els.btnSavePrompt.addEventListener('click', saveCustomPrompt);

  els.btnDebug.addEventListener('click', () => {
    state.debugMode = !state.debugMode;
    els.btnDebug.classList.toggle('btn-header--active', state.debugMode);
    renderAllMessages();
  });

  els.form.addEventListener('submit', e => {
    e.preventDefault();
    if (state.waitingForStart) { kickoffChat(); return; }
    const text = els.input.value.trim();
    if (!text || state.sending) return;
    els.input.value = '';
    autoResize(els.input);
    sendMessage(text);
  });

  els.input.addEventListener('keydown', e => {
    if (e.key === 'Enter' && !e.shiftKey && state.waitingForStart) {
      e.preventDefault();
      kickoffChat();
      return;
    }
    if (e.key === 'Enter' && e.shiftKey) {
      e.preventDefault();
      els.form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    }
  });

  els.input.addEventListener('input', () => autoResize(els.input));

  initMic();
  els.input.focus();
}

init();
