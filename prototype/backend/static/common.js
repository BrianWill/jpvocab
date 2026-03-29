// Initialize settings modal
function initializeSettings() {
  const settingsBtn = document.getElementById('settings-btn');
  const settingsModal = document.getElementById('settings-modal-backdrop');

  if (!settingsBtn || !settingsModal) return;

  const closeModal = () => settingsModal.classList.add('hidden');

  settingsBtn.addEventListener('click', () => settingsModal.classList.remove('hidden'));
  settingsModal.querySelector('.modal-close')?.addEventListener('click', closeModal);
  document.getElementById('settings-close-btn')?.addEventListener('click', closeModal);
  settingsModal.addEventListener('click', (e) => {
    if (e.target === settingsModal) closeModal();
  });
}

initializeSettings();
