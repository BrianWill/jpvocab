// Returns true if ch is a CJK kanji character.
export function isKanji(ch) {
  const cp = ch.codePointAt(0);
  return (cp >= 0x4E00 && cp <= 0x9FFF) ||
         (cp >= 0x3400 && cp <= 0x4DBF) ||
         (cp >= 0xF900 && cp <= 0xFAFF);
}

// Returns true if every character in s is katakana.
function _isKatakana(s) {
  return s.length > 0 && /^[\u30A0-\u30FF]+$/.test(s);
}

// Returns HTML for a reading with colour-coded segments.
// Walks word characters: each kanji consumes the next entry from kanjiData (katakana
// reading → on'yomi, hiragana → kun'yomi); each kana character in the word becomes a
// plain kana segment. Falls back to plain escaped text when there is no kanji data.
export function renderReading(reading, word, kanjiData) {
  if (!reading) return '';
  if (!kanjiData || kanjiData.length === 0) return esc(reading);

  const kanjiReadings = kanjiData.map(e => e.reading);
  let kanjiIdx = 0;
  let html = '';
  let kanaBuffer = '';

  function flushKana() {
    if (!kanaBuffer) return;
    html += '<span class="reading-seg reading-kana">' + esc(kanaBuffer) + '</span>';
    kanaBuffer = '';
  }

  for (const ch of word) {
    const cp = ch.codePointAt(0);
    if (isKanji(ch)) {
      flushKana();
      if (kanjiIdx >= kanjiReadings.length) continue;
      const r = kanjiReadings[kanjiIdx++];
      const type = _isKatakana(r) ? 'on' : 'kun';
      html += '<span class="reading-seg reading-' + type + '">' + esc(r) + '</span>';
    } else if ((cp >= 0x3040 && cp <= 0x309F) || (cp >= 0x30A0 && cp <= 0x30FF)) {
      kanaBuffer += ch;
    }
  }
  flushKana();

  return html || esc(reading);
}

export function esc(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function timeAgo(dateStr) {
  const sec = Math.floor((Date.now() - new Date(dateStr)) / 1000);
  const min = Math.floor(sec / 60);
  if (min < 1) return 'just now';
  if (min < 60)   return min + ' minute' + (min === 1 ? '' : 's') + ' ago';
  const hr = Math.floor(min / 60);
  if (hr < 24)    return hr + ' hour' + (hr === 1 ? '' : 's') + ' ago';
  const day = Math.floor(hr / 24);
  if (day < 30)   return day + ' day' + (day === 1 ? '' : 's') + ' ago';
  const mo = Math.floor(day / 30);
  if (mo < 12)    return mo + ' month' + (mo === 1 ? '' : 's') + ' ago';
  const yr = Math.floor(day / 365);
  return yr + ' year' + (yr === 1 ? '' : 's') + ' ago';
}

export function detailItemPosSelect(value, typeLabels) {
  const known = value in typeLabels;
  let options = known ? '' : '<option value="" selected>—</option>';
  options += Object.entries(typeLabels).map(([key, label]) => {
    const short = label.split(' — ')[0].split(' (')[0].toUpperCase();
    return '<option value="' + esc(key) + '"' + (value === key ? ' selected' : '') + '>' + esc(short) + '</option>';
  }).join('');
  return '<span class="detail-item detail-pos">' +
    '<span class="detail-label">pos</span> ' +
    '<select class="detail-pos-select">' + options + '</select>' +
    '</span>';
}

export function detailItemKanjiReadings(word, kanjiData) {
  if (!word || !kanjiData || kanjiData.length === 0) return '';
  let kanjiIdx = 0;
  let pairs = '';
  for (const ch of word) {
    if (isKanji(ch) && kanjiIdx < kanjiData.length) {
      const entry = kanjiData[kanjiIdx++];
      pairs +=
        '<span class="kanji-reading-pair">' +
          '<span class="kanji-reading-char">' + esc(ch) + '</span>' +
          '<span class="detail-input kanji-reading-input" contenteditable="true"' +
            ' data-kanji-id="' + entry.id + '">' + esc((entry.reading || '').trim()) + '</span>' +
        '</span>';
    }
  }
  if (!pairs) return '';
  return '<span class="detail-item detail-kanji">' +
    '<span class="detail-label">kanji readings</span> ' + pairs +
    '</span>';
}

export function detailItemInput(label, value, cls) {
  return '<span class="detail-item ' + cls + '">' +
    '<span class="detail-label">' + esc(label) + '</span> ' +
    '<span class="detail-input" contenteditable="true">' + esc((value || '').trim()) + '</span>' +
    '</span>';
}

export function detailItemExInput(exJp, exEn) {
  return '<span class="detail-item detail-ex">' +
    '<span class="detail-label">example</span> ' +
    '<span class="detail-ex-inputs">' +
      '<span class="detail-ex-flag">🇯🇵</span><span class="detail-input" contenteditable="true">' + esc((exJp || '').trim()) + '</span>' +
      '<span class="detail-ex-sep">🏴󠁧󠁢󠁥󠁮󠁧󠁿</span><span class="detail-input detail-input--en" contenteditable="true">' + esc((exEn || '').trim()) + '</span>' +
    '</span>' +
    '</span>';
}

export function getSortedWords(words, key, dir) {
  const asc = dir === 'asc';
  const byDate = (a, b, field) => {
    if (!a[field] && !b[field]) return 0;
    if (!a[field]) return asc ? -1 : 1;
    if (!b[field]) return asc ? 1 : -1;
    const diff = new Date(a[field]) - new Date(b[field]);
    return asc ? diff : -diff;
  };
  return [...words].sort((a, b) => {
    switch (key) {
      case 'added':    return byDate(a, b, 'createdAt');
      case 'drilled':  return byDate(a, b, 'lastDrilled');
      case 'correct': {
        const d = asc ? a.correct - b.correct : b.correct - a.correct;
        return d || new Date(b.createdAt) - new Date(a.createdAt);
      }
      case 'incorrect': {
        const d = asc ? a.incorrect - b.incorrect : b.incorrect - a.incorrect;
        return d || new Date(b.createdAt) - new Date(a.createdAt);
      }
      case 'target': {
        const d = asc ? a.target - b.target : b.target - a.target;
        return d || new Date(b.createdAt) - new Date(a.createdAt);
      }
      case 'reading': {
        const ra = a.reading || '';
        const rb = b.reading || '';
        const d = asc ? ra.localeCompare(rb, 'ja') : rb.localeCompare(ra, 'ja');
        return d || new Date(b.createdAt) - new Date(a.createdAt);
      }
      case 'type': {
        if (a.type < b.type) return asc ? -1 : 1;
        if (a.type > b.type) return asc ? 1 : -1;
        if (!a.lastDrilled && !b.lastDrilled) return 0;
        if (!a.lastDrilled) return asc ? 1 : -1;
        if (!b.lastDrilled) return asc ? -1 : 1;
        const diff = new Date(b.lastDrilled) - new Date(a.lastDrilled);
        return asc ? diff : -diff;
      }
      default: return 0;
    }
  });
}
