# Agent Instructions

This repository is a working local-first Japanese learner story app. It has a Go command, a filesystem-backed story format, deterministic chunking, Codex/Claude work item export and merge, a browser reader, VoiceVox playback, and a settings page.

Translation agents are invoked manually or semi-manually through Codex or Claude. They are not invoked by the Go server and should not call model or translation APIs. They read local files, translate assigned work items, and normally write completed plain-text translation sheets that are imported back into JSON by the coordinator.

## Current Project Shape

- `cmd/jpstories`: CLI entrypoint for server, chunking, work export, work merge, and validation.
- `internal/story`: story JSON schema, loading, saving, listing, and validation.
- `internal/chunker`: deterministic English paragraph/sentence chunking.
- `internal/workitem`: Codex/Claude translation work item export and merge.
- `internal/server`: local web UI, settings page, and HTTP handlers.
- `internal/voicevox`: local VoiceVox HTTP client.
- `internal/appconfig`: persisted local server settings.
- `stories/<story>/`: story-scoped raw source, cleaned source, story JSON, `chunk/` work items, and `done/` completed work items.

## Commands

Use these commands as the public interface:

```powershell
go run ./cmd/jpstories
go run ./cmd/jpstories serve -config config.json
go run ./cmd/jpstories clean-source -story my_story
go run ./cmd/jpstories prepare-story -story my_story
go run ./cmd/jpstories chunk -story my_story
go run ./cmd/jpstories export-work -story my_story
go run ./cmd/jpstories export-agent-work -story my_story
go run ./cmd/jpstories validate-agent-work -story my_story
go run ./cmd/jpstories validate-agent-work -story my_story my_story_chunk-001.txt
go run ./cmd/jpstories import-agent-work -story my_story -check
go run ./cmd/jpstories import-agent-work -story my_story
go run ./cmd/jpstories accept-story -story my_story
go run ./cmd/jpstories merge-work -story my_story
go run ./cmd/jpstories validate -story my_story
go run ./cmd/jpstories validate -complete -story my_story
go test ./...
```

`config.json` is local runtime state and is ignored by Git. `config.example.json` documents the shape.

## Coding Agents

- Keep v1 local-first and filesystem-backed. Do not add a database or hosted service boundary without updating the plan and docs.
- Treat story JSON, work item JSON, imported completed JSON, and config JSON as structured data. Use JSON parsers/encoders. Translation sheets are a human/agent authoring format; import them through `import-agent-work` instead of hand-editing completed JSON.
- Preserve stable IDs. Story, chunk, paragraph, and sentence IDs are used by validation, merge, rendering, and VoiceVox playback.
- Keep VoiceVox optional. The reader, chunker, validator, exporter, merger, and settings page must remain usable when VoiceVox is unavailable.
- Keep README command examples aligned with implemented commands.
- Add or update tests for changes to schema validation, chunking, work item export/merge, web handlers, VoiceVox, or config behavior.
- Run `go test ./...` after implementation changes.

## Translation Agents

Translate prepared plain-text sheet files from local disk when available. JSON work items live in `stories/<story>/chunk/`, translator sheets live in `stories/<story>/agent/`, completed sheets should be written or moved to `stories/<story>/agent-done/`, and `import-agent-work` converts them into completed JSON under `stories/<story>/done/`. Do not split full stories yourself unless explicitly asked to repair source chunking. Do not change story structure, IDs, English source text, unrelated translation levels, or unrelated sentences.

When given a translation sheet:

1. Read the assigned local `.txt` sheet.
2. Build a quick expected-ID and expected-label checklist from the source sheet.
3. Translate only the requested `levels`.
4. Fill only existing empty translation blocks such as `native`, `n3`, and `n3_abridged`.
5. Preserve metadata, sentence IDs, English text, block labels, fences, and sheet order exactly.
6. Re-read the output and verify every requested block is present exactly once and non-empty before reporting success.
7. Write the completed plain-text sheet only. Do not include extra Markdown fences or commentary.

The merge command rejects unknown sentence IDs, unsupported levels, mismatched story IDs, extra fields, and empty translations.

After each translation worker finishes, gate only the assigned completed sheets before marking that worker complete:

