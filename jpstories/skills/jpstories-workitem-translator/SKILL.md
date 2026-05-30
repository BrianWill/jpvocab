---
name: jpstories-workitem-translator
description: Prepare and translate a jpstories story by story name. Use when Codex is given a jpstories story name and asked to clean/chunk/export missing work items, translate them with subagents, validate each translation JSON, and merge completed work items into the story JSON.
---

# jpstories Story Translator

Translate a jpstories story from local disk using only the story name. Do not call model APIs, translation APIs, web services, or the jpstories Go server. Work through local files and the public `go run ./cmd/jpstories ...` commands.

The user should only need to provide a story name such as `foo_bar`. Resolve paths from that name:

- story directory: `stories/foo_bar/`
- raw source: `stories/foo_bar/foo_bar.txt`
- cleaned source: `stories/foo_bar/foo_bar.cleaned.txt`
- story JSON: `stories/foo_bar/foo_bar.json`
- outstanding work items: `stories/foo_bar/chunk/*.json`
- completed work items: `stories/foo_bar/done/`

## High-Level Workflow

1. Resolve the story name.
2. Prepare the story only for missing artifacts.
3. Discover outstanding work item JSON files.
4. Translate work items with subagents when available.
5. Ensure each subagent validates and fixes its output JSON.
6. Re-validate completed JSON files in the main agent.
7. Merge `stories/<story>/done/` into `stories/<story>/<story>.json`.
8. Validate the merged story.

## Preparation

Be idempotent. Do not overwrite existing cleaned source or story JSON unless the user explicitly asks for `-force`.

Given story name `<story>`:

1. If `stories/<story>/<story>.json` is missing, run:

```powershell
go run ./cmd/jpstories prepare-story -story <story>
```

This requires `stories/<story>/<story>.txt` to exist.

2. If the story JSON exists but no outstanding work item files are present, run:

```powershell
go run ./cmd/jpstories export-work -story <story>
```

3. If `export-work` reports no missing translations, run:

```powershell
go run ./cmd/jpstories validate -complete -story <story>
```

Then report that the story is already complete.

Do not re-run `chunk` or `prepare-story` over an existing story JSON unless the user explicitly asks to regenerate it. Stable IDs matter.

## Work Item Discovery

List candidate files in `stories/<story>/chunk/` only:

- include `*.json`
- sort by chunk number
- before assigning work, check `stories/<story>/done/` and skip files that already have valid completed output

Do not read every work item up front. Read only enough metadata to batch safely when needed.

## Batching

Work items are already grouped by chunk, so batch by file by default and let each subagent complete all requested levels in that chunk.

Default batch shape:

- long chunks: one grouped chunk file per subagent
- short chunks: 2-3 story chunks per subagent when the files are small

Keep batches sorted by chunk number. A work item may include `native`, `n3`, and `n2_abridged`; each sentence-level field controls the style rules for that translation.

Aim for about one subagent per story chunk. For example, a 30-chunk story with three outstanding levels should spawn about 30 or fewer subagents, not 90.

If subagents are available, delegate batches to fresh subagents. If subagents are not available, process one batch at a time in the current agent.

When spawning subagents, do not fork the full conversation history. The subagent prompt context should contain the worker instructions from `skills/jpstories-workitem-translator/SUBAGENT_SKILL.md` and the assigned file paths, not this coordinator skill or the broader conversation. In tool terms, leave full-history fork options disabled or unset, such as `fork_context: false`. Prefer low or minimal reasoning effort for normal translation batches unless the user explicitly asks for a higher-quality review pass.

## Subagent Requirements

Every subagent must:

1. Read only its assigned work item files.
2. Fill only existing empty sentence-level translation fields.
3. Preserve all IDs, English text, non-translation fields, object shape, and filenames.
4. Write completed JSON files to `stories/<story>/done/` with the same filenames.
5. Parse and validate each output JSON after writing it.
6. Fix any validation problems before reporting completion.
7. Report completed files, skipped files, and any unresolved validation issues.

Tell subagents they are not alone in the codebase and must not revert unrelated edits.

## Translation Rules

Use each sentence-level translation field exactly.

### native

