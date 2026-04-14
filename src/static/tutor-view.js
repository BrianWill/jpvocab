import { escapeHtml } from './html-utils.js';

const KNOWN_RESP_FIELDS = new Set(['jp', 'en', 'note', 'correction', 'question']);

export function parseResponse(content) {
  try {
    const parsed = JSON.parse(content);
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return parsed;
  } catch (_) {}
  return { en: content };
}

export function responseToHTML(resp) {
  const parts = [];

  if (resp.jp) {
    if (resp.en) {
      parts.push('<div class="tutor-seg tutor-seg--jp" data-tooltip="' + escapeHtml(resp.en) + '">' + escapeHtml(resp.jp) + '</div>');
    } else {
      parts.push('<div class="tutor-seg">' + escapeHtml(resp.jp) + '</div>');
    }
  } else if (resp.en) {
    parts.push('<div class="tutor-seg tutor-seg--en">' + escapeHtml(resp.en) + '</div>');
  }

  if (resp.question) {
    parts.push('<div class="tutor-seg">' + escapeHtml(resp.question) + '</div>');
  }

  for (const [key, value] of Object.entries(resp)) {
    if (!KNOWN_RESP_FIELDS.has(key) && value) {
      parts.push(
        '<div class="tutor-seg tutor-seg--extra">' +
          '<span class="tutor-seg-key">' + escapeHtml(key) + '</span>' +
          escapeHtml(value) +
        '</div>'
      );
    }
  }

  if (resp.note) {
    parts.push('<div class="tutor-seg tutor-seg--note">' + escapeHtml(resp.note) + '</div>');
  }

  return parts.join('');
}

function highlightJson(raw) {
  let src;
  try {
    src = JSON.stringify(JSON.parse(raw), null, 2);
  } catch (_) {
    return null;
  }
  return src.replace(
    /("(?:\\.|[^"\\])*")(:\s*)?|(true|false|null)|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|([\{\}\[\],:])/g,
    (match, str, colon, kw, num, punct) => {
      if (str !== undefined) {
        const cls = colon !== undefined ? 'json-key' : 'json-str';
        return '<span class="' + cls + '">' + escapeHtml(str) + '</span>' + (colon ? colon : '');
      }
      if (kw !== undefined) return '<span class="json-kw">' + kw + '</span>';
      if (num !== undefined) return '<span class="json-num">' + num + '</span>';
      if (punct !== undefined) return '<span class="json-punct">' + punct + '</span>';
      return escapeHtml(match);
    }
  );
}

export function autoResize(textarea) {
  textarea.style.height = 'auto';
  textarea.style.height = Math.min(textarea.scrollHeight, 160) + 'px';
}

export function createTutorView({ els, state, playJp }) {
  function appendMessage(role, content, { autoPlay = true } = {}) {
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
        btn.dataset.tooltip = 'Play Japanese (Alt+P replays last)';
        btn.textContent = '▶';
        btn.addEventListener('click', () => playJp(resp.jp, bubble));
        row.appendChild(btn);
        if (autoPlay) playJp(resp.jp);
      }
      if (resp.correction) {
        const userRows = els.messages.querySelectorAll('.tutor-message--user');
        if (userRows.length > 0) userRows[userRows.length - 1].classList.add('tutor-message--faded');
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
      if (state.pendingGreeting) appendMessage('assistant', JSON.stringify({ note: state.pendingGreeting }));
      els.input.placeholder = 'Press Send (Shift+Enter) to begin';
      return;
    }
    els.input.placeholder = 'Type a message…';

    if (state.debugMode) {
      if (!state.systemPrompt) {
        try {
          const resp = await fetch('/api/tutor/system-prompt?mode=' + encodeURIComponent(els.modeSelect.value));
          state.systemPrompt = resp.ok ? await resp.text() : '(failed to load)';
        } catch (_) {
          state.systemPrompt = '(failed to load)';
        }
      }
      appendDebugBlock('system', state.systemPrompt);
      for (const msg of state.history) appendDebugBlock(msg.role, msg.content);
      return;
    }

    for (const msg of state.history) appendMessage(msg.role, msg.content, { autoPlay: false });
  }

  return {
    appendDebugBlock,
    appendLoadingDots,
    appendMessage,
    autoResize,
    renderAllMessages,
  };
}
