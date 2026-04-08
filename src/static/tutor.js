import { PROVIDER_MODELS, playTts, WORD_TTS_RATE, checkVoicevoxAvailable, getVoicevoxSettings } from './common.js';

// ── VoiceVox / TTS playback ────────────────────────────────────────────────

let _tutorAudio = null;

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
      if (_tutorAudio) { _tutorAudio.pause(); URL.revokeObjectURL(_tutorAudio.src); }
      speechSynthesis.cancel();
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
const FREE_TOPICS = [
  { jp: '食べ物',         en: 'food'            },
  { jp: '天気',           en: 'the weather'     },
  { jp: '家族',           en: 'family'          },
  { jp: '趣味',           en: 'hobbies'         },
  { jp: '旅行',           en: 'travel'          },
  { jp: '学校',           en: 'school'          },
  { jp: '仕事',           en: 'work'            },
  { jp: '音楽',           en: 'music'           },
  { jp: '映画',           en: 'movies'          },
  { jp: 'スポーツ',       en: 'sports'          },
  { jp: '動物',           en: 'animals'         },
  { jp: '季節',           en: 'the seasons'     },
  { jp: '週末の予定',     en: 'weekend plans'   },
  { jp: '好きな食べ物',   en: 'favorite foods'  },
  { jp: '町',             en: 'your town'       },
  { jp: '買い物',         en: 'shopping'        },
  { jp: '健康',           en: 'health'          },
  { jp: '色',             en: 'colors'          },
];

function randomFreeGreeting(note) {
  const t = FREE_TOPICS[Math.floor(Math.random() * FREE_TOPICS.length)];
  const obj = {
    jp: `こんにちは！今日は${t.jp}について話しましょう！`,
    en: `Hello! Let's talk about ${t.en} today!`,
  };
  if (note) obj.note = note;
  return JSON.stringify(obj);
}

// Greetings are stored as JSON objects (same format as AI responses) so they
// go through the same rendering pipeline and appear in history consistently.
// greeting may be a string or a zero-arg function returning a string.
const TUTOR_MODES = [
  {
    value:    'free',
    label:    'Conversation (Speaking)',
    greeting: () => randomFreeGreeting(null),
  },
  {
    value:    'free-en',
    label:    'Conversation (Comprehension)',
    greeting: () => randomFreeGreeting('Read the Japanese above and reply in English to show you understood.'),
  },
  {
    value:    'grammar',
    label:    'Grammar Tutor',
    greeting: JSON.stringify({ note: "Welcome to Grammar Tutor mode! Write anything in Japanese and I'll analyze it for grammar errors, explain each mistake, and show you the correct form." }),
  },
  {
    value:    'vocab',
    label:    'Vocabulary Quiz',
    greeting: JSON.stringify({ jp: '「食べる」とは英語で何ですか？', en: 'What does 「食べる」 mean in English?', note: "Welcome to Vocabulary Quiz mode! I'll test you one word at a time." }),
  },
  {
    value:    'translation-en-jp',
    label:    'Translation: English → Japanese',
    greeting: JSON.stringify({ note: "Welcome to Translation Practice! I'll give you English sentences to translate into Japanese.", question: 'First sentence: "I drink water every day."' }),
  },
  {
    value:    'translation-jp-en',
    label:    'Translation: Japanese → English',
    greeting: JSON.stringify({ note: "Welcome to Translation Practice! I'll give you Japanese sentences to translate into English.", jp: '毎日、水を飲みます。', question: 'How would you translate this sentence into English?' }),
  },
  {
    value:    'reading',
    label:    'Reading Practice',
    greeting: JSON.stringify({ jp: '今日は晴れです。空は青くてきれいです。', en: 'It is sunny today. The sky is blue and beautiful.', note: 'Welcome to Reading Practice! Here is your first passage:', question: 'What is the weather like today?' }),
  },
];

