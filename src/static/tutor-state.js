import { PROVIDER_MODELS } from './common.js';

export const els = {
  levelSelect: document.getElementById('tutor-level-select'),
  modeSelect: document.getElementById('tutor-mode-select'),
  modelSelect: document.getElementById('tutor-model-select'),
  providerInfo: document.getElementById('tutor-provider-info'),
  btnNewChat: document.getElementById('btn-new-chat'),
  btnDebug: document.getElementById('btn-debug-toggle'),
  btnAddPrompt: document.getElementById('btn-add-prompt'),
  btnEditPrompt: document.getElementById('btn-edit-prompt'),
  btnDeletePrompt: document.getElementById('btn-delete-prompt'),
  messages: document.getElementById('tutor-messages'),
  form: document.getElementById('tutor-form'),
  input: document.getElementById('tutor-input'),
  btnMicLegacy: document.getElementById('btn-tutor-mic'),
  btnMicJa: document.getElementById('btn-tutor-mic-ja'),
  btnMicEn: document.getElementById('btn-tutor-mic-en'),
  btnSend: document.getElementById('btn-tutor-send'),
  promptModal: document.getElementById('prompt-modal'),
  promptModalTitle: document.getElementById('prompt-modal-title'),
  promptForm: document.getElementById('prompt-form'),
  promptLabelInput: document.getElementById('prompt-label-input'),
  promptSystemInput: document.getElementById('prompt-system-input'),
  promptGreetInput: document.getElementById('prompt-greeting-input'),
  promptLangInput: document.getElementById('prompt-lang-input'),
  promptModalError: document.getElementById('prompt-modal-error'),
  btnCancelPrompt: document.getElementById('btn-cancel-prompt'),
  btnSavePrompt: document.getElementById('btn-save-prompt'),
};

export const state = {
  providers: null,
  prompts: [],
  history: [],
  sending: false,
  debugMode: false,
  systemPrompt: null,
  listening: false,
  listeningLang: null,
  waitingForStart: false,
  pendingGreeting: null,
  editingPromptId: null,
  playingBubble: null,
  playingTimer: null,
};

export function currentPrompt(elsRef, stateRef) {
  const id = parseInt(elsRef.modeSelect.value, 10);
  return stateRef.prompts.find(prompt => prompt.id === id) || stateRef.prompts[0] || null;
}

export function setSendDisabled(elsRef, stateRef, disabled) {
  const hasProviders = PROVIDER_MODELS.some(provider => (stateRef.providers || {})[provider.key]);
  elsRef.btnSend.disabled = disabled || !hasProviders;
  elsRef.input.disabled = disabled || !hasProviders;
}
