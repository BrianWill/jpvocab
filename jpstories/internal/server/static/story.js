(() => {
  const reader = document.querySelector("[data-story-reader]");
  if (!reader) return;

  const storyID = reader.dataset.storyId;
  const level = reader.dataset.level;
  const status = document.querySelector("[data-playback-status]");
  const toggleButton = document.querySelector("[data-playback-toggle]");
  const speedInput = document.querySelector("[data-playback-speed]");
  const sentenceButtons = Array.from(reader.querySelectorAll(".sentence:not(:disabled)"));
  let currentIndex = 0;
  let isPlaying = false;
  let currentAudio;
  let currentAudioURL;
  let requestController;
  let playbackGeneration = 0;

  if (sentenceButtons.length === 0) {
    toggleButton && (toggleButton.disabled = true);
    setStatus("No playable sentences for this level.", true);
    return;
  }

  function setStatus(message, isError) {
    if (!status) return;
    status.textContent = message;
    status.classList.toggle("error", Boolean(isError));
  }

  function setToggleLabel() {
    if (!toggleButton) return;
    toggleButton.textContent = isPlaying ? "Pause" : "Play";
  }

  function sentenceRow(button) {
    return button.closest(".sentence-row");
  }

  function sentenceParagraph(button) {
    return button.closest(".paragraph");
  }

  function paragraphText(button) {
    const paragraph = sentenceParagraph(button);
    if (!paragraph) return button.textContent.trim();
    return Array.from(paragraph.querySelectorAll(".sentence:not(:disabled)"))
      .map((sentence) => sentence.textContent.trim())
      .filter(Boolean)
      .join("\n");
  }

  function openParagraphInGoogleTranslate(button) {
    const text = paragraphText(button);
    if (!text) return;
    const params = new URLSearchParams({
      sl: "ja",
      tl: "en",
      text,
      op: "translate"
    });
    window.open("https://translate.google.com/?" + params.toString(), "_blank", "noopener");
  }

  function updateCurrentHighlight() {
    sentenceButtons.forEach((button, index) => {
      const row = sentenceRow(button);
      row && row.classList.toggle("is-current", index === currentIndex);
    });
  }

  function cleanupAudio() {
    if (currentAudio) {
      currentAudio.pause();
      currentAudio.removeAttribute("src");
      currentAudio = null;
    }
    if (currentAudioURL) {
      URL.revokeObjectURL(currentAudioURL);
      currentAudioURL = "";
    }
  }

  function stopCurrentRequest() {
    if (requestController) {
      requestController.abort();
      requestController = null;
    }
  }

  function readSpeed() {
    if (!speedInput) return "1";
    const value = Number.parseFloat(speedInput.value);
    if (!Number.isFinite(value)) return "1";
    return Math.min(4, Math.max(0.1, value)).toFixed(2);
  }

  function setCurrentIndex(index) {
    currentIndex = Math.min(sentenceButtons.length - 1, Math.max(0, index));
    updateCurrentHighlight();
  }

  async function fetchSentenceAudio(button, signal) {
    const sentenceID = button.dataset.sentenceId;
    if (!storyID || !level || !sentenceID || button.disabled) return null;
    const params = new URLSearchParams({
      story: storyID,
      level,
      sentence: sentenceID,
      part: button.dataset.sentencePart || "0",
      speed: readSpeed()
    });
    const response = await fetch("/api/sentence-audio?" + params.toString(), { signal });
    if (!response.ok) {
      const message = await response.text();
      throw new Error(message || "VoiceVox playback failed.");
    }
    return response.blob();
  }

  async function playCurrentSentence() {
    const generation = ++playbackGeneration;
    const button = sentenceButtons[currentIndex];
    if (!button) return;
    cleanupAudio();
    stopCurrentRequest();
    setToggleLabel();
    setStatus("Loading audio...", false);

    requestController = new AbortController();
    try {
      const blob = await fetchSentenceAudio(button, requestController.signal);
      if (!blob || generation !== playbackGeneration || !isPlaying) return;
      currentAudioURL = URL.createObjectURL(blob);
      currentAudio = new Audio(currentAudioURL);
      currentAudio.addEventListener("ended", () => {
        if (generation !== playbackGeneration || !isPlaying) return;
        if (currentIndex + 1 >= sentenceButtons.length) {
          isPlaying = false;
          cleanupAudio();
          setToggleLabel();
          setStatus("", false);
          return;
        }
        setCurrentIndex(currentIndex + 1);
        playCurrentSentence();
      }, { once: true });
      await currentAudio.play();
      setStatus("Playing audio...", false);
    } catch (err) {
      if (err.name === "AbortError") return;
      isPlaying = false;
      cleanupAudio();
      setToggleLabel();
      setStatus(err.message || "VoiceVox playback unavailable.", true);
    } finally {
      if (generation === playbackGeneration) {
        requestController = null;
      }
    }
  }

  function play() {
    if (isPlaying) return;
    isPlaying = true;
    playCurrentSentence();
  }

  function pause() {
    isPlaying = false;
    playbackGeneration++;
    stopCurrentRequest();
    cleanupAudio();
    setToggleLabel();
    setStatus("", false);
  }

  toggleButton && toggleButton.addEventListener("click", () => {
    if (isPlaying) {
      pause();
    } else {
      play();
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.code !== "Space") return;
    const tagName = event.target && event.target.tagName;
    if (tagName === "INPUT" || tagName === "SELECT" || tagName === "TEXTAREA" || event.target.isContentEditable) {
      return;
    }
    event.preventDefault();
    if (isPlaying) {
      pause();
    } else {
      play();
    }
  });

  document.addEventListener("click", (event) => {
    const button = event.target.closest(".sentence");
    if (!button) return;
    const index = sentenceButtons.indexOf(button);
    if (index < 0) return;
    if (event.altKey) {
      event.preventDefault();
      openParagraphInGoogleTranslate(button);
      return;
    }
    if (isPlaying) {
      pause();
      setCurrentIndex(index);
      play();
      return;
    }
    setCurrentIndex(index);
    play();
  });

  setCurrentIndex(0);
  setToggleLabel();
})();
