
  1. Clause splitting

  Japanese has reliable splitting points: 、 (reading comma), and conjunctive endings like 〜て/で, 〜が, 〜けど, 〜から, 〜ので,
  〜ながら. A simple rule — split at 、 first, then at long conjunctive junctions if a chunk still exceeds ~15 chars — would keep
  most chunks under 400ms without needing a full parser.

  The tricky part is audio continuity: you'd be playing 2-4 separate audio clips per sentence, so there'd be tiny gaps between
  chunks unless you either:
  - Pre-stitch the WAV bytes server-side before returning (easy since VoiceVox outputs standard PCM WAV — you can concatenate data
  sections with a single merged header), or
  - Queue them client-side as a chain of Audio objects with ended event handoff (introduces ~10-20ms gap per chunk from Audio
  startup overhead)

  Stitching server-side is cleaner.

  ---
  2. Prefetch pipeline

  While sentence N is playing, fire off generation for N+1 (and optionally N+2). This is the most impactful of the three — after
  the first sentence, the user never waits again as long as generation stays faster than playback. For typical story sentences
  (~1-3 seconds of audio, ~400-800ms generation), the margin is comfortable.

  The JS state machine needs three slots: playing, ready (generated, waiting), and generating. When playing finishes, swap ready →
  playing and kick off the next generating.

  ★ Insight ─────────────────────────────────────
  This is the classic double-buffer pattern from realtime graphics and audio streaming: you display from one buffer while filling
  the other. The constraint is the same — fill time must be less than drain time. Here, VoiceVox synthesis time (~500ms avg) <
  typical sentence playback time (~1500ms avg), so the buffer almost always stays filled.
  ─────────────────────────────────────────────────

  ---
  3. JS audio cache

  A simple Map keyed on sentence text, with a max-size LRU eviction, storing the audio as a Blob URL. The cache survives
  play/pause/scrub within the session. 100-entry cap at ~30-80 KB per sentence WAV (or ~10-20 KB OGG) = 1-8 MB total, which is
  fine.

  One detail: cache keys should be (text + speakerId) if the speaker can change. And Blob URLs should be revoked on eviction
  (URL.revokeObjectURL) to release memory.

  ---
  Putting them together, the flow becomes:

  User starts story playback at sentence 3:
    → request chunks for sentence 3 (may split into 2-3 clauses)
    → stitch + play sentence 3 chunks
    → [during playback] request sentence 4 in background
    → sentence 3 ends → sentence 4 is already ready → play immediately
    → [during playback] request sentence 5...
    → user seeks back to sentence 3 → cache hit, instant

  Want me to implement this? It would touch:
  - A new /api/voicevox/synthesize endpoint (clause-split + stitch, return single WAV or OGG)
  - story.js — prefetch state machine + cache