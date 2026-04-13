export function bindBackdropClose(backdropEl, onClose) {
  if (!backdropEl || typeof onClose !== 'function') return;
  backdropEl.addEventListener('click', event => {
    if (event.target === event.currentTarget) onClose();
  });
}

export function bindEscapeClose(onClose) {
  if (typeof onClose !== 'function') return () => {};
  const handler = event => {
    if (event.key === 'Escape') onClose(event);
  };
  document.addEventListener('keydown', handler);
  return () => document.removeEventListener('keydown', handler);
}

export function setModalOpen(backdropEl, open) {
  if (!backdropEl) return;
  backdropEl.classList.toggle('hidden', !open);
}

export function setButtonBusy(button, busyHtml, idleText) {
  if (!button) return;
  if (busyHtml) {
    button.disabled = true;
    button.innerHTML = busyHtml;
    return;
  }
  button.disabled = false;
  button.textContent = idleText;
}
