---
name: jpstories-workitem-translator
description: Prepare and translate a jpstories story by story name. Use when an agent is given a jpstories story name and asked to clean/chunk/export missing work items, convert them to translation sheets, translate them with subagents, import sheets back into JSON, validate, and merge completed work items into the story JSON.
---

# jpstories Story Translator

Translate a jpstories story from local disk using only the story name. Do not call model APIs, translation APIs, or web services. Work through local files and the public `go run ./cmd/jpstories ...` commands.

The user should only need to provide a story name such as `foo_bar`. Resolve paths from that name:

- story directory: `stories/foo_bar/`
- raw source: `stories/foo_bar/foo_bar.txt`
- cleaned source: `stories/foo_bar/foo_bar.cleaned.txt`
- story JSON: `stories/foo_bar/foo_bar.json`
- source JSON work items: `stories/foo_bar/chunk/*.json`
- plain-text work sheets: `stories/foo_bar/agent/*.txt`
- completed plain-text sheets: `stories/foo_bar/agent-done/*.txt`
- imported completed JSON work items: `stories/foo_bar/done/*.json`

## High-Level Workflow

1. Resolve the story name.
2. Normalize the source file path if needed.
3. Prepare the story only for missing artifacts.
4. Export missing canonical JSON work items to `chunk/`.
5. Convert JSON work items to plain-text sheets in `agent/`.
6. Translate sheets with subagents when available.
7. Gate each completed worker batch with `validate-agent-work` before marking it complete.
8. Import completed sheets from `agent-done/` into completed JSON in `done/`.
9. Re-validate completed JSON files in the main agent.
10. Merge `stories/<story>/done/` into `stories/<story>/<story>.json`.
11. Run the final `accept-story` gate or the equivalent individual checks.

## Pre-flight: Source Path Normalization

Before running `prepare-story`, check whether the story source is in the expected location. If `stories/<story>/<story>.txt` does not exist but a file named `stories/<story>` exists, create the directory and move the file:

```powershell
New-Item -ItemType Directory -Path "stories\<story>_dir" -Force
Copy-Item "stories\<story>" "stories\<story>_dir\<story>.txt"
Remove-Item "stories\<story>"
Rename-Item "stories\<story>_dir" "<story>"
```

On Unix-like shells:

```sh
mkdir -p stories/<story>_tmp
mv stories/<story> stories/<story>_tmp/<story>.txt
mv stories/<story>_tmp stories/<story>
```

## Preparation

Be idempotent. Do not overwrite existing cleaned source or story JSON unless the user explicitly asks for `-force`.

Given story name `<story>`:

1. If `stories/<story>/<story>.json` is missing, run:

```powershell
go run ./cmd/jpstories prepare-story -story <story>
```

This requires `stories/<story>/<story>.txt` to exist.

2. If the story JSON exists but no outstanding JSON work item files are present, run:

```powershell
go run ./cmd/jpstories export-work -story <story>
```

3. Convert JSON work items to translation sheets:

```powershell
go run ./cmd/jpstories export-agent-work -story <story>
```

4. If `export-work` reports no missing translations, run:

```powershell
go run ./cmd/jpstories validate -complete -story <story>
```

Then report that the story is already complete.

Do not re-run `chunk` or `prepare-story` over an existing story JSON unless the user explicitly asks to regenerate it. Stable IDs matter.

## Work Sheet Discovery

List candidate files in `stories/<story>/agent/`:

- include `*.txt`
- sort by chunk number
- before assigning work, check `stories/<story>/agent-done/` and skip sheets that already have completed output

Do not read every sheet up front. Read only enough metadata to batch safely when needed.

## Batching

Work sheets are already grouped by chunk, so batch by file by default and let each subagent complete all requested levels in that chunk.

Default batch shape:

- long chunks: one sheet file per subagent
- short chunks: 2-3 sheet files per subagent when the files are small
- dialogue-heavy chunks: one sheet file per subagent
- retry batches: one failed sheet file per subagent

Keep batches sorted by chunk number. A sheet may include `native`, `n3`, and `n3_abridged`; each block label controls the style rules for that translation.

Aim for about one subagent per story chunk. For example, a 30-chunk story with three outstanding levels should spawn about 30 or fewer subagents, not 90.

If subagents are available, delegate batches to fresh subagents. If subagents are not available, process one batch at a time in the current agent.

When spawning subagents, pass only the worker instructions from `skills/jpstories-workitem-translator/SUBAGENT_SKILL.md` and the assigned sheet file paths. Do not forward this coordinator skill or the broader conversation history.

After each subagent reports completion, immediately validate only the assigned sheet filenames:

```powershell
go run ./cmd/jpstories validate-agent-work -story <story> <sheet-1.txt> <sheet-2.txt>
```

The command prints a per-file status table. Treat the worker batch as complete only if this command exits successfully. If it fails, retry the failed files immediately while the validation output is fresh; prefer one-file retries for malformed or incomplete sheets. Track each file as assigned, completed, failed, retried, imported, and merged.

