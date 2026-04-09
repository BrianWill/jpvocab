// synth-cache.js — shared LRU cache for on-demand VoiceVox audio synthesis.
//
// getSynthAudio(text, vvSettings) is the single public entry point. It fetches
// audio from /api/voicevox/synthesize on a cache miss and returns a Blob URL
// that stays valid as long as the entry lives in the cache.
//
// Cache keys include speaker, speedScale, and intonationScale so that changing
// any VoiceVox setting causes a fresh synthesis rather than serving stale audio.
// Evicted entries have their Blob URL revoked immediately to free memory.

const SYNTH_CACHE_MAX = 100;

// Map<key, blobUrl> — insertion order is the LRU order.
const _cache = new Map();

function cacheKey(text, speaker, speedScale, intonationScale) {
  return `${speaker}|${speedScale}|${intonationScale}|${text}`;
}

function cacheGet(key) {
  if (!_cache.has(key)) return null;
  // Move to end to mark as most-recently-used.
  const url = _cache.get(key);
  _cache.delete(key);
  _cache.set(key, url);
  return url;
}

function cacheSet(key, url) {
  if (_cache.size >= SYNTH_CACHE_MAX) {
    // Evict least-recently-used (first inserted / oldest) entry.
    const oldestKey = _cache.keys().next().value;
    URL.revokeObjectURL(_cache.get(oldestKey));
    _cache.delete(oldestKey);
  }
  _cache.set(key, url);
}

/**
 * Returns a Blob URL for the synthesized audio of `text` using the given
 * VoiceVox settings. Hits the cache first; synthesizes and caches on miss.
 * Throws if the synthesis request fails.
 *
 * @param {string} text
 * @param {{ speaker?: number, speedScale?: number, intonationScale?: number }} vvSettings
 * @returns {Promise<string>} Blob URL
 */
export async function getSynthAudio(text, { speaker = 1, speedScale = 1.0, intonationScale = 1.0 } = {}) {
  const key = cacheKey(text, speaker, speedScale, intonationScale);
  const cached = cacheGet(key);
  if (cached) return cached;

  const res = await fetch('/api/voicevox/synthesize', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text, speaker, speedScale, intonationScale }),
  });
  if (!res.ok) throw new Error(`synthesis failed: ${res.status}`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  cacheSet(key, url);
  return url;
}
