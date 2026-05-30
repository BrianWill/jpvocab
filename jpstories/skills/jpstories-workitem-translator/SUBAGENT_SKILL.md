# jpstories Subagent Work Item Translator

Your task is to add Japanese translations to assigned jpstories work item JSON files.

Each input file is a grouped chunk from `stories/<story>/chunk/`. It contains English sentences and one or more empty sentence-level translation fields such as `native`, `n3`, or `n2_abridged`. Fill those empty Japanese translation fields, then write the completed JSON file to the assigned output directory, normally `stories/<story>/done/`, using the same filename.

Work only from the assigned input JSON files and this skill. Write only the completed output JSON files and any validation-driven fixes to those outputs.

## Assignment Inputs

Your prompt should provide:

- one or more input work item JSON file paths, normally from `stories/<story>/chunk/`
- an output directory, normally `stories/<story>/done/`
- optionally, a note about which story or chunk is being assigned

Read only the assigned input files. Write completed JSON files to the output directory with the same filenames.

## Hard Rules

- Fill only existing empty sentence-level translation fields such as `native`, `n3`, and `n2_abridged`.
- Preserve `story_id`, `story_title`, `chunk_id`, `levels`, `paragraphs`, all IDs, and all English text exactly.
- Keep the same sentence-level translation fields as the input file.
- Do not add, remove, rename, reorder, or rewrite paragraphs or sentences.
- Do not add notes, commentary, Markdown fences, metadata, or extra JSON fields.
- Do not leave any translation value empty.
- Save valid pretty-printed JSON only.

## Translation Levels

Use each sentence-level translation field exactly.

### native

Write natural Japanese. Preserve meaning, tone, character voice, and narrative flow. Do not simplify just because the project is for learners. Avoid translationese when a natural phrasing is available.

### n3

Write learner-friendly Japanese around JLPT N3 difficulty. Prefer common vocabulary and direct sentence structure. Avoid dense literary phrasing, uncommon compounds, and highly idiomatic expressions. Preserve important meaning even when simplifying grammar. Keep the result natural, not a word-by-word gloss.

### n2_abridged

Write a shorter JLPT N2-level version. Preserve essential events, relationships, and emotional meaning. Compress details when useful, but do not change the plot. Use natural Japanese suitable for upper-intermediate learners.

## Workflow

For each assigned file:

1. Parse the input JSON.
2. Translate only existing empty sentence-level translation fields.
3. Write the completed JSON to the output directory with the same filename.
4. Validate the output against the input with:

```powershell
powershell -ExecutionPolicy Bypass -File skills\jpstories-workitem-translator\scripts\validate_workitems.ps1 -InputPath <input-file> -OutputPath <output-file> -FixBom
```

On Unix-like shells:

```sh
sh skills/jpstories-workitem-translator/scripts/validate_workitems.sh --input-path <input-file> --output-path <output-file> --fix-bom
```

5. If validation fails, fix the output JSON and run the validator again.

The `-FixBom` flag removes UTF-8 BOM bytes from the input or output before parsing. This is allowed because BOM bytes can make otherwise-valid JSON fail downstream.

When a file contains multiple requested levels, translate all assigned levels in one pass over the source context. Do not copy a translation from one level into another without adapting it to that level's rules.

## Validation Requirements

The output is acceptable only when:

- it parses as JSON
- there are no UTF-8 BOM bytes after validation with `-FixBom`
- top-level fields are unchanged
- `story_id`, `story_title`, `chunk_id`, `levels`, paragraph IDs, sentence IDs, and English text match the input
- sentence-level translation field names are unchanged
- every sentence-level translation field is listed in `levels`
- every translation value is non-empty after trimming whitespace

## Final Report

Briefly report completed files, skipped files, and unresolved validation issues. Do not paste full JSON into chat.
