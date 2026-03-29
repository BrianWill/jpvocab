// Initialize settings modal
function initializeSettings() {
  const settingsBtn = document.getElementById('settings-btn');
  const settingsModal = document.getElementById('settings-modal-backdrop');
  const settingsCloseBtn = document.getElementById('settings-close-btn');

  if (!settingsBtn || !settingsModal || !settingsCloseBtn) return;

  settingsBtn.addEventListener('click', () => {
    settingsModal.classList.remove('hidden');
  });

  settingsCloseBtn.addEventListener('click', () => {
    settingsModal.classList.add('hidden');
  });

  settingsModal.addEventListener('click', (e) => {
    if (e.target === settingsModal) {
      settingsModal.classList.add('hidden');
    }
  });
}

initializeSettings();
