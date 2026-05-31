# jpstories

`jpstories` turns English stories into Japanese learner reading material. It is a local-first Go app: each story lives in its own local directory, deterministic code splits source text into stable chunks, Codex/Claude agents translate exported work items, and the web reader displays Japanese beside the original English.

The Go server does not call language model APIs. Translation agents are triggered outside the app through Codex or Claude and normally operate on local plain-text translation sheets imported back into JSON by the coordinator.

## What Works Now

- Deterministic chunking from English `.txt` files into story JSON.
- Source text cleanup for OCR/PDF copy-paste artifacts before chunking.
- Story validation for draft and complete stories.
- Export of missing translations into focused Codex/Claude work item files and plain-text translation sheets.
- Import of completed sheets into work item JSON, then merge back into story JSON.
- Local web reader with story list, translation-level tabs, and side-by-side Japanese/English text.
- Clickable Japanese sentence playback through local VoiceVox.
- Settings page for VoiceVox URL, voice/style selection, preview, and persisted config.
- Sample story data under `stories/sample-station/`.

## Story Directory Layout

Each story has one directory under `stories/`. The directory name is the story name used by CLI commands:

```text
stories/
  my_story/
    my_story.txt
    my_story.cleaned.txt
    my_story.json
    chunk/
      my_story_chunk-001.json
    agent/
      my_story_chunk-001.txt
    agent-done/
      my_story_chunk-001.txt
    done/
      my_story_chunk-001.json
```

Conventions:

- `stories/my_story/my_story.txt`: raw English source text.
- `stories/my_story/my_story.cleaned.txt`: cleaned source text written by `clean-source` and `prepare-story`.
- `stories/my_story/my_story.json`: story JSON source of truth for the reader.
- `stories/my_story/chunk/*.json`: untranslated work items exported for agents.
- `stories/my_story/agent/*.txt`: translator-friendly plain-text sheets generated from JSON work items.
- `stories/my_story/agent-done/*.txt`: completed plain-text sheets returned by translation agents.
- `stories/my_story/done/*.json`: completed work item JSON imported from sheets and ready to merge.

## Requirements

- Go 1.24 or newer.
- Optional: VoiceVox running locally for audio playback.

VoiceVox usually runs at:

```text
http://127.0.0.1:50021
```

The app still works as a reader if VoiceVox is not running.

## Quick Start

Run the sample reader:

```powershell
go run ./cmd/jpstories
```

Open:

```text
http://127.0.0.1:8080
```

Open the included sample story, switch between `native`, `n3`, and `n3_abridged`, and click Japanese sentence text to request VoiceVox playback.

Run tests:

```powershell
go test ./...
```

## Commands

Start the server with default settings:

```powershell
go run ./cmd/jpstories
```

Start the server with an explicit config file:

```powershell
go run ./cmd/jpstories serve -config config.json
```

Override runtime settings for one run:

```powershell
go run ./cmd/jpstories serve -addr 127.0.0.1:8080 -stories stories -voicevox http://127.0.0.1:50021 -voicevox-speaker 1
```

Clean raw source text before chunking:

```powershell
go run ./cmd/jpstories clean-source -story my_story
```

Clean, chunk, and export work items in one run:

```powershell
go run ./cmd/jpstories prepare-story -story my_story
```

Create draft story JSON from source text:

```powershell
go run ./cmd/jpstories chunk -story my_story
```

Export missing translations as Codex/Claude work items:

```powershell
go run ./cmd/jpstories export-work -story my_story
```

Export a specific level or chunk:

```powershell
go run ./cmd/jpstories export-work -story my_story -level n3
go run ./cmd/jpstories export-work -story my_story -chunk chunk-001
```

Convert exported JSON work items into translator-friendly text sheets:

```powershell
go run ./cmd/jpstories export-agent-work -story my_story
```

After agents complete sheets in `stories/my_story/agent-done/`, validate them against the original sheets:

```powershell
go run ./cmd/jpstories validate-agent-work -story my_story
```

The validator reports missing or extra sheets, changed metadata or English text, malformed fences, duplicate labels, missing blocks, unknown content, and empty requested translations.

To gate a just-finished worker batch, pass only the assigned sheet names. This ignores unrelated pending sheets and exits non-zero if the batch is not ready:

```powershell
go run ./cmd/jpstories validate-agent-work -story my_story my_story_chunk-001.txt my_story_chunk-002.txt
```

Then import them back into completed JSON:

```powershell
go run ./cmd/jpstories import-agent-work -story my_story -check
```

Check mode validates every completed sheet against its source JSON work item and reports all import failures without writing `done/*.json`.

Before importing, the repair helper can fix BOMs, restore smart quotes in English blocks, repair missing closing fences where the next sheet label/header is clear, and report missing, invalid, or extra completed sheets:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story my_story
```

Check or repair only a just-failed sheet:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story my_story -file my_story_chunk-001.txt -check
```