Write natural Japanese. Preserve meaning, tone, character voice, and narrative flow. Do not simplify just because the project is for learners. Avoid translationese when a natural phrasing is available.

### n3

Write learner-friendly Japanese around JLPT N3 difficulty. Prefer common vocabulary and direct sentence structure. Avoid dense literary phrasing, uncommon compounds, and highly idiomatic expressions. Preserve important meaning even when simplifying grammar. Keep the result natural, not a word-by-word gloss.

### n2_abridged

Write a shorter JLPT N2-level version. Preserve essential events, relationships, and emotional meaning. Compress details when useful, but do not change the plot. Use natural Japanese suitable for upper-intermediate learners.

## JSON Validation For Each Work Item

For each input/output pair, validate with a JSON parser:

- output parses as JSON
- `story_id`, `story_title`, `chunk_id`, `levels`, paragraph IDs, sentence IDs, and English text match the input exactly
- every entry in `levels` is one of `native`, `n3`, or `n2_abridged`
- sentence-level translation fields are unchanged as keys
- every requested sentence-level translation value is non-empty after trimming whitespace
- no extra top-level fields were added
- no `instructions` field is added; instruction text lives in this skill and the worker skill
- no Markdown fences, commentary, notes, or metadata were written

If validation fails, fix the JSON and validate again before moving on.

## Main-Agent Validation Before Merge

After all subagents finish, the main agent must independently validate the files in `stories/<story>/done/` using the same rules above. Prefer the batch validator, which reports valid, missing, invalid, and extra files and can remove UTF-8 BOM bytes:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\validate_workitems.ps1 -Story <story> -FixBom
```

On Unix-like shells:

```sh
sh skills/jpstories-workitem-translator/scripts/validate_workitems.sh --story <story> --fix-bom
```

Use the same validator in single-file mode when repairing one output:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\validate_workitems.ps1 -InputPath <input-file> -OutputPath <done-file> -FixBom
```

On Unix-like shells:

```sh
sh skills/jpstories-workitem-translator/scripts/validate_workitems.sh --input-path <input-file> --output-path <done-file> --fix-bom
```

If a file is missing or invalid, repair it locally or send it back to a subagent before merging. If a file is extra, remove it only when the user explicitly approves or it is clearly generated by this translation run; otherwise report it and do not merge.

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

## Subagent Prompt Template

When delegating a batch, use a concise prompt like:

```text
Follow the worker instructions from skills/jpstories-workitem-translator/SUBAGENT_SKILL.md.

Complete these jpstories work item files for story <story>:

<file 1>
<file 2>

Read only those files from stories/<story>/chunk/. Fill only existing empty sentence-level translation fields (`native`, `n3`, `n2_abridged`). Preserve all IDs, English text, non-translation fields, object shape, and filenames exactly. Write completed JSON to stories/<story>/done/ with the same filenames.

After writing each file, parse and validate the JSON. Confirm non-translation fields match the input, sentence-level translation fields are unchanged and listed in `levels`, and no translation value is empty. Fix any problems before reporting completion.

You are not alone in the codebase; do not revert unrelated edits. Report completed files, skipped files, and unresolved validation issues.
```

Spawn this subagent with a standalone prompt only. Do not fork the full conversation history.

For grouped chunk files, include:

```text
The assigned files may include multiple requested levels. Use each sentence-level field exactly and follow that level's style rules.
```

## Handling Problems

- If `stories/<story>/<story>.txt` is missing and the story JSON has not been prepared, stop and ask the user to add the source text.
- If the work item contains an unsupported level, skip it and report the file.
- If a sentence is incomplete or malformed in English, translate the best recoverable meaning and preserve the source text unchanged.
- If preserving one sentence per ID produces slightly awkward segmentation, still keep one translation per ID.
- If subagents are unavailable or fail, continue locally only for batches you can validate carefully.
- If merge rejects a file, fix the completed work item JSON and rerun `merge-work`.

## Final Response

Briefly report:

- preparation commands run or skipped
- number of work items translated
- number of work items merged
- validation commands run
- any skipped or unresolved files

Do not paste full JSON into chat unless the user explicitly asks.
