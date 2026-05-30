# jpstories Optimization Plan

This plan improves translation speed, token efficiency, and reliability while keeping v1 local-first and filesystem-backed.

## Phase 1: Lower Subagent Overhead

Status: implemented.

Depends on: none.

- Update the coordinator skill to batch more work per subagent by default.
- Default batch shape:
  - long chunks: one story chunk per subagent, all available levels for that chunk
  - short chunks: 2-3 story chunks per subagent when file sizes are small
- Keep subagent prompts standalone and concise.
- Never fork full conversation history into translation subagents.
- Prefer lower reasoning effort for subagents unless the user explicitly asks for higher quality review.
- Keep the worker skill as the compact source of subagent rules.

Success criteria:

- A 30-chunk, 3-level story should spawn about 30 or fewer subagents, not 90.
- Re-running the skill skips already valid files in `done/`.

## Phase 2: Batch Validation And Resume Safety

Status: implemented.

Depends on: Phase 1 can happen independently, but this phase should land before grouped work items.

- Add a batch validator command or script that validates all completed work items for a story:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\validate_workitems.ps1 -Story my_story -FixBom
```

- The batch validator should:
  - compare each `done/` file against its source file in `chunk/`
  - strip UTF-8 BOMs when `-FixBom` is set
  - verify unchanged non-translation fields
  - verify unchanged sentence-level translation fields
  - reject empty translations
  - report valid, missing, invalid, and extra files
- Update the main skill to validate before merge and reassign only missing or invalid outputs.

Success criteria:

- Main agent can validate all completed chunk outputs with one command.
- Failed or interrupted runs can resume without redoing valid completed files.

## Phase 3: Remove Repeated Instruction Payload

Status: implemented.

Depends on: Phase 2 recommended.

- Stop exporting repeated `instructions` arrays inside every work item JSON.
- Treat the main skill and worker skill as the instruction source of truth.
- Update validators to reject extra `instructions` fields in current work items.
- Update merge logic and tests to emit work items without `instructions`.
- Update docs and examples.

Success criteria:

- Work item files contain only story/chunk/level metadata, source sentences, and translations.
- Old work items with `instructions` are not part of the current format.

## Phase 4: Group Levels By Chunk

Status: implemented.

Depends on: Phase 2 required. Phase 3 recommended.

- Replace per-level work item files with one grouped file per chunk:

```text
stories/my_story/chunk/my_story_chunk-001.json
```

- Grouped work item shape:
  - one `story_id`
  - one `story_title`
  - one `chunk_id`
  - `levels`: requested missing levels for that chunk
  - `paragraphs`: sentence IDs and English source text
  - requested translation fields directly on each sentence object

Example:

```json
{
  "story_id": "my_story",
  "story_title": "My Story",
  "chunk_id": "chunk-001",
  "levels": ["native", "n3", "n2_abridged"],
  "paragraphs": [
    {
      "id": "p-001",
      "sentences": [
        {
          "id": "s-001",
          "english": "Hello.",
          "native": "",
          "n3": "",
          "n2_abridged": ""
        }
      ]
    }
  ]
}
```

- Update export, merge, validators, tests, README, AGENTS, and skills.
- Old per-level work items are not part of the current format.

Success criteria:

- A 30-chunk, 3-level story exports about 30 work item files instead of 90.
- Subagents read the English source once and fill all requested levels for a chunk.
- Merge accepts grouped completed files and updates all included levels.

## Phase 5: Tune Chunk Size For Translation Throughput

Status: implemented.

Depends on: Phase 4 recommended.

- Default new chunking now targets source word count for translation throughput, while preserving paragraph boundaries.
- `-words-per-chunk` controls the default grouped work item size.
- `-paragraphs-per-chunk` remains available as an explicit fixed-paragraph override.
- Existing stories are not a compatibility constraint; regenerate intentionally when new chunk boundaries are desired.

Success criteria:

- New stories produce fewer, more balanced work items.
- Existing stories remain valid and readable without forced regeneration.

## Phase 6: Optional Translation QA Pass

Depends on: Phase 4 recommended.

- Add an optional review mode after merge.
- Review mode should sample or scan completed translations for:
  - untranslated English
  - obvious level mismatch
  - empty or duplicate-looking translations
  - malformed punctuation or JSON artifacts
- Keep this opt-in so the default path stays fast.

Success criteria:

- Users can request extra quality assurance without slowing every run.
- QA reports actionable issues without rewriting good translations unnecessarily.

## Recommended Order

1. Phase 1: immediate speed win with skill-only changes.
2. Phase 2: reliability foundation and resumability.
3. Phase 3: smaller files and less repeated token payload.
4. Phase 4: largest structural win, reducing work item count by about 3x.
5. Phase 5: tune future story exports after grouped chunks exist.
6. Phase 6: optional quality layer once the fast path is stable.