For badly malformed output, rebuild the completed sheet from the original `agent/` sheet shape and salvage only translation text:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story my_story -file my_story_chunk-001.txt -rewrite-from-source
```

Explicit source/done sheet paths are also supported:

```powershell
go run ./cmd/jpstories repair-agent-sheets -source-sheet stories\my_story\agent\my_story_chunk-001.txt -done-sheet stories\my_story\agent-done\my_story_chunk-001.txt -rewrite-from-source
```

If a worker disconnects, is content-filtered, or leaves a partial completed sheet, treat that output as incomplete. Quarantine the invalid completed file before retrying the sheet with a fresh worker:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story my_story -file my_story_chunk-001.txt -quarantine-invalid
```

The helper writes a JSONL repair log at `stories/my_story/agent-repair-log.jsonl` for fixed, invalid, missing, and extra sheet events. Use that log to spot files that repeatedly need repair or escalation.

```powershell
go run ./cmd/jpstories import-agent-work -story my_story
```

Merge completed work item output back into a story:

```powershell
go run ./cmd/jpstories merge-work -story my_story
```

Run the full end-to-end acceptance gate after all agent sheets are complete:

```powershell
go run ./cmd/jpstories accept-story -story my_story
```

This validates exact `agent/` to `agent-done/` sheet coverage, checks and imports completed sheets into `done/`, validates completed work item JSON, merges the expected number of translations, and requires both draft and complete story validation to pass.

If the completed sheets only need mechanical repair before the strict gate, run:

```powershell
go run ./cmd/jpstories accept-story -story my_story -repair-agent-sheets
```

Validate completed work item output before merging:

```powershell
go run ./cmd/jpstories validate-workitems -story my_story -fix-bom
```

The Go validator compares every file in `stories/my_story/done/` with its matching source work item in `stories/my_story/chunk/`.

Validate story JSON:

```powershell
go run ./cmd/jpstories validate -story my_story
```

Require every configured translation level to be present:

```powershell
go run ./cmd/jpstories validate -complete -story my_story
```

## Full Story Workflow

### 1. Add English Source Text

Create a story directory and raw source file:

```text
stories/my_story/my_story.txt
```

Separate paragraphs with blank lines. Wrapped lines inside a paragraph are normalized by the chunker.

### 2. Prepare Story In One Command

For messy copied source text, the shortest path is:

```powershell
go run ./cmd/jpstories prepare-story -story my_story
```

This command reads `stories/my_story/my_story.txt` and writes:

- `stories/my_story/my_story.cleaned.txt`
- `stories/my_story/my_story.json`
- missing work item JSON files in `stories/my_story/chunk/`

Use `-force` to overwrite the cleaned source and story JSON outputs. Use `-words-per-chunk` to tune grouped work item size. Use `-level native`, `-level n3`, or `-level n3_abridged` to export only one translation level.

### 3. Clean Raw Source Text Manually When Needed

If a source file came from OCR, PDF extraction, or copy/paste, run:

```powershell
go run ./cmd/jpstories clean-source -story my_story
```

The cleaner writes `stories/my_story/my_story.cleaned.txt` before story IDs are generated. It can:

- repair common mojibake and ligature artifacts
- join line-wrapped hyphenated words
- unwrap PDF-width lines
- infer likely paragraph breaks around dialogue

Paragraph modes:

- `preserve`: only existing blank lines create paragraph breaks
- `conservative`: existing blank lines plus clear new dialogue turns
- `dialogue`: also separates likely prose after dialogue; this is the default for messy copied fiction

Use `-force` to overwrite an existing cleaned output file:

```powershell
go run ./cmd/jpstories clean-source -story my_story -paragraph-mode dialogue -force
```

After cleaning, skim the cleaned file and use it as the input to `chunk`.

### 4. Chunk The Story

Run:

```powershell
go run ./cmd/jpstories chunk -story my_story
```

The chunker reads `my_story.cleaned.txt` when present, otherwise `my_story.txt`. It writes `my_story.json`, stable IDs such as `chunk-001`, `p-001`, and `s-001`, and empty translation maps. It refuses to overwrite existing output unless `-force` is supplied.

By default, new stories target about 220 English source words per chunk while preserving paragraph boundaries. Use `-id`, `-title`, `-words-per-chunk`, or `-paragraphs-per-chunk` when needed:

```powershell
go run ./cmd/jpstories chunk -story my_story -title "My Story" -words-per-chunk 180
go run ./cmd/jpstories chunk -story my_story -paragraphs-per-chunk 2
```

### 5. Export Work Items

Run:

```powershell
go run ./cmd/jpstories export-work -story my_story
```

The exporter writes small canonical JSON files in `stories/my_story/chunk/`, one per chunk with missing translations. Each work item includes:

- story ID and title
- chunk ID
- requested missing translation levels
- paragraph and sentence IDs
- English source text
- empty sentence-level translation fields such as `native`, `n3`, or `n3_abridged`

