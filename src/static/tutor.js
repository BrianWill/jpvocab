import { PROVIDER_MODELS, checkVoicevoxAvailable } from './common.js';
import { els, state, currentPrompt, setSendDisabled } from './tutor-state.js';
import { createTutorAudio } from './tutor-audio.js';
import { createTutorView, parseResponse } from './tutor-view.js';
import { createTutorChat } from './tutor-chat.js';
import { createTutorPrompts } from './tutor-prompts.js';

function populateModelSelect() {
  const providers = state.providers || {};
  const hasProviders = PROVIDER_MODELS.some(provider => providers[provider.key]);
  const missingLines = PROVIDER_MODELS
    .filter(provider => !providers[provider.key])
    .map(provider => provider.label + ': set ' + provider.envKey + ' to enable');
  const tip = missingLines.length ? missingLines.join('\n') + '\n— then restart the program' : null;

  let firstAvailSet = false;
  const optgroupsHtml = PROVIDER_MODELS.map(({ key, label, models }) => {
    const avail = providers[key];
    const groupLabel = avail ? label : label + ' — no API key';
    const options = models.map(([value, text]) => {
      const selected = avail && !firstAvailSet ? ' selected' : '';
      if (selected) firstAvailSet = true;
      return '<option value="' + value + '"' + selected + '>' + text + '</option>';
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

  setSendDisabled(els, state, !hasProviders || state.sending);
}

async function init() {
  const audio = createTutorAudio({ els, state, parseResponse });
  const view = createTutorView({ els, state, playJp: audio.playJp });
  const chat = createTutorChat({
    els,
    state,
    appendDebugBlock: view.appendDebugBlock,
    appendLoadingDots: view.appendLoadingDots,
    appendMessage: view.appendMessage,
    autoResize: view.autoResize,
    currentPrompt: () => currentPrompt(els, state),
    renderAllMessages: view.renderAllMessages,
    setSendDisabled: disabled => setSendDisabled(els, state, disabled),
    stopAudio: audio.stopAudio,
  });
  const prompts = createTutorPrompts({
    els,
    state,
    currentPrompt: () => currentPrompt(els, state),
    startNewChat: chat.startNewChat,
  });

  const [promptsResp, providersResp, sessionResp] = await Promise.allSettled([
    fetch('/api/tutor/prompts').then(resp => resp.json()),
    fetch('/api/providers').then(resp => resp.json()),
    fetch('/api/tutor/session').then(resp => resp.json()),
  ]);
  checkVoicevoxAvailable();

  state.prompts = promptsResp.status === 'fulfilled' ? (promptsResp.value || []) : [];
  state.providers = providersResp.status === 'fulfilled' ? (providersResp.value.ai || {}) : {};
  const session = sessionResp.status === 'fulfilled' ? sessionResp.value : {};

  prompts.populateModeSelect();
  populateModelSelect();
  chat.restoreSession(session);

  els.modeSelect.addEventListener('change', () => {
    prompts.updatePromptButtons();
    chat.startNewChat();
  });
  els.levelSelect.addEventListener('change', chat.startNewChat);
  els.btnNewChat.addEventListener('click', chat.startNewChat);
  els.btnAddPrompt.addEventListener('click', prompts.openAddPromptModal);
  els.btnEditPrompt.addEventListener('click', prompts.openEditPromptModal);
  els.btnDeletePrompt.addEventListener('click', prompts.deleteCurrentPrompt);
  els.btnCancelPrompt.addEventListener('click', () => els.promptModal.close());
  els.btnSavePrompt.addEventListener('click', prompts.saveCustomPrompt);

  els.promptModal.addEventListener('click', event => {
    const rect = els.promptModal.getBoundingClientRect();
    if (event.clientX < rect.left || event.clientX > rect.right ||
        event.clientY < rect.top || event.clientY > rect.bottom) {
      els.btnCancelPrompt.classList.remove('btn-modal-cancel--flash');
      void els.btnCancelPrompt.offsetWidth;
      els.btnCancelPrompt.classList.add('btn-modal-cancel--flash');
    }
  });

  els.btnDebug.addEventListener('click', () => {
    state.debugMode = !state.debugMode;
    els.btnDebug.classList.toggle('btn-header--active', state.debugMode);
    view.renderAllMessages();
  });

  els.form.addEventListener('submit', event => {
    event.preventDefault();
    if (state.waitingForStart) {
      chat.kickoffChat();
      return;
    }
    const text = els.input.value.trim();
    if (!text || state.sending) return;
    els.input.value = '';
    view.autoResize(els.input);
    chat.sendMessage(text);
  });

  els.input.addEventListener('keydown', event => {
    if (event.key === 'Enter' && !event.shiftKey && state.waitingForStart) {
      event.preventDefault();
      chat.kickoffChat();
      return;
    }
    if (event.key === 'Enter' && event.shiftKey) {
      event.preventDefault();
      els.form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    }
  });

  els.input.addEventListener('input', () => view.autoResize(els.input));

  function handleGlobalTutorHotkeys(event) {
    if (event.altKey && !event.shiftKey && !event.ctrlKey && !event.metaKey && event.key.toLowerCase() === 'j') {
      event.preventDefault();
      event.stopPropagation();
      (els.btnMicJa || els.btnMicLegacy)?.click();
      return;
    }
    if (event.altKey && !event.shiftKey && !event.ctrlKey && !event.metaKey && event.key.toLowerCase() === 'e') {
      event.preventDefault();
      event.stopPropagation();
      (els.btnMicEn || els.btnMicLegacy)?.click();
      return;
    }
    if (event.altKey && event.key === 'p') {
      event.preventDefault();
      event.stopPropagation();
      audio.handleReplayLastAssistant();
    }
  }

  window.addEventListener('keydown', handleGlobalTutorHotkeys, true);
  document.addEventListener('keydown', handleGlobalTutorHotkeys);

  audio.initMic(view.autoResize);
  els.input.focus();
}

init();
