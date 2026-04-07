const els = {
  totals: document.getElementById('tu-totals'),
  summary: document.getElementById('tu-summary'),
  logBody: document.getElementById('tu-log-body'),
  logEmpty: document.getElementById('tu-log-empty'),
};

function fmtNum(n) {
  return n.toLocaleString();
}

function fmtTime(iso) {
  const d = new Date(iso);
  const now = new Date();
  const diffMs = now - d;
  const diffMin = Math.floor(diffMs / 60000);
  const diffHr = Math.floor(diffMs / 3600000);
  const diffDay = Math.floor(diffMs / 86400000);
  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHr < 24) return `${diffHr}h ago`;
  if (diffDay < 7) return `${diffDay}d ago`;
  return d.toLocaleDateString();
}

function providerChip(provider) {
  return `<span class="tu-provider-chip ${provider}">${provider}</span>`;
}

function renderTotals(totals) {
  els.totals.innerHTML = `
    <div class="tu-total-card">
      <div class="tu-total-value">${fmtNum(totals.calls)}</div>
      <div class="tu-total-label">Total calls</div>
    </div>
    <div class="tu-total-card">
      <div class="tu-total-value">${fmtNum(totals.input_tokens)}</div>
      <div class="tu-total-label">Input tokens</div>
    </div>
    <div class="tu-total-card">
      <div class="tu-total-value">${fmtNum(totals.output_tokens)}</div>
      <div class="tu-total-label">Output tokens</div>
    </div>
  `;
}

function renderSummary(summary) {
  if (!summary.length) {
    els.summary.innerHTML = '';
    return;
  }
  const rows = summary.map(r => `
    <tr>
      <td>${providerChip(r.provider)}</td>
      <td class="tu-model">${r.model}</td>
      <td class="tu-num">${fmtNum(r.total_calls)}</td>
      <td class="tu-num">${fmtNum(r.total_input_tokens)}</td>
      <td class="tu-num">${fmtNum(r.total_output_tokens)}</td>
      <td class="tu-num">${fmtNum(r.total_input_tokens + r.total_output_tokens)}</td>
    </tr>
  `).join('');
  els.summary.innerHTML = `
    <table class="tu-summary-table">
      <thead>
        <tr>
          <th>Provider</th>
          <th>Model</th>
          <th class="tu-num">Calls</th>
          <th class="tu-num">Input tokens</th>
          <th class="tu-num">Output tokens</th>
          <th class="tu-num">Total tokens</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderLog(log) {
  if (!log.length) {
    els.logEmpty.classList.remove('hidden');
    return;
  }
  els.logEmpty.classList.add('hidden');
  els.logBody.innerHTML = log.map(e => `
    <tr>
      <td class="tu-time" data-tooltip="${new Date(e.called_at).toLocaleString()}">${fmtTime(e.called_at)}</td>
      <td>${providerChip(e.provider)}</td>
      <td class="tu-model">${e.model}</td>
      <td><span class="tu-op-chip">${e.operation}</span></td>
      <td class="tu-num"><span class="tu-num-val">${fmtNum(e.input_tokens)}</span></td>
      <td class="tu-num"><span class="tu-num-val">${fmtNum(e.output_tokens)}</span></td>
      <td class="tu-num"><span class="tu-num-total">${fmtNum(e.input_tokens + e.output_tokens)}</span></td>
    </tr>
  `).join('');
}

async function load() {
  const resp = await fetch('/api/token-usage');
  if (!resp.ok) return;
  const data = await resp.json();
  renderTotals(data.totals);
  renderSummary(data.summary);
  renderLog(data.log);
}

load();
