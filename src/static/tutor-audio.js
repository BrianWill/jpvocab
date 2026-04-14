import { WORD_TTS_RATE, playJapaneseText, stopCurrentPlayback } from './common.js';

export function createTutorAudio({ els, state, parseResponse }) {
  function clearPlayingBubble() {
    if (state.playingBubble) {
      state.playingBubble.classList.remove('tutor-bubble--playing');
      state.playingBubble = null;
    }
    if (state.playingTimer) {
      clearTimeout(state.playingTimer);
      state.playingTimer = null;
    }
  }

  function stopAudio() {
    clearPlayingBubble();
    stopCurrentPlayback();
  }

  async function playJp(text, bubble = null) {
    clearPlayingBubble();
    if (bubble) {
      state.playingBubble = bubble;
      bubble.classList.add('tutor-bubble--playing');
      state.playingTimer = setTimeout(clearPlayingBubble, 30000);
    }
    const onEnd = bubble ? () => {
      if (state.playingBubble === bubble) clearPlayingBubble();
    } : null;
    await playJapaneseText(text, WORD_TTS_RATE, {
      preferSynthesis: true,
      fallbackToBrowserTts: true,
      onEnd,
    });
  }

  function micButtonForLang(lang) {
    if (lang === 'en-US') return els.btnMicEn || els.btnMicLegacy;
    return els.btnMicJa || els.btnMicLegacy;
  }

  function setMicButtonsVisible(visible) {
    const display = visible ? '' : 'none';
    if (els.btnMicLegacy) els.btnMicLegacy.style.display = display;
    if (els.btnMicJa) els.btnMicJa.style.display = display;
    if (els.btnMicEn) els.btnMicEn.style.display = display;
  }

  function clearMicActiveState() {
    if (els.btnMicLegacy) els.btnMicLegacy.classList.remove('btn-tutor-mic--active');
    if (els.btnMicJa) els.btnMicJa.classList.remove('btn-tutor-mic--active');
    if (els.btnMicEn) els.btnMicEn.classList.remove('btn-tutor-mic--active');
  }

  function initMic(autoResize) {
    const SpeechRecognitionCtor = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!SpeechRecognitionCtor) {
      setMicButtonsVisible(false);
      return;
    }

    let recognition = null;
    let pendingStartLang = null;
    let baseText = '';

    function stopListening() {
      recognition?.stop();
    }

    function startListening(lang) {
      try {
        baseText = els.input.value;
        recognition = new SpeechRecognitionCtor();
        if (!recognition) throw new Error('Speech recognition instance unavailable');
        recognition.lang = lang;
        recognition.interimResults = true;
        recognition.maxAlternatives = 1;
      } catch (err) {
        recognition = null;
        pendingStartLang = null;
        state.listening = false;
        state.listeningLang = null;
        clearMicActiveState();
        console.warn('Speech recognition unavailable:', err);
        return;
      }

      if (typeof recognition.addEventListener !== 'function') {
        recognition = null;
        pendingStartLang = null;
        state.listening = false;
        state.listeningLang = null;
        clearMicActiveState();
        console.warn('Speech recognition does not support addEventListener.');
        return;
      }

      recognition.addEventListener('start', () => {
        state.listening = true;
        state.listeningLang = lang;
        clearMicActiveState();
        const btn = micButtonForLang(lang);
        if (btn) btn.classList.add('btn-tutor-mic--active');
      });

      recognition.addEventListener('result', event => {
        let interim = '';
        let final = '';
        for (const result of event.results) {
          if (result.isFinal) final += result[0].transcript;
          else interim += result[0].transcript;
        }
        els.input.value = baseText + final + interim;
        if (final) baseText += final;
        autoResize(els.input);
      });

      recognition.addEventListener('end', () => {
        state.listening = false;
        state.listeningLang = null;
        clearMicActiveState();
        recognition = null;
        if (pendingStartLang) {
          const nextLang = pendingStartLang;
          pendingStartLang = null;
          startListening(nextLang);
        }
      });

      recognition.addEventListener('error', event => {
        if (event.error !== 'aborted') console.warn('Speech recognition error:', event.error);
      });

      try {
        recognition.start();
      } catch (err) {
        recognition = null;
        pendingStartLang = null;
        state.listening = false;
        state.listeningLang = null;
        clearMicActiveState();
        console.warn('Unable to start speech recognition:', err);
      }
    }

    function toggleListening(lang) {
      if (state.listening) {
        if (state.listeningLang === lang) {
          pendingStartLang = null;
          stopListening();
        } else {
          pendingStartLang = lang;
          clearMicActiveState();
          const btn = micButtonForLang(lang);
          if (btn) btn.classList.add('btn-tutor-mic--active');
          stopListening();
        }
        return;
      }
      pendingStartLang = null;
      startListening(lang);
    }

    function bindMicButton(btn, lang) {
      if (!btn || typeof btn.addEventListener !== 'function') return;
      btn.addEventListener('click', () => toggleListening(lang));
    }

    bindMicButton(els.btnMicLegacy, 'ja-JP');
    bindMicButton(els.btnMicJa, 'ja-JP');
    bindMicButton(els.btnMicEn, 'en-US');
  }

  function handleReplayLastAssistant() {
    const last = [...state.history].reverse().find(msg => msg.role === 'assistant');
    if (!last) return;
    const resp = parseResponse(last.content);
    if (!resp.jp) return;
    const lastRow = [...els.messages.querySelectorAll('.tutor-message--assistant')].pop();
    const bubble = lastRow?.querySelector('.tutor-bubble') || null;
    playJp(resp.jp, bubble);
  }

  return {
    handleReplayLastAssistant,
    initMic,
    playJp,
    stopAudio,
  };
}
