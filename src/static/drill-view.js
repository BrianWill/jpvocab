import { isTtsAutoplayEnabled, playWordAudio, positionAnchoredWordTooltip, renderWordTooltipKanji } from './common.js';
import { renderReading } from './lexicon-utils.js';
import { isSessionComplete } from './drill-state.js';

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
    progressBar: document.querySelector('.progress-bar'),
    promptExampleJp: document.getElementById('prompt-example-jp'),
    promptWordJp: document.getElementById('prompt-word-jp'),
    promptSection: document.querySelector('.prompt-section'),
    restartBackdrop: document.getElementById('restart-modal-backdrop'),
    restartRoundSize: document.getElementById('restart-round-size'),
    restartSkipAnswerReveal: document.getElementById('restart-skip-answer-reveal'),
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

function timeAgo(date) {
  const sec = Math.floor((Date.now() - date) / 1000);
  const min = Math.floor(sec / 60);
  if (min < 1) return 'just now';
  if (min < 60) return min + ' minute' + (min === 1 ? '' : 's') + ' ago';
  const hr = Math.floor(min / 60);
  if (hr < 24) return hr + ' hour' + (hr === 1 ? '' : 's') + ' ago';
  const day = Math.floor(hr / 24);
  return day + ' day' + (day === 1 ? '' : 's') + ' ago';
}

function renderStats(els, state) {
  els.statToGo.textContent = (state.poolSize - state.doneCount) + ' to go of ' + state.poolSize;
  els.sidebarTitle.textContent = 'Round ' + state.round;
  els.headerBegan.textContent = 'began ' + timeAgo(state.drillStartedAt);

  const pct = state.poolSize > 0 ? (state.doneCount / state.poolSize) * 100 : 0;
  els.progressBar.style.width = pct + '%';
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
  renderWordTooltipKanji(els.lastKanjiInfo, answered);
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
  renderSidebar(els, state);
  renderLastAnswered(els, state);
  renderStats(els, state);
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
