export function createWordResultModalController({
  els,
  state,
  closeModal,
  canClose,
  closeButtonId,
  onGenerateAll,
  onPlayExampleSentence,
  onUploadComplete,
  onSaveRowEdits,
  bindWordResultEditorEvents,
  bindWordResultImageUpload,
}) {
  function openRemoveConfirm(message, action) {
    state.pendingRemoveAction = action;
    els.removeConfirmText.textContent = message;
    els.removeConfirmModalBackdrop.classList.remove('hidden');
  }

  function closeRemoveConfirm() {
    state.pendingRemoveAction = null;
    els.removeConfirmModalBackdrop.classList.add('hidden');
  }

  function openGenerateConfirm() {
    const addedCount = els.addResultBody.querySelectorAll('.result-added .btn-generate:not(.btn-generate--busy):not([disabled])').length;
    const skippedCount = els.addResultBody.querySelectorAll('.result-skipped .btn-generate:not(.btn-generate--busy):not([disabled])').length;
    els.generateConfirmAddedText.textContent = addedCount + ' newly added words';
    els.generateConfirmSkippedText.textContent = skippedCount + ' already existing words';
    els.generateConfirmAddedCheckbox.checked = true;
    els.generateConfirmSkippedCheckbox.checked = false;
    els.generateConfirmModalBackdrop.classList.remove('hidden');
  }

  function closeGenerateConfirm() {
    els.generateConfirmModalBackdrop.classList.add('hidden');
  }

  function bindBaseEvents() {
    if (state.eventsBound) return;
    state.eventsBound = true;

    document.addEventListener('mousedown', () => {
      if (els.splitBtnMenu) els.splitBtnMenu.hidden = true;
    });

    els.addResultModalBackdrop.addEventListener('click', event => {
      if (event.target === els.addResultModalBackdrop && canClose()) closeModal();
    });
    els.addResultClose.addEventListener('click', closeModal);

    els.addResultBody.addEventListener('click', event => {
      if (!event.target.closest('.detail-ex-play')) return;
      const row = event.target.closest('.word-result-row');
      const jpInput = event.target.closest('.detail-ex-inputs')?.querySelector('.detail-input:not(.detail-input--en)');
      const text = jpInput?.textContent.trim();
      if (text) onPlayExampleSentence({ row, text });
    });

    bindWordResultEditorEvents({
      containerEl: els.addResultBody,
      footerEl: els.addResultFooter,
      closeButtonId,
      state,
      onSaveRowEdits,
    });

    bindWordResultImageUpload({
      containerEl: els.addResultBody,
      onUploadComplete,
    });

    els.removeConfirmModalBackdrop.addEventListener('click', event => {
      if (event.target === els.removeConfirmModalBackdrop) closeRemoveConfirm();
    });
    els.removeConfirmCancel.addEventListener('click', closeRemoveConfirm);
    els.removeConfirmOk.addEventListener('click', () => {
      const action = state.pendingRemoveAction;
      closeRemoveConfirm();
      if (action) action();
    });

    els.generateConfirmModalBackdrop.addEventListener('click', event => {
      if (event.target === els.generateConfirmModalBackdrop) closeGenerateConfirm();
    });
    els.generateConfirmCancel.addEventListener('click', closeGenerateConfirm);
    els.generateConfirmOk.addEventListener('click', () => {
      const includeAdded = els.generateConfirmAddedCheckbox.checked;
      const includeSkipped = els.generateConfirmSkippedCheckbox.checked;
      closeGenerateConfirm();
      onGenerateAll(includeAdded, includeSkipped);
    });
  }

  return {
    bindBaseEvents,
    closeGenerateConfirm,
    closeRemoveConfirm,
    openGenerateConfirm,
    openRemoveConfirm,
  };
}