For retries, include only the failed file and the exact validation failure lines. Do not reassign neighboring files that already passed.

Treat a subagent stream disconnect, content-filter interruption, missing final report, or partially written completed sheet as incomplete output. Do not mark the file complete from a partial transcript. Retry failed or interrupted files one at a time, and quarantine invalid completed sheets before retrying when they may confuse the next worker:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story <story> -file <sheet.txt> -quarantine-invalid
```

The repair helper records fixed, invalid, missing, and extra sheet events in `stories/<story>/agent-repair-log.jsonl`. Check this log when a file repeatedly fails; after two failed one-file retries, escalate that file to the coordinator or a stronger model with the exact failure lines and the repair-log history.

## Subagent Model Selection

Translation sheets are numerous, structured, and independently imported/validated, so prefer cheaper, faster subagent models when the host tool supports model selection. For Claude, use a Haiku-class model by default unless the user asks for a stronger model. For other providers, choose the fastest low-cost model that can reliably translate English prose to Japanese and follow sheet-format constraints.

When the user launches this skill, they may specify a subagent model preference such as:

```text
Use Haiku for subagents.
Use the fastest cheap model for subagents.
Use a stronger model for native-level literary translation.
```

If the user gives no preference, default to cheap/fast subagents and rely on import validation plus spot checks in the coordinator. Escalate a specific chunk to a stronger model only when a subagent repeatedly fails import/validation or the source prose clearly needs more nuance than the default model is providing.

## Subagent Requirements

Every subagent must:

1. Read only its assigned sheet files.
2. Build a private expected-ID and expected-label checklist from each source sheet before editing.
3. Fill only existing empty translation blocks.
4. Preserve metadata, IDs, English text, block labels, fences, order, and filenames.
5. Write completed sheet files to `stories/<story>/agent-done/` with the same filenames.
6. Re-read each output sheet and compare it against the source checklist.
7. Confirm every requested translation block is non-empty before reporting success.
8. Fix any sheet-format problems before reporting completion.
9. Report completed files, skipped files, and any unresolved issues.

Tell subagents they are not alone in the codebase and must not revert unrelated edits.

## Translation Rules

Use each translation block label exactly.

### native

Write natural Japanese. Preserve meaning, tone, character voice, and narrative flow. Do not simplify just because the project is for learners. Avoid translationese when a natural phrasing is available.

### n3

Write learner-friendly Japanese around JLPT N3 difficulty. Prefer common vocabulary and direct sentence structure. Avoid dense literary phrasing, uncommon compounds, and highly idiomatic expressions. Preserve important meaning even when simplifying grammar. Keep the result natural, not a word-by-word gloss.

### n3_abridged

Write a shorter JLPT N3-level version. Preserve essential events, relationships, and emotional meaning. Compress details when useful, but do not change the plot. Use natural Japanese suitable for early intermediate learners.

## Import And Validation

After each subagent batch finishes, validate only that batch before assigning it complete:

```powershell
go run ./cmd/jpstories validate-agent-work -story <story> <sheet-1.txt> <sheet-2.txt>
```

The batch gate ignores unrelated pending sheets and reports failures for the assigned files. Retry failed files one at a time and re-run the same command until it passes.

After all subagents finish, repair known subagent encoding defects (UTF-8 BOM and smart-quote corruption in english blocks) before importing:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story <story>
```

The Go repair command inserts an obviously missing `JPSTORIES>>>` before the next known sheet label/header, detects duplicate labels, empty requested blocks, suspicious mojibake in translation blocks, missing completed sheets, and extra completed sheets. Treat `invalid`, `missing`, or `extra` results as blockers before import.

Use single-file check mode for a failed worker result:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story <story> -file <sheet.txt> -check
```

If a completed sheet is badly malformed, prefer a source-shaped rewrite. This rebuilds the output from `stories/<story>/agent/<sheet.txt>`, preserving source metadata, sentence IDs, English text, block labels, fences, and order, then fills only translations salvaged from the malformed completed sheet:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story <story> -file <sheet.txt> -rewrite-from-source
```

Explicit source/done sheet paths are also supported:

```powershell
go run ./cmd/jpstories repair-agent-sheets -source-sheet stories\<story>\agent\<sheet.txt> -done-sheet stories\<story>\agent-done\<sheet.txt> -rewrite-from-source
```

If the file is too partial or malformed to salvage safely, quarantine it and retry from the original source sheet:

```powershell
go run ./cmd/jpstories repair-agent-sheets -story <story> -file <sheet.txt> -quarantine-invalid
```

Then import completed sheets into canonical completed JSON:

```powershell
go run ./cmd/jpstories import-agent-work -story <story> -check
```

Check mode reports all sheet import diagnostics without writing JSON. Repair any reported sheet failures before importing for real.

```powershell
go run ./cmd/jpstories import-agent-work -story <story>
```

The importer compares each sheet against the matching source JSON work item in `chunk/` and rejects changed metadata, changed English text, missing translations, extra translation blocks, or malformed sheet structure.

