import { PROVIDER_MODELS, getVoicevoxSettings, playDing } from './common.js';

let _els, _state, _storyId, _onAudioDone, _onTranslationDone, _stopPlayback;

// ── Generate audio modal ──────────────────────────────────────────────────────
function openGenModal() {
  _stopPlayback();
  // Always reset to confirmation state when opening.
  setModalGenerating(false);
  _els.genModalBackdrop.classList.remove('hidden');
}

function closeGenModal() {
  if (_state.generating) return;
  _els.genModalBackdrop.classList.add('hidden');
}

function setModalGenerating(generating) {
  _state.generating = generating;
  _els.genConfirmBody.classList.toggle('hidden', generating);
  _els.genProgressBody.classList.toggle('hidden', !generating);
  _els.genModalCancel.classList.toggle('hidden', generating);
  _els.genModalConfirm.classList.toggle('hidden', generating);
  _els.genCancelGenerationBtn.classList.toggle('hidden', !generating);
  _els.genModalDone.classList.add('hidden');
  _els.genModalClose.disabled = generating;
}

function buildSentenceList() {
  _els.genSentenceList.innerHTML = '';
  for (const sentence of _state.story.sentences) {
    const text = sentence.words.map(w => w.displayWord).join('');
    const preview = text.length > 35 ? text.slice(0, 35) + '…' : text;

    const row = document.createElement('div');
    row.className = 'gen-sentence-row';
    row.id = `gen-row-${sentence.position}`;

    const icon = document.createElement('span');
    icon.className = 'gen-sentence-icon';
    const dot = document.createElement('span');
    dot.className = 'gen-pending-dot';
    icon.appendChild(dot);

    const previewEl = document.createElement('span');
    previewEl.className = 'gen-sentence-preview';
    previewEl.textContent = preview;

    row.appendChild(icon);
    row.appendChild(previewEl);
    _els.genSentenceList.appendChild(row);
  }
}

function setRowActive(position) {
  const row = document.getElementById(`gen-row-${position}`);
  if (!row) return;
  row.classList.add('gen-sentence-row--active');
  const icon = row.querySelector('.gen-sentence-icon');
  icon.innerHTML = '<span class="spinner"></span>';
  row.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
}

function setRowDone(position) {
  const row = document.getElementById(`gen-row-${position}`);
  if (!row) return;
  row.classList.remove('gen-sentence-row--active');
  row.classList.add('gen-sentence-row--done');
  const icon = row.querySelector('.gen-sentence-icon');
  icon.innerHTML = '<span class="gen-checkmark">✓</span>';
}

