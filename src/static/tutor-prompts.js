import { escapeHtml } from './html-utils.js';

export function createTutorPrompts({ els, state, currentPrompt, startNewChat }) {
  function updatePromptButtons() {
    const prompt = currentPrompt();
    els.btnEditPrompt.style.display = prompt && prompt.can_remove ? '' : 'none';
  }

  function populateModeSelect() {
    const saved = els.modeSelect.value;

    const builtIn = state.prompts.filter(prompt => !prompt.can_remove).sort((a, b) => a.label.localeCompare(b.label));
    const custom = state.prompts.filter(prompt => prompt.can_remove).sort((a, b) => a.label.localeCompare(b.label));
    const option = prompt => '<option value="' + prompt.id + '">' + escapeHtml(prompt.label) + '</option>';

    els.modeSelect.innerHTML = (builtIn.length && custom.length)
      ? '<optgroup label="Built-in">' + builtIn.map(option).join('') + '</optgroup>' +
        '<optgroup label="Custom">' + custom.map(option).join('') + '</optgroup>'
      : [...builtIn, ...custom].map(option).join('');

    if (saved && state.prompts.some(prompt => String(prompt.id) === saved)) {
      els.modeSelect.value = saved;
    }
    updatePromptButtons();
  }

  function openAddPromptModal() {
    state.editingPromptId = null;
    els.promptModalTitle.textContent = 'Add Custom Prompt';
    const base = currentPrompt();
    els.promptLabelInput.value = base ? base.label + ' (custom)' : '';
    els.promptSystemInput.value = base ? base.system_prompt : '';
    els.promptGreetInput.value = base ? base.greeting : '';
    els.promptLangInput.value = base?.lang_input || 'ja';
    els.promptModalError.style.display = 'none';
    els.btnSavePrompt.disabled = false;
    els.btnDeletePrompt.style.display = 'none';
    els.promptModal.showModal();
  }

  function openEditPromptModal() {
    const prompt = currentPrompt();
    if (!prompt || !prompt.can_remove) return;
    state.editingPromptId = prompt.id;
    els.promptModalTitle.textContent = 'Edit Prompt';
    els.promptLabelInput.value = prompt.label;
    els.promptSystemInput.value = prompt.system_prompt;
    els.promptGreetInput.value = prompt.greeting;
    els.promptLangInput.value = prompt.lang_input || 'ja';
    els.promptModalError.style.display = 'none';
    els.btnSavePrompt.disabled = false;
    els.btnDeletePrompt.style.display = '';
    els.btnDeletePrompt.dataset.armed = '';
    els.btnDeletePrompt.textContent = 'Delete';
    els.promptModal.showModal();
  }

  async function saveCustomPrompt() {
    const label = els.promptLabelInput.value.trim();
    const systemPrompt = els.promptSystemInput.value.trim();
    const greeting = els.promptGreetInput.value.trim();
    const langInput = els.promptLangInput.value;

    if (!label || !systemPrompt) {
      els.promptModalError.textContent = 'Name and Instructions are required.';
      els.promptModalError.style.display = '';
      return;
    }

    els.btnSavePrompt.disabled = true;
    els.promptModalError.style.display = 'none';

    const isEdit = state.editingPromptId !== null;
    const url = isEdit ? '/api/tutor/prompts/' + state.editingPromptId : '/api/tutor/prompts';
    const method = isEdit ? 'PATCH' : 'POST';

    try {
      const resp = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label, system_prompt: systemPrompt, greeting, lang_input: langInput }),
      });
      if (!resp.ok) {
        const msg = await resp.text();
        throw new Error(msg || resp.statusText);
      }
      const saved = await resp.json();
      els.promptModal.close();

      if (isEdit) {
        state.prompts = state.prompts.map(prompt => prompt.id === saved.id ? saved : prompt);
      } else {
        state.prompts.push(saved);
      }
      populateModeSelect();
      els.modeSelect.value = String(saved.id);
      updatePromptButtons();
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

    if (els.btnDeletePrompt.dataset.armed !== 'true') {
      els.btnDeletePrompt.dataset.armed = 'true';
      els.btnDeletePrompt.textContent = 'Confirm delete?';
      setTimeout(() => {
        els.btnDeletePrompt.dataset.armed = '';
        els.btnDeletePrompt.textContent = 'Delete';
      }, 3000);
      return;
    }

    els.btnDeletePrompt.dataset.armed = '';
    els.btnDeletePrompt.textContent = 'Delete';

    try {
      const resp = await fetch('/api/tutor/prompts/' + prompt.id, { method: 'DELETE' });
      if (!resp.ok) return;
      els.promptModal.close();
      state.prompts = state.prompts.filter(item => item.id !== prompt.id);
      populateModeSelect();
      startNewChat();
    } catch (_) {}
  }

  return {
    deleteCurrentPrompt,
    openAddPromptModal,
    openEditPromptModal,
    populateModeSelect,
    saveCustomPrompt,
    updatePromptButtons,
  };
}