Then independently validate the imported JSON files in `stories/<story>/done/` using the batch validator:

```powershell
go run ./cmd/jpstories validate-workitems -story <story> -fix-bom
```

Use the same validator in single-file mode when repairing one imported JSON output:

```powershell
go run ./cmd/jpstories validate-workitems -input-path <input-file> -output-path <done-file> -fix-bom
```

If import fails, repair the completed sheet and rerun `import-agent-work`. If JSON validation fails after import, repair the sheet when possible, re-import, and validate again. If a file is extra, remove it only when the user explicitly approves or it is clearly generated by this translation run; otherwise report it and do not merge.

Then merge:

```powershell
go run ./cmd/jpstories merge-work -story <story>
```

After merging, validate the story:

```powershell
go run ./cmd/jpstories validate -story <story>
```

If all work items for all configured levels were completed, also run:

```powershell
go run ./cmd/jpstories validate -complete -story <story>
```

For final acceptance, prefer the single executable gate:

```powershell
go run ./cmd/jpstories accept-story -story <story>
```

If the remaining known defects are mechanical sheet-format problems, run the acceptance gate with its explicit repair pass:

```powershell
go run ./cmd/jpstories accept-story -story <story> -repair-agent-sheets
```

This command validates exact `agent/` to `agent-done/` coverage, checks and imports completed sheets, validates `done/*.json` against `chunk/*.json` with BOM repair enabled by default, merges completed work, checks the expected file and translation counts, and requires both normal and complete story validation to pass. Treat the translation run as incomplete until this command succeeds.

## Subagent Prompt Template

When delegating a batch, use a concise prompt like:

```text
Follow the worker instructions from skills/jpstories-workitem-translator/SUBAGENT_SKILL.md.

Use the configured cheap/fast subagent model for this translation batch unless the coordinator explicitly assigned a different model.

Complete these jpstories translation sheet files for story <story>:

<file 1>
<file 2>

Read only those files from stories/<story>/agent/. Fill only existing empty translation blocks (`native`, `n3`, `n3_abridged`). Preserve metadata, IDs, English text, block labels, fences, order, and filenames exactly. Write completed sheets to stories/<story>/agent-done/ with the same filenames.

After writing each file, re-read it and check that requested translation blocks are non-empty and the sheet structure is intact. Fix any problems before reporting completion.

Before reporting success, compare the output against the source sheet: metadata unchanged, sentence headers unchanged and in order, English unchanged, requested labels present exactly once, no extra labels, all requested translations non-empty, and all fences present.

You are not alone in the codebase; do not revert unrelated edits. Report completed files, skipped files, and unresolved issues.
```

Spawn each subagent with a standalone prompt only. Do not forward the full conversation history.

For grouped chunk files, include:

```text
The assigned sheets may include multiple requested levels. Use each translation block label exactly and follow that level's style rules.
```

For retrying a failed sheet, use a one-file prompt like:

```text
Follow the worker instructions from skills/jpstories-workitem-translator/SUBAGENT_SKILL.md.

Repair only this completed jpstories sheet:

stories/<story>/agent-done/<sheet.txt>

Compare it against its source sheet:

stories/<story>/agent/<sheet.txt>

Previous validation failure:

<paste exact validate-agent-work or import-agent-work -check failure lines>

Previous repair-log entries, if any:

<paste relevant stories/<story>/agent-repair-log.jsonl lines for this file>

Do not rewrite neighboring files. Preserve metadata, sentence headers, English blocks, labels, fences, and order exactly. Fill or repair only the requested translation blocks needed to make this one file pass validation. Re-read the output and do not report success until every requested block is present exactly once and non-empty. If your stream is interrupted or you cannot finish the whole file, report it as incomplete.
```

## Handling Problems

- If `stories/<story>/<story>.txt` is missing and the story JSON has not been prepared, stop and ask the user to add the source text.
- If a sheet contains an unsupported level, skip it and report the file.
- Treat interrupted worker output as incomplete even if a partial file exists.
- Quarantine invalid partial files before a clean retry when repair is not clearly safe.
- Retry failed files one at a time; after two failed one-file retries, escalate the file with exact failure lines.
- If a sentence is incomplete or malformed in English, translate the best recoverable meaning and preserve the source text unchanged.
- If preserving one translation per sentence ID produces slightly awkward segmentation, still keep one translation per ID.
- If subagents are unavailable or fail, continue locally only for batches you can validate carefully.
- If `import-agent-work` rejects a sheet, repair the completed sheet and rerun import.
- If `merge-work` rejects a file, repair the completed sheet or imported JSON and rerun `import-agent-work`/`merge-work`.

## Final Response

Briefly report:

- preparation commands run or skipped
- number of work sheets translated
- number of sheets imported into JSON
- number of work items merged
- validation commands run
- any skipped or unresolved files

If `accept-story` was run, include whether it passed and the number of accepted sheets/work items/translations from its output.

Do not paste full sheets or JSON into chat unless the user explicitly asks.
