import { isTtsAutoplayEnabled, playWordAudio, positionAnchoredWordTooltip } from './common.js';
import { renderReading } from './lexicon-utils.js';
import { formatRelativeTime } from './format-utils.js';
import { isMatchingRoundComplete, isSessionComplete } from './drill-state.js';

const DRILL_AUDIO_OPTIONS = { preferSynthesis: true, fallbackToBrowserTts: true };

export function createDrillElements() {
  const els = {
    actionPrompt: document.getElementById('action-prompt'),
    dontKnowBtn: document.querySelector('.btn-no'),
    filterHint: document.getElementById('filter-hint'),
    headerBegan: document.getElementById('header-began'),
    headerRestartBtn: document.querySelector('.btn-header'),
    knowBtn: document.querySelector('.btn-yes'),
    lastExampleEn: document.getElementById('last-example-en'),
    lastExampleJp: document.getElementById('last-example-jp'),
    lastKanjiInfo: document.getElementById('last-kanji-info'),
    lastMeaning: document.getElementById('last-meaning'),
    lastPos: document.getElementById('last-pos'),
    lastReading: document.getElementById('last-reading'),
    lastWordCard: document.getElementById('last-word-card'),
    lastWordImage: document.getElementById('last-word-image'),
    lastWordJp: document.getElementById('last-word-jp'),
    matchingArea: document.getElementById('matching-area'),
    matchingInfoList: document.getElementById('matching-info-list'),
    matchingNextRoundBtn: document.getElementById('matching-next-round-btn'),
    matchingNextRow: document.getElementById('matching-next-row'),
    matchingWordList: document.getElementById('matching-word-list'),
    mainArea: document.querySelector('.main-area'),
    pageBody: document.querySelector('.page-body'),
    progressBar: document.querySelector('.progress-bar'),
    promptExampleJp: document.getElementById('prompt-example-jp'),
    promptWordJp: document.getElementById('prompt-word-jp'),
    promptSection: document.querySelector('.prompt-section'),
    restartBackdrop: document.getElementById('restart-modal-backdrop'),
    restartDrillMode: document.getElementById('restart-drill-mode'),
    restartRoundSize: document.getElementById('restart-round-size'),
    restartStartBtn: document.getElementById('restart-start-btn'),
    restartTotalWords: document.getElementById('restart-total-words'),
    sidebar: document.querySelector('.sidebar'),
    sidebarList: document.getElementById('sidebar-list'),
    sidebarTitle: document.getElementById('sidebar-title'),
    statToGo: document.getElementById('stat-togo'),
    tip: document.getElementById('tooltip'),
    nextBtn: document.getElementById('drill-next-btn'),
  };

  els.restartFilterButtons = Array.from(
    els.restartBackdrop.querySelectorAll('.filter-chip[data-filter]')
  );
  els.restartCloseBtn = els.restartBackdrop.querySelector('.modal-close');
  els.restartCancelBtn = els.restartBackdrop.querySelector('.btn-cancel');

  return els;
}

export function syncRestartFilterButtons(els, activeFilters) {
  els.restartFilterButtons.forEach(btn => {
    btn.classList.toggle('active', activeFilters.has(btn.dataset.filter));
  });
}

export function updateFilterHint(els, activeFilters, filteredCount, totalCount, totalFilterCount) {
  if (activeFilters.size === 0) {
    els.filterHint.textContent = 'Select at least one word type';
    els.filterHint.classList.add('error');
    els.restartStartBtn.disabled = true;
    return;
  }

  els.filterHint.textContent = activeFilters.size === totalFilterCount
    ? 'All ' + filteredCount + ' words'
    : filteredCount + ' of ' + totalCount + ' words';
  els.filterHint.classList.remove('error');
  els.restartStartBtn.disabled = false;
}

function renderStats(els, state) {
  els.statToGo.textContent = (state.poolSize - state.doneCount) + ' to go of ' + state.poolSize;
  if (els.sidebarTitle) els.sidebarTitle.textContent = 'Round ' + state.round;
  els.headerBegan.textContent = 'began ' + formatRelativeTime(state.drillStartedAt);

  const pct = state.poolSize > 0 ? (state.doneCount / state.poolSize) * 100 : 0;
  els.progressBar.style.width = pct + '%';
}

function matchingWordStatus(state, wordId) {
  const isMatched = typeof state.matchingMatchedPairs[wordId] === 'number';
  if (state.matchingCarryoverWordIds.includes(wordId)) {
    return isMatched ? 'missed-resolved' : 'missed-pending';
  }
  if (state.matchingFirstTryCorrectWordIds.includes(wordId)) return 'known';
  if (state.matchingRedoWordIds.includes(wordId)) return 'redo';
  return 'unseen';
}

