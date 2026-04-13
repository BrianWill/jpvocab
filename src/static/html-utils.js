export function escapeHtml(value, { escapeQuotes = true, escapeApostrophe = false } = {}) {
  const chars = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
  };
  if (escapeQuotes) chars['"'] = '&quot;';
  if (escapeApostrophe) chars["'"] = '&#39;';
  return String(value ?? '').replace(/[&<>"']/g, char => chars[char] || char);
}
