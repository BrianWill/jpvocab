const els = {
  form: document.querySelector('.add-word-form'),
  modal: document.getElementById('batch-modal'),
  modalBody: document.getElementById('batch-modal-body'),
  modalFooter: document.getElementById('batch-modal-footer'),
  modalStatus: document.getElementById('batch-modal-status'),
};

const state = {
  addedWords: [],
  controller: null,
  phase: 'idle', // 'loading' | 'done' | 'cancelled' | 'error'
};

els.modal.addEventListener('click', function (e) {
  if (e.target === els.modal && state.phase !== 'loading') {
    els.modal.style.display = 'none';
    location.reload();
  }
});

els.form.addEventListener('submit', function (e) {
  e.preventDefault();
  state.addedWords = [];
  state.phase = 'loading';
  els.modalBody.innerHTML = '';
  els.modal.style.display = 'flex';
  setStatus('loading', 'Processing\u2026');
  initFooter();
  state.controller = new AbortController();
  runBatch(new FormData(els.form), state.controller.signal);
});

async function runBatch(formData, signal) {
  try {
    var resp = await fetch('/admin/words/batch', {
      method: 'POST',
      body: formData,
      signal: signal,
    });
    var reader = resp.body.getReader();
    var decoder = new TextDecoder();
    var buf = '';
    while (true) {
      var chunk = await reader.read();
      if (chunk.done) break;
      buf += decoder.decode(chunk.value, { stream: true });
      var parts = buf.split('\n\n');
      buf = parts.pop();
      for (var i = 0; i < parts.length; i++) {
        var part = parts[i];
        if (!part.startsWith('data: ')) continue;
        var data = JSON.parse(part.slice(6));
        if (data.done) {
          state.phase = 'done';
          setStatus('done', state.addedWords.length + ' added, ' + (els.modalBody.children.length - state.addedWords.length) + ' skipped');
          updateFooter();
          return;
        }
        appendResult(data);
        if (data.added) state.addedWords.push(data.word);
        updateFooter();
      }
    }
    state.phase = 'done';
    setStatus('done', state.addedWords.length + ' added');
    updateFooter();
  } catch (err) {
    if (err.name === 'AbortError') {
      state.phase = 'cancelled';
      setStatus('cancelled', 'Cancelled \u2014 ' + state.addedWords.length + ' word(s) added before cancel');
    } else {
      state.phase = 'error';
      setStatus('error', 'Error: ' + err.message);
    }
    updateFooter();
  }
}

function appendResult(data) {
  var row = document.createElement('div');
  row.className = 'word-result-row ' + (data.added ? 'result-added' : 'result-skipped');

  var wordSpan = data.input !== data.word
    ? esc(data.word) + '<span class="result-original"> \u2190 ' + esc(data.input) + '</span>'
    : esc(data.word);

  var badge = data.added
    ? '<span class="result-badge badge-added">added</span>'
    : '<span class="result-badge badge-skipped">' + esc(data.reason) + '</span>';

  var details = '';
  if (data.reading || data.part_of_speech || data.meaning || data.example_jp) {
    var items = [];
    if (data.reading)        items.push(detail('reading', data.reading));
    if (data.part_of_speech) items.push(detail('pos', data.part_of_speech));
    if (data.meaning)        items.push(detail('meaning', data.meaning));
    if (data.example_jp)     items.push(detail('ex.', data.example_jp + (data.example_en ? '  ' + data.example_en : '')));
    details = '<div class="word-result-details">' + items.join('') + '</div>';
  }

  row.innerHTML =
    '<div class="word-result-main"><span class="result-word">' + wordSpan + '</span>' + badge + '</div>' +
    details;
  els.modalBody.appendChild(row);
}

function detail(label, text) {
  return '<span class="detail-item"><span class="detail-label">' + esc(label) + '</span> ' + esc(text) + '</span>';
}

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

function setStatus(type, text) {
  var spinner = type === 'loading' ? '<span class="spinner"></span>' : '';
  els.modalStatus.className = 'modal-status modal-status-' + type;
  els.modalStatus.innerHTML = spinner + '<span>' + esc(text) + '</span>';
}

// Render all three buttons once when the modal opens.
function initFooter() {
  els.modalFooter.innerHTML =
    '<button id="btn-cancel">Cancel request</button>' +
    '<button id="btn-remove" class="btn-danger">Remove added words</button>' +
    '<button id="btn-close">Close</button>';

  document.getElementById('btn-cancel').addEventListener('click', function () {
    if (state.controller) state.controller.abort();
  });
  document.getElementById('btn-remove').addEventListener('click', async function () {
    var words = state.addedWords.slice();
    await fetch('/admin/words/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ words: words }),
    });
    state.addedWords = [];
    els.modalBody.querySelectorAll('.badge-added').forEach(function (badge) {
      badge.className = 'result-badge badge-removed';
      badge.textContent = 'removed';
    });
    setStatus('done', 'Removed \u2014 0 words in lexicon from this batch');
    updateFooter();
  });
  document.getElementById('btn-close').addEventListener('click', function () {
    els.modal.style.display = 'none';
    location.reload();
  });
  updateFooter();
}

// Sync button enabled/disabled state and remove-button label to current state.
function updateFooter() {
  var btnCancel = document.getElementById('btn-cancel');
  var btnRemove = document.getElementById('btn-remove');
  var btnClose  = document.getElementById('btn-close');
  if (!btnCancel) return;

  btnCancel.disabled = state.phase !== 'loading';
  btnRemove.disabled = state.addedWords.length === 0 || state.phase === 'loading';
  btnRemove.textContent = state.addedWords.length > 0
    ? 'Remove the ' + state.addedWords.length + ' added word' + (state.addedWords.length === 1 ? '' : 's')
    : 'Remove added words';
  btnClose.disabled = state.phase === 'loading';
}
