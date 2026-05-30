(() => {
  const form = document.querySelector("[data-settings-form]");
  if (!form) return;

  const status = document.querySelector("[data-playback-status]");
  const speakerSelect = document.querySelector("[data-speaker-select]");
  const speakerName = document.querySelector("[data-speaker-name]");
  let currentAudio;

  function setStatus(message, isError) {
    if (!status) return;
    status.textContent = message;
    status.classList.toggle("error", Boolean(isError));
  }

  function syncSpeakerName() {
    if (!speakerSelect || !speakerName) return;
    const selected = speakerSelect.options[speakerSelect.selectedIndex];
    speakerName.value = selected ? selected.dataset.label || selected.textContent : "";
  }

  async function preview() {
    syncSpeakerName();
    setStatus("Loading preview...", false);
    try {
      const data = new URLSearchParams();
      data.set("voicevox_base_url", form.elements.voicevox_base_url.value);
      data.set("voicevox_speaker_id", form.elements.voicevox_speaker_id.value);
      data.set("voicevox_speed_scale", form.elements.voicevox_speed_scale.value);
      data.set("voicevox_pause_length_scale", form.elements.voicevox_pause_length_scale.value);
      data.set("voicevox_volume_scale", form.elements.voicevox_volume_scale.value);
      data.set("voicevox_pitch_scale", form.elements.voicevox_pitch_scale.value);
      data.set("voicevox_intonation_scale", form.elements.voicevox_intonation_scale.value);
      data.set("voicevox_pre_phoneme_length", form.elements.voicevox_pre_phoneme_length.value);
      data.set("voicevox_post_phoneme_length", form.elements.voicevox_post_phoneme_length.value);
      data.set("text", form.elements.preview_text.value);
      const response = await fetch("/api/voicevox-preview", {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: data.toString()
      });
      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || "VoiceVox preview failed.");
      }
      const blob = await response.blob();
      if (currentAudio) {
        currentAudio.pause();
        URL.revokeObjectURL(currentAudio.src);
      }
      const url = URL.createObjectURL(blob);
      currentAudio = new Audio(url);
      currentAudio.addEventListener("ended", () => {
        URL.revokeObjectURL(url);
        setStatus("", false);
      }, { once: true });
      await currentAudio.play();
      setStatus("Playing preview...", false);
    } catch (err) {
      setStatus(err.message || "VoiceVox preview unavailable.", true);
    }
  }

  speakerSelect && speakerSelect.addEventListener("change", syncSpeakerName);
  form.addEventListener("submit", syncSpeakerName);
  const previewButton = document.querySelector("[data-preview]");
  previewButton && previewButton.addEventListener("click", preview);
  syncSpeakerName();
})();