```powershell
go run ./cmd/jpstories validate-agent-work -story my_story my_story_chunk-001.txt
```

For failed batches, retry the failed sheet files one at a time while the validation output is fresh.
Include the exact failure lines in the retry prompt and do not reassign neighboring files that already passed.
Treat interrupted worker streams, content-filter stops, missing final reports, or partially written files as incomplete output, not success. Before retrying a known-bad completed sheet, quarantine it so the next worker starts from the clean source sheet instead of editing corrupted output:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\repair_agent_sheets.ps1 -Story my_story -File my_story_chunk-001.txt -QuarantineInvalid
```

The repair helper records fixed, invalid, missing, and extra sheet events in `stories/my_story/agent-repair-log.jsonl`. Repeated failures on the same file should be retried one at a time and escalated to the coordinator or a stronger model.

Before writing imported completed JSON, dry-run the import diagnostics:

```powershell
go run ./cmd/jpstories import-agent-work -story my_story -check
```

When a completed sheet needs mechanical repair before import, use the reusable helper under `skills/jpstories-workitem-translator/scripts/` rather than creating one-off scripts in the repo root:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\repair_agent_sheets.ps1 -Story my_story -File my_story_chunk-001.txt -Check
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\repair_agent_sheets.ps1 -Story my_story -File my_story_chunk-001.txt -RewriteFromSource
```

`-RewriteFromSource` rebuilds the completed sheet from the original `agent/` sheet, preserving metadata, sentence IDs, English text, labels, fences, and order while filling only salvaged translations.

Before merging completed work items, validate all `done/` files against their matching `chunk/` source files:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\validate_workitems.ps1 -Story my_story -FixBom
```

On Unix-like shells:

```sh
sh skills/jpstories-workitem-translator/scripts/validate_workitems.sh --story my_story --fix-bom
```

The batch validator reports valid, missing, invalid, and extra files. Reassign or repair missing/invalid outputs before running `merge-work`.

For the final completion gate, prefer the executable acceptance command:

```powershell
go run ./cmd/jpstories accept-story -story my_story
```

It requires exact `agent-done/` coverage, strict sheet validation, successful import, completed JSON validation, expected merge counts, `validate -story`, and `validate -complete`. Do not report a translation run complete until this command passes.

## Translation Levels

### `native`

- Write natural Japanese.
- Preserve meaning, tone, and narrative flow.
- Do not simplify just because the project is for learners.
- Avoid translationese when a natural Japanese phrasing is available.

### `n3`

- Write learner-friendly Japanese around JLPT N3 difficulty.
- Prefer common vocabulary and direct sentence structure.
- Avoid dense literary phrasing, uncommon compounds, and highly idiomatic expressions.
- Preserve important meaning even when simplifying grammar.
- Keep the result natural, not a word-by-word gloss.

### `n3_abridged`

- Write a shorter JLPT N3-level version.
- Preserve essential events, relationships, and emotional meaning.
- Compress details when useful, but do not change the plot.
- Use natural Japanese suitable for intermediate learners.

## Story JSON Rules

- `stories/<story>/<story>.json` is the reader source of truth.
- Required levels are exactly:
  - `native`
  - `n3`
  - `n3_abridged`
- Draft stories may have empty translation maps.
- Complete stories should pass:

```powershell
go run ./cmd/jpstories validate -complete -story my_story
```

- Do not add extra translation-level keys unless the schema and tests are intentionally changed.
- Do not add notes/commentary fields unless the schema and docs are intentionally changed.

## VoiceVox And Settings

- Sentence playback resolves text by story ID, selected level, and sentence ID on the server.
- VoiceVox defaults to `http://127.0.0.1:50021`.
- `/settings` lets the user edit VoiceVox URL, list voices, preview a sample sentence, and save speaker settings.
- Saved settings are written to `config.json`; local CLI flags can override them for a run.
- VoiceVox errors should be visible and non-fatal.

## Documentation

- Update `README.md` when commands, config behavior, workflow, or setup changes.
- Update `plan.md` when project phases or intended interfaces change.
- If a future version adds API-driven model calls, document it as a deliberate architecture change rather than silently changing the v1 Codex/Claude local-file workflow.
