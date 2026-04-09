import { PROVIDER_MODELS, playDing } from './common.js';

let _els, _state, _storyId, _onTranslationDone, _stopPlayback;

function formatElapsedSeconds(seconds) {
  return `${Math.max(0, Math.round(seconds))}s`;
}

function formatTokenCount(count) {
  return new Intl.NumberFormat().format(Math.max(0, count || 0));
}

function getTranslationTarget() {
  if (!_state.story) return null;
  if (_state.translationChunkId) {
    return (_state.story.chunks || []).find(chunk => String(chunk.id || 0) === String(_state.translationChunkId)) || null;
  }
  return {
    sentences: _state.story.sentences || [],
    storyWords: _state.story.storyWords || [],
  };
}

function updateTranslationCountsText(extraWordInfoCount = null) {
  const target = getTranslationTarget();
  const sentenceCount = Array.isArray(target?.sentences) ? target.sentences.length : 0;
  const uniqueWordCount = Array.isArray(target?.storyWords) ? target.storyWords.length : 0;
  let text = `${sentenceCount} sentence${sentenceCount === 1 ? '' : 's'}, ${uniqueWordCount} unique word${uniqueWordCount === 1 ? '' : 's'}`;
  if (typeof extraWordInfoCount === 'number') {
    text += ` (${extraWordInfoCount} need word info)`;
  }
  _els.genTranslationCounts.textContent = text;
}

function stopTranslationTimer() {
  if (_state.translationElapsedTimer !== null) {
    clearInterval(_state.translationElapsedTimer);
    _state.translationElapsedTimer = null;
  }
}

function startTranslationTimer() {
  stopTranslationTimer();
  _state.translationStartedAt = Date.now();
  _els.genTranslationElapsed.textContent = '0s elapsed';
  _state.translationElapsedTimer = setInterval(() => {
    const elapsedSeconds = (Date.now() - _state.translationStartedAt) / 1000;
    _els.genTranslationElapsed.textContent = `${formatElapsedSeconds(elapsedSeconds)} elapsed`;
  }, 1000);
}

function resetTranslationProgressUi() {
  stopTranslationTimer();
  _state.translationStartedAt = 0;
  _state.translationWordInfoCount = null;
  updateTranslationCountsText();
  _els.genTranslationElapsed.textContent = '0s elapsed';
  _els.genTranslationSummary.textContent = '';
  _els.genTranslationSummary.classList.add('hidden');
  _els.genTranslationSpinner.classList.remove('hidden');
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

export function openTranslationModal(chunkId = null, chunkLabel = '') {
  _state.translationChunkId = chunkId || null;
  _state.translationChunkLabel = chunkLabel || '';
  resetTranslationProgressUi();
  if (_els.genTranslationCopy) {
    const target = _state.translationChunkLabel || 'this chunk';
    _els.genTranslationCopy.innerHTML = `Generate English sentence translations for ${target} using AI?<br>Any existing translations will be replaced.`;
  }
  setTranslationModalGenerating(false);
  _els.genTranslationModalBackdrop.classList.remove('hidden');
}

function closeTranslationModal() {
  if (_state.translating) return;
  stopTranslationTimer();
  _els.genTranslationModalBackdrop.classList.add('hidden');
}

// ── Init ──────────────────────────────────────────────────────────────────────
export function initGenerateModals(els, state, { storyId, onTranslationDone, stopPlayback }) {
  _els = els;
  _state = state;
  _storyId = storyId;
  _onTranslationDone = onTranslationDone;
  _stopPlayback = stopPlayback;

  if (els.genTranslationBtn) {
    els.genTranslationBtn.addEventListener('click', () => openTranslationModal(null, 'this story'));
  }
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
      if (!res.ok) return null;
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      let doneMsg = null;
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
          onMsg(msg);
          if (msg.allDone) doneMsg = msg;
        }
      }
      if (buf.trim()) {
        try {
          const msg = JSON.parse(buf);
          onMsg(msg);
          if (msg.allDone) doneMsg = msg;
        } catch (_) {}
      }
      return doneMsg;
    };

    resetTranslationProgressUi();
    startTranslationTimer();
    _els.genTranslationStatusText.textContent = _state.translationChunkLabel ? `Generating ${_state.translationChunkLabel}…` : 'Generating…';

    const runPhase = async (url, onMsg) => {
      try {
        const res = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ai_model: aiModel, chunk_id: _state.translationChunkId }),
          signal: _state.translationController.signal,
        });
        return await readNDJSON(res, onMsg);
      } catch (_) {
        return null;
      }
    };

    const [phase1Done, phase2Done] = await Promise.all([
      runPhase(`/api/stories/${_storyId}/generate-translation`, msg => {
        if (typeof msg.sentenceCount === 'number') updateTranslationCountsText(_state.translationWordInfoCount);
      }),
      runPhase(`/api/stories/${_storyId}/generate-word-info`, msg => {
        if (typeof msg.wordCount === 'number') {
          _state.translationWordInfoCount = msg.wordCount;
          updateTranslationCountsText(_state.translationWordInfoCount);
        }
      }),
    ]);

    stopTranslationTimer();
    _state.translationController = null;

    if (phase1Done && phase2Done) {
      const elapsedSeconds = _state.translationStartedAt ? (Date.now() - _state.translationStartedAt) / 1000 : 0;
      const totalTokens = (phase1Done.totalTokens || 0) + (phase2Done.totalTokens || 0);
      playDing();
      _state.translating = false;
      _els.genTranslationSpinner.classList.add('hidden');
      _els.genTranslationModalCancelGen.classList.add('hidden');
      _els.genTranslationStatusText.textContent = 'Done.';
      _els.genTranslationElapsed.textContent = `${formatElapsedSeconds(elapsedSeconds)} elapsed`;
      _els.genTranslationSummary.textContent = `${formatTokenCount(totalTokens)} token${totalTokens === 1 ? '' : 's'} used`;
      _els.genTranslationSummary.classList.remove('hidden');
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