const els = {
  modeSelect:   document.getElementById('tutor-mode-select'),
  modelSelect:  document.getElementById('tutor-model-select'),
  providerInfo: document.getElementById('tutor-provider-info'),
  btnNewChat:   document.getElementById('btn-new-chat'),
  btnDebug:     document.getElementById('btn-debug-toggle'),
  messages:     document.getElementById('tutor-messages'),
  form:         document.getElementById('tutor-form'),
  input:        document.getElementById('tutor-input'),
  btnSend:      document.getElementById('btn-tutor-send'),
};

const state = {
  providers:    null,
  history:      [],   // { role: 'user'|'assistant', content: string }[]
  sending:      false,
  debugMode:    false,
  systemPrompt: null, // cached for the current mode
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
const KNOWN_RESP_FIELDS = new Set(['jp', 'en', 'note', 'correction']);

// Build the inner HTML for an assistant message bubble from a response object.
// Renders in a fixed semantic order: jp/en → note → other fields.
// Correction is rendered separately outside the bubble (see appendMessage).
function responseToHTML(resp) {
  const parts = [];

  if (resp.jp && resp.en) {
    parts.push('<div class="tutor-seg tutor-seg--jp" data-tooltip="' + escHtml(resp.en) + '">' + escHtml(resp.jp) + '</div>');
  } else if (resp.jp) {
    parts.push('<div class="tutor-seg">' + escHtml(resp.jp) + '</div>');
  } else if (resp.en) {
    parts.push('<div class="tutor-seg tutor-seg--en">' + escHtml(resp.en) + '</div>');
  }

  if (resp.note) {
    parts.push('<div class="tutor-seg tutor-seg--note">' + escHtml(resp.note) + '</div>');
  }

  // Any other fields: generic labeled display.
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

function currentMode() {
  return TUTOR_MODES.find(m => m.value === els.modeSelect.value) || TUTOR_MODES[0];
}

function startNewChat() {
  state.history = [];
  state.systemPrompt = null; // invalidate cache; mode may have changed
  const mode = currentMode();
  const greeting = typeof mode.greeting === 'function' ? mode.greeting() : mode.greeting;
  state.history.push({ role: 'assistant', content: greeting });
  renderAllMessages();
  // Tell the server to forget the old session so navigation restores this fresh chat.
  fetch('/api/tutor/session', { method: 'DELETE' });
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

async function init() {
  // Populate mode select
  els.modeSelect.innerHTML = TUTOR_MODES.map(m =>
    '<option value="' + m.value + '">' + m.label + '</option>'
  ).join('');

  // Fetch providers, saved session, and VoiceVox availability in parallel
  const [providersResp, sessionResp] = await Promise.allSettled([
    fetch('/api/providers').then(r => r.json()),
    fetch('/api/tutor/session').then(r => r.json()),
  ]);
  checkVoicevoxAvailable(); // warm cache; no need to await

  state.providers = providersResp.status === 'fulfilled' ? (providersResp.value.ai || {}) : {};
  const session   = sessionResp.status === 'fulfilled'   ? sessionResp.value : {};

  populateModelSelect();

  // Restore saved session, or start a fresh greeting
  restoreSession(session);

  els.modeSelect.addEventListener('change', startNewChat);
  els.btnNewChat.addEventListener('click', startNewChat);

  els.btnDebug.addEventListener('click', () => {
    state.debugMode = !state.debugMode;
    els.btnDebug.classList.toggle('btn-header--active', state.debugMode);
    renderAllMessages();
  });

  els.form.addEventListener('submit', e => {
    e.preventDefault();
    const text = els.input.value.trim();
    if (!text || state.sending) return;
    els.input.value = '';
    autoResize(els.input);
    sendMessage(text);
  });

  els.input.addEventListener('keydown', e => {
    if (e.key === 'Enter' && e.shiftKey) {
      e.preventDefault();
      els.form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    }
  });

  els.input.addEventListener('input', () => autoResize(els.input));

  els.input.focus();
}

init();
