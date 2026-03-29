// ── Settings modal step helpers ────────────────────────────────────────────
const SETTINGS_STEP_INTERVAL = 230;
let _settingsStepTimer = null;
function startSettingsStep(fn, ...args) {
  fn(...args);
  _settingsStepTimer = setInterval(() => fn(...args), SETTINGS_STEP_INTERVAL);
}
function stopSettingsStep() {
  clearInterval(_settingsStepTimer);
  _settingsStepTimer = null;
}
function adjustSettingsInput(id, delta) {
  const input = document.getElementById(id);
  if (!input) return;
  const val = parseInt(input.value, 10) || 5;
  input.value = delta > 0
    ? Math.min(995, Math.floor(val / 5) * 5 + 5)
    : Math.max(5, Math.ceil(val / 5) * 5 - 5);
}
function capSettingsInput(input) {
  if (input.value.length > 3) input.value = input.value.slice(0, 3);
  if (input.value === '0') input.value = '1';
}

// ── Settings modal ─────────────────────────────────────────────────────────
const SETTINGS_FILTER_KEYS = ['katakana', 'verbs', 'nouns', 'other'];

function injectSettingsModal() {
  if (document.getElementById('settings-modal-backdrop')) return;
  const el = document.createElement('div');
  el.id = 'settings-modal-backdrop';
  el.className = 'modal-backdrop hidden';
  el.innerHTML = `
    <div class="modal">
      <div class="modal-header">
        <span>Settings</span>
        <button class="modal-close">✕</button>
      </div>
      <div class="modal-body">
        <div class="settings-section-label">Drill defaults</div>
        <div class="restart-field">
          <label>Max total words</label>
          <div class="num-stepper">
            <button class="num-btn" type="button">−</button>
            <input type="number" id="settings-total-words" min="1">
            <button class="num-btn" type="button">+</button>
          </div>
        </div>
        <div class="restart-field">
          <label>Words per round</label>
          <div class="num-stepper">
            <button class="num-btn" type="button">−</button>
            <input type="number" id="settings-round-size" min="1">
            <button class="num-btn" type="button">+</button>
          </div>
        </div>
        <div class="restart-field restart-field-filter">
          <label>Word type</label>
          <div class="filter-chips">
            <button type="button" class="filter-chip" data-setting-filter="katakana">Katakana</button>
            <button type="button" class="filter-chip" data-setting-filter="verbs">Verbs</button>
            <button type="button" class="filter-chip" data-setting-filter="nouns">Nouns</button>
            <button type="button" class="filter-chip" data-setting-filter="other">Other</button>
          </div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="btn-cancel" id="settings-cancel-btn">Cancel</button>
        <button class="btn-save" id="settings-save-btn">Save</button>
      </div>
    </div>`;
  document.body.appendChild(el);
}

function initializeSettings() {
  const settingsBtn = document.getElementById('settings-btn');
  const settingsModal = document.getElementById('settings-modal-backdrop');
  if (!settingsBtn || !settingsModal) return;

  const saveBtn = document.getElementById('settings-save-btn');
  const closeModal = () => settingsModal.classList.add('hidden');

  const setDirty = () => saveBtn?.classList.add('btn-save--dirty');
  const clearDirty = () => saveBtn?.classList.remove('btn-save--dirty');

  // Open: fetch current settings, populate, and reset dirty state
  settingsBtn.addEventListener('click', async () => {
    const resp = await fetch('/api/settings/drill');
    const settings = await resp.json();

    const totalInput = document.getElementById('settings-total-words');
    const roundInput = document.getElementById('settings-round-size');
    if (totalInput) totalInput.value = settings.maxWords;
    if (roundInput) roundInput.value = settings.roundSize;

    settingsModal.querySelectorAll('.filter-chip[data-setting-filter]').forEach(btn => {
      btn.classList.toggle('active', settings.wordTypes.includes(btn.dataset.settingFilter));
    });

    clearDirty();
    settingsModal.classList.remove('hidden');
  });

  // Close
  settingsModal.querySelector('.modal-close')?.addEventListener('click', closeModal);
  document.getElementById('settings-cancel-btn')?.addEventListener('click', closeModal);
  settingsModal.addEventListener('click', (e) => {
    if (e.target === settingsModal) closeModal();
  });

  // Save
  saveBtn?.addEventListener('click', async () => {
    const totalVal = parseInt(document.getElementById('settings-total-words')?.value, 10);
    const roundVal = parseInt(document.getElementById('settings-round-size')?.value, 10);
    const wordTypes = SETTINGS_FILTER_KEYS.filter(f =>
      settingsModal.querySelector(`[data-setting-filter="${f}"]`)?.classList.contains('active')
    );
    if (wordTypes.length === 0) return;

    await fetch('/api/settings/drill', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        maxWords: Math.max(1, isNaN(totalVal) ? 100 : Math.min(995, totalVal)),
        roundSize: isNaN(roundVal) ? 10 : Math.max(1, Math.min(995, roundVal)),
        wordTypes,
      }),
    });
    closeModal();
  });

  // Filter chip toggles — mark dirty on change
  settingsModal.querySelectorAll('.filter-chip[data-setting-filter]').forEach(btn => {
    btn.addEventListener('click', () => { btn.classList.toggle('active'); setDirty(); });
  });

  // Stepper buttons — mark dirty on adjust; inputs mark dirty on manual edit
  const totalInput = document.getElementById('settings-total-words');
  if (totalInput) {
    const [tMinus, tPlus] = totalInput.closest('.num-stepper').querySelectorAll('.num-btn');
    tMinus.addEventListener('mousedown', () => startSettingsStep(adjustSettingsInput, 'settings-total-words', -5));
    tMinus.addEventListener('mouseup', stopSettingsStep);
    tMinus.addEventListener('mouseleave', stopSettingsStep);
    tPlus.addEventListener('mousedown', () => startSettingsStep(adjustSettingsInput, 'settings-total-words', 5));
    tPlus.addEventListener('mouseup', stopSettingsStep);
    tPlus.addEventListener('mouseleave', stopSettingsStep);
    totalInput.addEventListener('input', () => { capSettingsInput(totalInput); setDirty(); });
    tMinus.addEventListener('click', setDirty);
    tPlus.addEventListener('click', setDirty);
  }

  const roundInput = document.getElementById('settings-round-size');
  if (roundInput) {
    const [rMinus, rPlus] = roundInput.closest('.num-stepper').querySelectorAll('.num-btn');
    rMinus.addEventListener('mousedown', () => startSettingsStep(adjustSettingsInput, 'settings-round-size', -5));
    rMinus.addEventListener('mouseup', stopSettingsStep);
    rMinus.addEventListener('mouseleave', stopSettingsStep);
    rPlus.addEventListener('mousedown', () => startSettingsStep(adjustSettingsInput, 'settings-round-size', 5));
    rPlus.addEventListener('mouseup', stopSettingsStep);
    rPlus.addEventListener('mouseleave', stopSettingsStep);
    roundInput.addEventListener('input', () => { capSettingsInput(roundInput); setDirty(); });
    rMinus.addEventListener('click', setDirty);
    rPlus.addEventListener('click', setDirty);
  }
}

injectSettingsModal();
initializeSettings();
