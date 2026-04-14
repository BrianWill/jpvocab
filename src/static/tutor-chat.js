export function createTutorChat({
  els,
  state,
  appendDebugBlock,
  appendLoadingDots,
  appendMessage,
  autoResize,
  currentPrompt,
  renderAllMessages,
  setSendDisabled,
  stopAudio,
}) {
  function startNewChat() {
    stopAudio();
    state.history = [];
    state.systemPrompt = null;
    state.waitingForStart = true;
    const prompt = currentPrompt();
    if (!prompt) return;
    state.pendingGreeting = prompt.greeting || '{}';
    renderAllMessages();
    fetch('/api/tutor/session', { method: 'DELETE' });
  }

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
          ai_model: aiModel,
          tutor_mode: els.modeSelect.value,
          jlpt_level: els.levelSelect.value,
          messages: [],
        }),
      });
      if (!resp.ok) throw new Error((await resp.text()).trim() || resp.statusText);
      const data = await resp.json();
      const response = data.response || { en: '(empty response)' };
      const rawJson = JSON.stringify(response);
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
          ai_model: aiModel,
          tutor_mode: els.modeSelect.value,
          jlpt_level: els.levelSelect.value,
          messages: state.history,
        }),
      });

      if (!resp.ok) {
        const errText = await resp.text();
        throw new Error(errText.trim() || resp.statusText);
      }

      const data = await resp.json();
      const response = data.response || { en: '(empty response)' };
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
      state.history.pop();
    } finally {
      state.sending = false;
      setSendDisabled(false);
      els.input.focus();
      autoResize(els.input);
    }
  }

  function restoreSession(session) {
    if (!session.messages || session.messages.length === 0) {
      startNewChat();
      return;
    }
    if (session.tutor_mode) els.modeSelect.value = session.tutor_mode;
    if (session.ai_model) els.modelSelect.value = session.ai_model;
    if (session.jlpt_level) els.levelSelect.value = session.jlpt_level;

    state.history = session.messages;
    renderAllMessages();
  }

  return {
    kickoffChat,
    restoreSession,
    sendMessage,
    startNewChat,
  };
}
