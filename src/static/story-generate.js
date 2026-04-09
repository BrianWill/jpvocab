import { PROVIDER_MODELS, playDing } from './common.js';

let _els, _state, _storyId, _onTranslationDone, _stopPlayback;

// ── Generate translation modal ────────────────────────────────────────────────
export function populateTranslationModelSelect(providers) {
  const hasProviders = PROVIDER_MODELS.some(p => providers[p.key]);
  const missingLines = PROVIDER_MODELS
    .filter(p => !providers[p.key])
    .map(p => p.label + ': set ' + p.envKey + ' to enable');
  const tip = missingLines.length ? missingLines.join('\n') + '\n— then restart the program' : null;

  let firstAvailSet = false;
  const optgroupsHtml = PROVIDER_MODELS.map(({ key, label, models }) => {
    const avail = providers[key];
    const groupLabel = avail ? label : label + ' — no API key';
    const options = models.map(([val, text], i) => {
      const sel = avail && !firstAvailSet && i === 0 ? ' selected' : '';
      if (sel) firstAvailSet = true;
      return '<option value="' + val + '"' + sel + '>' + text + '</option>';
    }).join('');
    return '<optgroup label="' + groupLabel + '"' + (avail ? '' : ' disabled') + '>' + options + '</optgroup>';
  }).join('');

  _els.genTranslationModelSelect.innerHTML =
    (!hasProviders ? '<option value="" selected>no API keys configured</option>' : '') +
    optgroupsHtml;
  _els.genTranslationModelSelect.disabled = !hasProviders;

  if (tip) {
    _els.genTranslationProviderInfo.dataset.tooltip = tip;
    _els.genTranslationProviderInfo.style.display = '';
  } else {
    _els.genTranslationProviderInfo.style.display = 'none';
  }

  _els.genTranslationModalConfirm.disabled = !hasProviders;
}

function setTranslationModalGenerating(generating) {
  _state.translating = generating;
  _els.genTranslationConfirmBody.classList.toggle('hidden', generating);
  _els.genTranslationProgressBody.classList.toggle('hidden', !generating);
  _els.genTranslationModalCancel.classList.toggle('hidden', generating);
  _els.genTranslationModalCancelGen.classList.toggle('hidden', !generating);
  _els.genTranslationModalConfirm.classList.toggle('hidden', generating);
  _els.genTranslationModalDone.classList.add('hidden');
  _els.genTranslationModalClose.disabled = generating;
}

function openTranslationModal() {
  setTranslationModalGenerating(false);
  _els.genTranslationModalBackdrop.classList.remove('hidden');
}

function closeTranslationModal() {
  if (_state.translating) return;
  _els.genTranslationModalBackdrop.classList.add('hidden');
}

// ── Init ──────────────────────────────────────────────────────────────────────
export function initGenerateModals(els, state, { storyId, onTranslationDone, stopPlayback }) {
  _els = els;
  _state = state;
  _storyId = storyId;
  _onTranslationDone = onTranslationDone;
  _stopPlayback = stopPlayback;

  els.genTranslationBtn.addEventListener('click', openTranslationModal);
  els.genTranslationModalClose.addEventListener('click', closeTranslationModal);
  els.genTranslationModalCancel.addEventListener('click', closeTranslationModal);
  els.genTranslationModalCancelGen.addEventListener('click', () => {
    if (_state.translationController) _state.translationController.abort();
  });
  els.genTranslationModalBackdrop.addEventListener('click', e => {
    if (e.target === els.genTranslationModalBackdrop) closeTranslationModal();
  });

  els.genTranslationModalConfirm.addEventListener('click', async () => {
    if (_state.translationController) return;

    const aiModel = _els.genTranslationModelSelect.value;
    if (!aiModel) return;

    _state.translationController = new AbortController();
    setTranslationModalGenerating(true);

    // Helper: consume an NDJSON stream; calls onMsg for each parsed message.
    // Returns true if the stream ended with {allDone: true}.
    const readNDJSON = async (res, onMsg) => {
      if (!res.ok) return false;
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      let done = false;
      while (true) {
        let value, streamDone;
        try { ({ value, done: streamDone } = await reader.read()); } catch (_) { break; }
        if (streamDone) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split('\n');
        buf = lines.pop();
        for (const line of lines) {
          if (!line.trim()) continue;
          let msg;
          try { msg = JSON.parse(line); } catch (_) { continue; }
          if (msg.allDone) { done = true; }
          else { onMsg(msg); }
        }
      }
      return done;
    };

    _els.genTranslationStatusText.textContent = 'Generating…';

    const runPhase = async (url, onMsg) => {
      try {
        const res = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ai_model: aiModel }),
          signal: _state.translationController.signal,
        });
        return await readNDJSON(res, onMsg);
      } catch (_) {
        return false;
      }
    };

    const [phase1Done, phase2Done] = await Promise.all([
      runPhase(`/api/stories/${_storyId}/generate-translation`, () => {}),
      runPhase(`/api/stories/${_storyId}/generate-word-info`, () => {}),
    ]);

    _state.translationController = null;

    if (phase1Done && phase2Done) {
      playDing();
      _state.translating = false;
      _els.genTranslationSpinner.classList.add('hidden');
      _els.genTranslationModalCancelGen.classList.add('hidden');
      _els.genTranslationStatusText.textContent = 'Done.';
      _els.genTranslationModalConfirm.classList.add('hidden');
      _els.genTranslationModalDone.classList.remove('hidden');
      _els.genTranslationModalClose.disabled = false;
      const updated = await fetch(`/api/stories/${_storyId}`).then(r => r.json());
      _onTranslationDone(updated);
    } else {
      setTranslationModalGenerating(false);
      closeTranslationModal();
    }
  });

  els.genTranslationModalDone.addEventListener('click', closeTranslationModal);
}