Instruction text is kept in `AGENTS.md` and the local translator skills, not repeated inside each work item JSON file.

For normal agent translation, convert those JSON work items into plain-text sheets:

```powershell
go run ./cmd/jpstories export-agent-work -story my_story
```

The sheet files in `stories/my_story/agent/` contain metadata, sentence IDs, English source text, and empty translation blocks. They are easier for subagents to edit than JSON. The original JSON files remain the source of truth for validation and import.

### 6. Translate With Codex Or Claude

Give one text sheet from `stories/my_story/agent/` to Codex or Claude and ask it to fill only the empty translation blocks.

Important translation-agent rules:

- preserve every ID exactly
- translate only the requested `levels`
- do not change English source text
- do not add or remove translation block labels
- write the completed plain-text sheet, not JSON
- do not call translation APIs from inside this project

Move completed sheet files into:

```text
stories/my_story/agent-done/
```

Then import them back into completed JSON:

```powershell
go run ./cmd/jpstories import-agent-work -story my_story
```

The importer compares each completed sheet with its matching source JSON work item and writes completed JSON into:

```text
stories/my_story/done/
```

### 7. Merge Completed Work

Before merging, validate the completed work item files:

```powershell
go run ./cmd/jpstories validate-workitems -story my_story -fix-bom
```

The batch validator compares every file in `stories/my_story/done/` with its matching source work item in `stories/my_story/chunk/`. It reports valid, missing, invalid, and extra files, and `-fix-bom` strips UTF-8 BOM bytes when present.

Run:

```powershell
go run ./cmd/jpstories merge-work -story my_story
```

The merge command reads `stories/my_story/done/` and rejects malformed output, unknown sentence IDs, mismatched story IDs, unsupported levels, and empty translations.

### 8. Validate

For draft or partially translated stories:

```powershell
go run ./cmd/jpstories validate -story my_story
```

For completed stories:

```powershell
go run ./cmd/jpstories validate -complete -story my_story
```

Or run the whole final gate in one command:

```powershell
go run ./cmd/jpstories accept-story -story my_story
```

To allow the final gate to repair mechanical completed-sheet issues first:

```powershell
go run ./cmd/jpstories accept-story -story my_story -repair-agent-sheets
```

### 9. Read In The Browser

Start the server:

```powershell
go run ./cmd/jpstories
```

Then open:

```text
http://127.0.0.1:8080
```

The reader shows:

- a story directory
- Japanese on the left
- English on the right
- translation tabs for `native`, `n3`, and `n3_abridged`
- clickable Japanese sentences for VoiceVox playback

## VoiceVox Settings

Open:

```text
http://127.0.0.1:8080/settings
```

The settings page can edit the VoiceVox base URL, list available speakers/styles, tune playback options, preview a short Japanese sample sentence, and save selected voice settings.

Saved settings are written to `config.json`, which is ignored by Git because it is local machine state. `config.example.json` shows the shape.

Command-line flags override config values for that server run. The settings page still treats the saved config file as persistent local state.

## Story JSON

Story JSON is the source of truth for the reader. It contains stable IDs, original English text, and translations keyed by supported level.

Supported translation levels are exactly:

- `native`
- `n3`
- `n3_abridged`

Example shape:

```json
{
  "id": "my_story",
  "title": "My Story",
  "source_language": "en",
  "target_language": "ja",
  "source_file": "stories/my_story/my_story.cleaned.txt",
  "levels": ["native", "n3", "n3_abridged"],
  "chunks": [
    {
      "id": "chunk-001",
      "paragraphs": [
        {
          "id": "p-001",
          "sentences": [
            {
              "id": "s-001",
              "english": "The first paragraph appears here.",
              "translations": {
                "native": "<natural Japanese translation>",
                "n3": "<JLPT N3-level Japanese translation>",
                "n3_abridged": "<shorter JLPT N3-level Japanese translation>"
              }
            }
          ]
        }
      ]
    }
  ]
}
```

IDs must remain stable. The web UI, work item export, merge tooling, validation, and VoiceVox playback all depend on them.

## Translation Levels

`native`: natural full Japanese translation.

`n3`: learner-friendly JLPT N3-level translation.

`n3_abridged`: shorter JLPT N3-level version that preserves the story.

See `AGENTS.md` for detailed translation-agent guidance.

## Development

Run all tests:

```powershell
go test ./...
```

Useful checks:

```powershell
go run ./cmd/jpstories validate -complete -story sample-station
go run ./cmd/jpstories export-work -story sample-station
```

The sample story is already complete, so exporting it should report no missing translations.

Validate completed agent sheets before importing:

```powershell
go run ./cmd/jpstories validate-agent-work -story my_story
```

## Notes

- V1 is local-first and filesystem-backed.
- Codex/Claude translation agents operate outside the Go server.
- The Go server does not manage agent execution.
- Deterministic chunking is owned by code.
- Translation quality is owned by agents.
- VoiceVox is optional and should fail gently.
