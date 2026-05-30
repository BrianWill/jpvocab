# Agent Instructions

This repository is a working local-first Japanese learner story app. It has a Go command, a filesystem-backed story format, deterministic chunking, Codex/Claude work item export and merge, a browser reader, VoiceVox playback, and a settings page.

Translation agents are invoked manually or semi-manually through Codex or Claude. They are not invoked by the Go server and should not call model or translation APIs. They read local files, translate assigned work items, and write local JSON output.

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
go run ./cmd/jpstories merge-work -story my_story
go run ./cmd/jpstories validate -story my_story
go run ./cmd/jpstories validate -complete -story my_story
go test ./...
```

`config.json` is local runtime state and is ignored by Git. `config.example.json` documents the shape.

## Coding Agents

- Keep v1 local-first and filesystem-backed. Do not add a database or hosted service boundary without updating the plan and docs.
- Treat story JSON, work item JSON, and config JSON as structured data. Use JSON parsers/encoders.
- Preserve stable IDs. Story, chunk, paragraph, and sentence IDs are used by validation, merge, rendering, and VoiceVox playback.
- Keep VoiceVox optional. The reader, chunker, validator, exporter, merger, and settings page must remain usable when VoiceVox is unavailable.
- Keep README command examples aligned with implemented commands.
- Add or update tests for changes to schema validation, chunking, work item export/merge, web handlers, VoiceVox, or config behavior.
- Run `go test ./...` after implementation changes.

## Translation Agents

Translate prepared work item JSON files from local disk. Work items live in `stories/<story>/chunk/`; completed files should be written or moved to `stories/<story>/done/`. Do not split full stories yourself unless explicitly asked to repair source chunking. Do not change story structure, IDs, English source text, unrelated translation levels, or unrelated sentences.

When given a work item:

1. Read the assigned local JSON file.
2. Translate only the requested `levels`.
3. Fill only existing empty sentence-level translation fields such as `native`, `n3`, and `n2_abridged`.
4. Preserve `story_id`, `chunk_id`, `levels`, `paragraphs`, sentence IDs, English text, and JSON shape exactly.
5. Write valid JSON only. Do not include Markdown fences or commentary in generated JSON files.

The merge command rejects unknown sentence IDs, unsupported levels, mismatched story IDs, extra fields, and empty translations.

Before merging completed work items, validate all `done/` files against their matching `chunk/` source files:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\validate_workitems.ps1 -Story my_story -FixBom
```

On Unix-like shells:

```sh
sh skills/jpstories-workitem-translator/scripts/validate_workitems.sh --story my_story --fix-bom
```

The batch validator reports valid, missing, invalid, and extra files. Reassign or repair missing/invalid outputs before running `merge-work`.

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

### `n2_abridged`

- Write a shorter JLPT N2-level version.
- Preserve essential events, relationships, and emotional meaning.
- Compress details when useful, but do not change the plot.
- Use natural Japanese suitable for upper-intermediate learners.

## Story JSON Rules

- `stories/<story>/<story>.json` is the reader source of truth.
- Required levels are exactly:
  - `native`
  - `n3`
  - `n2_abridged`
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
