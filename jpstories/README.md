# jpstories

`jpstories` turns English stories into Japanese learner reading material. It is a local-first Go app: each story lives in its own local directory, deterministic code splits source text into stable chunks, Codex/Claude agents translate exported work items, and the web reader displays Japanese beside the original English.

The Go server does not call language model APIs. Translation agents are triggered outside the app through Codex or Claude and operate on local JSON files.

## What Works Now

- Deterministic chunking from English `.txt` files into story JSON.
- Source text cleanup for OCR/PDF copy-paste artifacts before chunking.
- Story validation for draft and complete stories.
- Export of missing translations into focused Codex/Claude work item files.
- Merge of completed work item JSON back into story JSON.
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
    done/
      my_story_chunk-001.json
```

Conventions:

- `stories/my_story/my_story.txt`: raw English source text.
- `stories/my_story/my_story.cleaned.txt`: cleaned source text written by `clean-source` and `prepare-story`.
- `stories/my_story/my_story.json`: story JSON source of truth for the reader.
- `stories/my_story/chunk/*.json`: untranslated work items exported for agents.
- `stories/my_story/done/*.json`: completed work items to merge.

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

Open the included sample story, switch between `native`, `n3`, and `n2_abridged`, and click Japanese sentence text to request VoiceVox playback.

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

Merge completed work item output back into a story:

```powershell
go run ./cmd/jpstories merge-work -story my_story
```

Validate completed work item output before merging:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\validate_workitems.ps1 -Story my_story -FixBom
```

On Unix-like shells:

```sh
sh skills/jpstories-workitem-translator/scripts/validate_workitems.sh --story my_story --fix-bom
```

The shell validator uses `python3` or `python` from `PATH`.

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

Use `-force` to overwrite the cleaned source and story JSON outputs. Use `-words-per-chunk` to tune grouped work item size. Use `-level native`, `-level n3`, or `-level n2_abridged` to export only one translation level.

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

The exporter writes small JSON files in `stories/my_story/chunk/`, one per chunk with missing translations. Each work item includes:

- story ID and title
- chunk ID
- requested missing translation levels
- paragraph and sentence IDs
- English source text
- empty sentence-level translation fields such as `native`, `n3`, or `n2_abridged`

Instruction text is kept in `AGENTS.md` and the local translator skills, not repeated inside each work item JSON file.

### 6. Translate With Codex Or Claude

Give one work item file to Codex or Claude and ask it to fill only the empty sentence-level translation fields.

Important translation-agent rules:

- preserve every ID exactly
- translate only the requested `levels`
- do not change English source text
- do not add or remove sentence-level translation fields
- write valid JSON only
- do not call translation APIs from inside this project

Move completed work item JSON files into:

```text
stories/my_story/done/
```

### 7. Merge Completed Work

Before merging, validate the completed work item files:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\validate_workitems.ps1 -Story my_story -FixBom
```

On Unix-like shells:

```sh
sh skills/jpstories-workitem-translator/scripts/validate_workitems.sh --story my_story --fix-bom
```

The shell validator uses `python3` or `python` from `PATH`.

The batch validator compares every file in `stories/my_story/done/` with its matching source work item in `stories/my_story/chunk/`. It reports valid, missing, invalid, and extra files, and `-FixBom` strips UTF-8 BOM bytes when present.

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
- translation tabs for `native`, `n3`, and `n2_abridged`
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
- `n2_abridged`

Example shape:

```json
{
  "id": "my_story",
  "title": "My Story",
  "source_language": "en",
  "target_language": "ja",
  "source_file": "stories/my_story/my_story.cleaned.txt",
  "levels": ["native", "n3", "n2_abridged"],
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
                "n2_abridged": "<shorter JLPT N2-level Japanese translation>"
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

`n2_abridged`: shorter JLPT N2-level version that preserves the story.

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

## Notes

- V1 is local-first and filesystem-backed.
- Codex/Claude translation agents operate outside the Go server.
- The Go server does not manage agent execution.
- Deterministic chunking is owned by code.
- Translation quality is owned by agents.
- VoiceVox is optional and should fail gently.
