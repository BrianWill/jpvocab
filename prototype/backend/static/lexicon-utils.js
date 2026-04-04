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
//
// When pitchAccent (NHK integer) is provided, pitch styling is combined into the same
// spans: each reading-seg wrapper preserves the on/kun/kana colour, and individual
// pitch-mora spans inside carry pitch-high (overline) where applicable, with a
// pitch-drop indicator inserted at the drop point.
export function renderReading(reading, word, kanjiData, pitchAccent) {
  if (!reading) return '';

  const hasPitch = pitchAccent !== null && pitchAccent !== undefined;

  if (!hasPitch) {
    // No pitch data — one span per kanji reading or kana buffer, existing behaviour.
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
        html += '<span class="reading-seg reading-' + (_isKatakana(r) ? 'on' : 'kun') + '">' + esc(r) + '</span>';
      } else if ((cp >= 0x3040 && cp <= 0x309F) || (cp >= 0x30A0 && cp <= 0x30FF)) {
        kanaBuffer += ch;
      }
    }
    flushKana();
    return html || esc(reading);
  }

  // Pitch mode: build segments [{text, type}], then render per mora inside each segment
  // wrapper so on/kun colour and pitch-high overline are combined on one set of spans.
  const allMorae = _splitMorae(reading);
  const n = allMorae.length;

  function isHigh(i) {
    if (pitchAccent === 0) return i > 0;
    if (pitchAccent === 1) return i === 0;
    return i > 0 && i < pitchAccent;
  }

  const segments = [];
  if (!kanjiData || kanjiData.length === 0) {
    segments.push({ text: reading, type: 'kana' });
  } else {
    const kanjiReadings = kanjiData.map(e => e.reading);
    let kanjiIdx = 0;
    let kanaBuffer = '';
    for (const ch of word) {
      const cp = ch.codePointAt(0);
      if (isKanji(ch)) {
        if (kanaBuffer) { segments.push({ text: kanaBuffer, type: 'kana' }); kanaBuffer = ''; }
        if (kanjiIdx < kanjiReadings.length) {
          const r = kanjiReadings[kanjiIdx++];
          segments.push({ text: r, type: _isKatakana(r) ? 'on' : 'kun' });
        }
      } else if ((cp >= 0x3040 && cp <= 0x309F) || (cp >= 0x30A0 && cp <= 0x30FF)) {
        kanaBuffer += ch;
      }
    }
    if (kanaBuffer) segments.push({ text: kanaBuffer, type: 'kana' });
  }

  let moraIdx = 0;
  let lastMoraHigh = false;
  let html = '';
  for (const seg of segments) {
    const segMorae = _splitMorae(seg.text);
    // Bridge the inter-segment gap with an overline when both sides are high-pitched.
    const connected = lastMoraHigh && segMorae.length > 0 && isHigh(moraIdx);
    let inner = '';
    for (const mora of segMorae) {
      const i = moraIdx++;
      const high = isHigh(i);
      if (high && i === 1 && pitchAccent !== 1) {
        inner += '<span class="pitch-rise"></span>';
      }
      inner += '<span class="pitch-mora' + (high ? ' pitch-high' : '') + '">' + esc(mora) + '</span>';
      if (pitchAccent > 0 && i === pitchAccent - 1 && (i < n - 1 || pitchAccent === n)) {
        inner += '<span class="pitch-drop"></span>';
      }
      lastMoraHigh = high;
    }
    const cls = 'reading-seg reading-' + seg.type + (connected ? ' pitch-connected' : '');
    html += '<span class="' + cls + '">' + inner + '</span>';
  }
  return html || esc(reading);
}

// Splits a hiragana/katakana reading string into morae.
// Small combining kana (ゃゅょゎ and katakana equivalents, plus small vowels) attach
// to the preceding mora to form a single rhythmic unit (e.g. きゃ = 1 mora).
function _splitMorae(reading) {
  const combining = new Set('ゃゅょゎャュョヮぁぃぅぇぉァィゥェォ');
  const morae = [];
  for (const ch of reading) {
    if (combining.has(ch) && morae.length > 0) {
      morae[morae.length - 1] += ch;
    } else {
      morae.push(ch);
    }
  }
  return morae;
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
      '<span class="detail-ex-flag">🇯🇵</span><button class="detail-ex-play" data-tooltip="Play sentence" tabindex="-1">▶</button><span class="detail-input" contenteditable="true">' + esc((exJp || '').trim()) + '</span>' +
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
