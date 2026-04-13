export function getFieldLanguageKind(el) {
  if (el.closest('.detail-ex')) {
    return el.classList.contains('detail-input--en') ? 'example-en' : 'example-jp';
  }
  if (el.closest('.detail-reading')) return 'reading';
  if (el.classList.contains('kanji-reading-input')) return 'kanji-reading';
  return null;
}

export function getFieldLanguageFilter(kind) {
  switch (kind) {
    case 'example-en':
      return text => text.replace(/[\u3040-\u30FF\u4E00-\u9FFF\u3400-\u4DBF\uFF01-\uFF9F]/g, '');
    case 'example-jp':
    case 'reading':
    case 'kanji-reading':
      return text => text.replace(/[a-zA-Z]/g, '');
    default:
      return null;
  }
}

export function sanitizeFieldInput(text, kind) {
  const filter = getFieldLanguageFilter(kind);
  return filter ? filter(String(text)) : String(text);
}

export function getFieldLanguageErrorMsg(kind) {
  if (kind === 'example-en') return 'English only - Japanese characters are not allowed here';
  if (kind === 'example-jp' || kind === 'reading' || kind === 'kanji-reading') {
    return 'Japanese only - Latin letters are not allowed here';
  }
  return '';
}