function renderMatchingDrill(els, state) {
  const matchedInfoIds = new Set(Object.values(state.matchingMatchedPairs || {}));
  const infoStatusById = new Map();
  const matchedWordByInfoId = new Map();
  Object.entries(state.matchingMatchedPairs || {}).forEach(([wordId, infoId]) => {
    const numericWordId = parseInt(wordId, 10);
    const status = matchingWordStatus(state, numericWordId);
    const matchedWord = state.matchingRoundWords.find(word => word.id === numericWordId) || null;
    infoStatusById.set(infoId, status);
    if (matchedWord) matchedWordByInfoId.set(infoId, matchedWord);
  });
  els.matchingWordList.innerHTML = '';
  els.matchingInfoList.innerHTML = '';

  state.matchingRoundWords.forEach(word => {
    const button = document.createElement('button');
    const status = matchingWordStatus(state, word.id);
    const isMatched = typeof state.matchingMatchedPairs[word.id] === 'number';
    button.type = 'button';
    button.className = 'matching-word-row matching-status-' + status;
    if (state.matchingSelectedWordId === word.id) button.classList.add('selected');
    if (isMatched) button.classList.add('locked');
    button.dataset.wordId = String(word.id);
    button.textContent = word.word;
    els.matchingWordList.appendChild(button);
  });

  state.matchingInfoWords.forEach(word => {
    const card = document.createElement('button');
    const isMatched = matchedInfoIds.has(word.id);
    const status = infoStatusById.get(word.id);
    const matchedWord = matchedWordByInfoId.get(word.id);
    card.type = 'button';
    card.className = 'matching-info-card';
    if (status) card.classList.add('matching-status-' + status);
    if (isMatched) card.classList.add('locked');
    card.disabled = isMatched;
    card.dataset.infoId = String(word.id);
    const imagePath = typeof word.imagePath === 'string' ? word.imagePath.trim() : '';
    card.innerHTML = `
      <div class="matching-info-top">
        <div class="matching-info-meaning">${word.meaning || ''}</div>
        <div class="matching-info-pos">${word.type || ''}</div>
      </div>
      ${imagePath ? `<img class="matching-info-image" src="/static/${imagePath}" alt="">` : ''}
    `;
    if (matchedWord) card.appendChild(buildMatchingReveal(matchedWord));
    els.matchingInfoList.appendChild(card);
  });
}

function buildKanjiMeaningsList(word, extraClass = '') {
  const wrap = document.createElement('div');
  wrap.className = ('matching-info-kanji ' + extraClass).trim();

  if (!Array.isArray(word.kanjiData) || word.kanjiData.length === 0) return wrap;

  word.kanjiData.forEach(entry => {
    const item = document.createElement('div');
    item.className = 'kanji-entry';

    const char = document.createElement('div');
    char.className = 'kanji-char';
    char.textContent = entry.character || '';

    const detail = document.createElement('div');
    detail.className = 'kanji-detail';

    const meanings = document.createElement('div');
    meanings.className = 'kanji-meanings';
    meanings.textContent = Array.isArray(entry.meanings) ? entry.meanings.join(', ') : '';

    detail.appendChild(meanings);
    item.appendChild(detail);
    item.appendChild(char);
    wrap.appendChild(item);
  });

  return wrap;
}

function buildMatchingReveal(word) {
  const wrap = document.createElement('div');
  wrap.className = 'matching-info-reveal';
  const readingCol = document.createElement('div');
  readingCol.className = 'matching-info-reveal-col matching-info-reveal-col-reading';
  const kanjiCol = document.createElement('div');
  kanjiCol.className = 'matching-info-reveal-col matching-info-reveal-col-kanji';

  if (word.reading) {
    const reading = document.createElement('div');
    reading.className = 'matching-info-reading';
    reading.innerHTML = renderReading(word.reading, word.word, word.kanjiData, word.pitchAccent);
    readingCol.appendChild(reading);
  }

  if (Array.isArray(word.kanjiData) && word.kanjiData.length > 0) {
    kanjiCol.appendChild(buildKanjiMeaningsList(word));
  }

  wrap.appendChild(readingCol);
  wrap.appendChild(kanjiCol);

  return wrap;
}

function renderSidebar(els, state) {
  els.sidebarList.innerHTML = '';
  state.sidebarItems.forEach(itemData => {
    const li = document.createElement('li');
    li.className = 'sidebar-item ' + itemData.status;
    const flashClass = state.sidebarFlash?.word === itemData.word.word
      ? (state.sidebarFlash.knew ? 'flash-known' : 'flash-missed')
      : '';
    if (flashClass) li.classList.add(flashClass);
    li.textContent = itemData.word.word;
    li.dataset.word = JSON.stringify(itemData.word);
    li.dataset.id = itemData.word.word;
    if (flashClass) {
      li.addEventListener('animationend', () => {
        li.classList.remove('flash-known', 'flash-missed');
        if (state.sidebarFlash?.word === itemData.word.word) state.sidebarFlash = null;
      }, { once: true });
    }
    els.sidebarList.appendChild(li);
  });
}