function markNextRowActive(donePosition) {
  const positions = _state.story.sentences.map(s => s.position);
  const idx = positions.indexOf(donePosition);
  if (idx < 0 || idx + 1 >= positions.length) return;
  const nextRow = document.getElementById(`gen-row-${positions[idx + 1]}`);
  if (nextRow && !nextRow.classList.contains('gen-sentence-row--done')) {
    setRowActive(positions[idx + 1]);
  }
}

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
export function initGenerateModals(els, state, { storyId, onAudioDone, onTranslationDone, stopPlayback }) {
  _els = els;
  _state = state;
  _storyId = storyId;
  _onAudioDone = onAudioDone;
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
    _els.genTranslationStatusText.textContent = 'Translating…';

    let allDone = false;
    let baseStatusText = 'Translating…';
    let elapsedSecs = 0;
    let elapsedTimer = null;

    const startElapsedTimer = () => {
      elapsedTimer = setInterval(() => {
        elapsedSecs++;
        _els.genTranslationStatusText.textContent = `${baseStatusText} (${elapsedSecs}s)`;
      }, 1000);
    };
    const stopElapsedTimer = () => {
      if (elapsedTimer !== null) { clearInterval(elapsedTimer); elapsedTimer = null; }
    };

    try {
      const res = await fetch(`/api/stories/${_storyId}/generate-translation`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ai_model: aiModel }),
        signal: _state.translationController.signal,
      });

      if (res.ok) {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = '';
        while (true) {
          let value, done;
          try { ({ value, done } = await reader.read()); } catch (_) { break; }
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split('\n');
          buf = lines.pop();
          for (const line of lines) {
            if (!line.trim()) continue;
            let msg;
            try { msg = JSON.parse(line); } catch (_) { continue; }
            if (msg.status === 'translating') {
              baseStatusText =
                `Translating ${msg.sentenceCount} sentence${msg.sentenceCount !== 1 ? 's' : ''}` +
                (msg.wordCount > 0 ? ` and ${msg.wordCount} word${msg.wordCount !== 1 ? 's' : ''}` : '') +
                '…';
              _els.genTranslationStatusText.textContent = baseStatusText;
              startElapsedTimer();
            } else if (msg.allDone) {
              allDone = true;
            }
          }
        }
      }
    } catch (_) {
      // Aborted or network error.
    }

    stopElapsedTimer();
    _state.translationController = null;

    if (allDone) {
      playDing();
      _state.translating = false;
      _els.genTranslationSpinner.classList.add('hidden');
      _els.genTranslationModalCancelGen.classList.add('hidden');
      _els.genTranslationStatusText.textContent = baseStatusText.replace(/^Translating/, 'Translated').replace(/….*$/, '.');
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

  els.genBtn.addEventListener('click', openGenModal);
  els.genModalClose.addEventListener('click', closeGenModal);
  els.genModalCancel.addEventListener('click', closeGenModal);
  els.genModalBackdrop.addEventListener('click', e => { if (e.target === els.genModalBackdrop) closeGenModal(); });

  els.genModalConfirm.addEventListener('click', async () => {
    if (_state.generateController) return;

    const vv = getVoicevoxSettings();
    _state.generateController = new AbortController();

    const total = _state.story?.sentences.length ?? 0;
    let completed = 0;

    function updateProgressCount() {
      _els.genProgressCount.textContent = `${completed} / ${total} sentences`;
    }

    buildSentenceList();
    setModalGenerating(true);
    updateProgressCount();
    if (total > 0) {
      setRowActive(_state.story.sentences[0].position);
    }

    let allDone = false;
    try {
      const res = await fetch(`/api/stories/${_storyId}/generate-audio`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ speaker: vv.speaker, speedScale: vv.speedScale, intonationScale: vv.intonationScale }),
        signal: _state.generateController.signal,
      });

      if (res.ok) {
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = '';
        outer: while (true) {
          let value, done;
          try { ({ value, done } = await reader.read()); } catch (_) { break; }
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split('\n');
          buf = lines.pop();
          for (const line of lines) {
            if (!line.trim()) continue;
            let msg;
            try { msg = JSON.parse(line); } catch (_) { continue; }
            if (msg.sentencePosition !== undefined) {
              completed++;
              updateProgressCount();
              setRowDone(msg.sentencePosition);
              markNextRowActive(msg.sentencePosition);
            } else if (msg.allDone) {
              allDone = true;
              break outer;
            }
          }
        }
      }
    } catch (_) {
      // Aborted or network error.
    }

    _state.generateController = null;

    if (allDone) {
      playDing();
      // Unlock the modal so the user can close it manually; keep the progress view visible.
      _state.generating = false;
      _els.genCancelGenerationBtn.classList.add('hidden');
      _els.genModalDone.classList.remove('hidden');
      _els.genModalClose.disabled = false;
      const updated = await fetch(`/api/stories/${_storyId}`).then(r => r.json());
      _onAudioDone(updated);
    } else {
      // Cancelled or error — close and reset immediately.
      setModalGenerating(false);
      closeGenModal();
    }
  });

  els.genCancelGenerationBtn.addEventListener('click', () => {
    _state.generateController?.abort();
  });

  els.genModalDone.addEventListener('click', closeGenModal);
}
