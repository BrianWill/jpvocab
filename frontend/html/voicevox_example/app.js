let speakers = [];

async function loadSpeakers() {
  try {
    const res = await fetch('/api/speakers');
    if (!res.ok) throw new Error(res.statusText);
    speakers = await res.json();

    const charSel = document.getElementById('char-select');
    charSel.innerHTML = speakers.map((s, i) =>
      `<option value="${i}">${s.name}</option>`
    ).join('');
    updateStyles();
  } catch (e) {
    document.getElementById('char-select').innerHTML = '<option>Unavailable</option>';
    setStatus('Could not reach VoiceVox at localhost:50021 — is it running?');
  }
}

function updateStyles() {
  const i = parseInt(document.getElementById('char-select').value, 10);
  const speaker = speakers[i];
  if (!speaker) return;
  document.getElementById('style-select').innerHTML = speaker.styles.map(s =>
    `<option value="${s.id}">${s.name}</option>`
  ).join('');
}

document.getElementById('char-select').addEventListener('change', updateStyles);

function syncVal(input, labelId, decimals) {
  document.getElementById(labelId).textContent =
    parseFloat(input.value).toFixed(decimals);
}

function setStatus(msg) {
  document.getElementById('status').textContent = msg;
}

async function generate() {
  const lines = document.getElementById('text-input').value
    .split('\n').map(l => l.trim()).filter(l => l.length > 0);
  if (lines.length === 0) return;

  const btn = document.getElementById('gen-btn');
  btn.disabled = true;
  setStatus(`Generating ${lines.length} item(s)…`);

  const body = {
    texts:           lines,
    speaker:         parseInt(document.getElementById('style-select').value, 10),
    speedScale:      parseFloat(document.getElementById('speed').value),
    pitchScale:      parseFloat(document.getElementById('pitch').value),
    intonationScale: parseFloat(document.getElementById('intonation').value),
    volumeScale:     parseFloat(document.getElementById('volume').value),
  };

  try {
    const res = await fetch('/api/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(await res.text());
    const results = await res.json();
    renderResults(results);
    const ok = results.filter(r => r.url).length;
    setStatus(`Done — ${ok} of ${results.length} file(s) saved.`);
  } catch (e) {
    setStatus('Error: ' + e.message);
  } finally {
    btn.disabled = false;
  }
}

function renderResults(results) {
  const container = document.getElementById('results');
  const frag = document.createDocumentFragment();

  for (const r of results) {
    const div = document.createElement('div');
    div.className = 'result-item';

    if (r.error) {
      div.innerHTML = `
        <div class="result-text">${esc(r.text)}</div>
        <div class="result-error">&#x26A0; ${esc(r.error)}</div>`;
    } else {
      div.innerHTML = `
        <div class="result-text">${esc(r.text)}</div>
        <div class="result-audio-row">
          <audio controls src="${r.url}"></audio>
          <a class="dl-link" href="${r.url}" download>↓ WAV</a>
        </div>`;
    }
    frag.appendChild(div);
  }

  container.insertBefore(frag, container.firstChild);
}

function esc(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

loadSpeakers();