function renderLastAnswered(els, state) {
  if (!state.lastAnswered) {
    els.lastWordCard.style.display = 'none';
    return;
  }

  const answered = state.lastAnswered.word;
  els.lastWordCard.style.display = '';
  els.lastWordJp.textContent = answered.word;
  els.lastWordJp.className = 'tooltip-word ' + (state.lastAnswered.knew ? 'knew' : 'missed');
  els.lastReading.innerHTML = renderReading(answered.reading, answered.word, answered.kanjiData, answered.pitchAccent);
  els.lastPos.textContent = answered.type;
  els.lastMeaning.textContent = answered.meaning;
  els.lastExampleJp.textContent = answered.exampleJp;
  els.lastExampleEn.textContent = answered.exampleEn;
  els.lastKanjiInfo.innerHTML = '';
  els.lastKanjiInfo.appendChild(buildKanjiMeaningsList(answered, 'last-word-kanji-list'));
  const imagePath = typeof answered.imagePath === 'string' ? answered.imagePath.trim() : '';
  if (imagePath) {
    els.lastWordImage.src = '/static/' + imagePath;
    els.lastWordImage.style.display = '';
  } else {
    els.lastWordImage.removeAttribute('src');
    els.lastWordImage.style.display = 'none';
  }
}

function setPromptText(el, text, keyHint) {
  el.textContent = text;
  if (keyHint) {
    el.dataset.keyHint = keyHint;
    return;
  }
  delete el.dataset.keyHint;
}

function renderPrompt(els, state) {
  els.sidebarList.querySelectorAll('.sidebar-item.current').forEach(el => el.classList.remove('current'));
  els.promptSection.classList.remove('revealed-known', 'revealed-missed');
  if (isSessionComplete(state)) {
    if (state.poolSize === 0) {
      setPromptText(els.promptWordJp, 'No words to drill', '');
      setPromptText(els.promptExampleJp, 'There are no active words available with current drill settings.', '');
    } else {
      setPromptText(els.promptWordJp, 'Done!', '');
      setPromptText(els.promptExampleJp, 'All words cleared.', '');
    }
    els.actionPrompt.style.display = 'none';
    return;
  }

  els.actionPrompt.style.display = '';
  if (!state.currentWord) return;

  setPromptText(els.promptWordJp, state.currentWord.word, 'W');
  setPromptText(els.promptExampleJp, state.currentWord.exampleJp, 'S');
  els.dontKnowBtn.style.display = state.awaitingAdvance ? 'none' : '';
  els.knowBtn.style.display = state.awaitingAdvance ? 'none' : '';
  els.nextBtn.style.display = state.awaitingAdvance ? '' : 'none';
  if (state.awaitingAdvance && state.lastAnswered) {
    els.promptSection.classList.add(state.lastAnswered.knew ? 'revealed-known' : 'revealed-missed');
  }
  if (isTtsAutoplayEnabled() && state.currentWord.id !== state.lastAutoPlayedId) {
    state.lastAutoPlayedId = state.currentWord.id;
    playWordAudio(state.currentWord, 1, DRILL_AUDIO_OPTIONS);
  }
  const item = els.sidebarList.querySelector('[data-id="' + state.currentWord.word + '"]');
  if (item) item.classList.add('current');
}

export function renderDrill(els, state) {
  renderStats(els, state);
  els.pageBody.classList.toggle('matching-mode', state.matchingPairsMode);

  if (state.matchingPairsMode) {
    const sessionComplete = isSessionComplete(state);
    const roundComplete = !sessionComplete && isMatchingRoundComplete(state);
    els.sidebar.style.display = 'none';
    els.tip.classList.remove('visible');
    els.lastWordCard.style.display = 'none';
    if (sessionComplete) {
      els.matchingWordList.innerHTML = '';
      els.matchingInfoList.innerHTML = '';
    }
    els.matchingArea.classList.toggle('hidden', sessionComplete);
    els.matchingArea.style.display = sessionComplete ? 'none' : '';
    els.matchingNextRow.style.display = roundComplete ? '' : 'none';
    els.mainArea.style.display = sessionComplete ? '' : 'none';
    if (sessionComplete) {
      renderPrompt(els, state);
    } else {
      els.actionPrompt.style.display = 'none';
      renderMatchingDrill(els, state);
    }
    return;
  }

  els.matchingArea.classList.add('hidden');
  els.matchingArea.style.display = 'none';
  els.matchingNextRow.style.display = 'none';
  els.sidebar.style.display = '';
  els.mainArea.style.display = '';
  els.tip.classList.remove('visible');
  renderSidebar(els, state);
  renderLastAnswered(els, state);
  renderPrompt(els, state);
}

export function positionSidebarTooltip(els, item, tip) {
  const itemRect = item.getBoundingClientRect();
  const sidebarRect = els.sidebar.getBoundingClientRect();
  positionAnchoredWordTooltip(tip, {
    anchorRect: itemRect,
    left: sidebarRect.right - 14,
  });
}
