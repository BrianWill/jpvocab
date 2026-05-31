# jpstories Translation Validation Plan

## Phase 1: Add A Strict Agent-Sheet Validator

Status: implemented as `go run ./cmd/jpstories validate-agent-work -story <story>`.

Create a first-class validator for completed translation sheets before import.

- Add a command such as `go run ./cmd/jpstories validate-agent-work -story <story>`.
- Compare every `agent-done/*.txt` against the matching source sheet in `agent/*.txt`.
- Report all failures in one run rather than stopping at the first error.
- Validate:
  - expected file count and no extra files
  - sentence IDs, order, and metadata preserved exactly
  - English blocks unchanged
  - all expected level blocks present
  - no duplicate labels inside a sentence
  - all requested translations non-empty
  - all fences present and correctly ordered
  - no text outside known sheet structure

## Phase 2: Gate Each Subagent Batch

Status: implemented through scoped `validate-agent-work` calls, for example `go run ./cmd/jpstories validate-agent-work -story <story> <sheet.txt>`.

Validate subagent output immediately after each worker finishes.

- Run the strict sheet validator in single-file or batch mode for the worker's assigned files.
- Mark a worker result complete only after local validation passes.
- Retry failed files immediately while the failure context is fresh.
- Prefer one-file retries for any chunk that fails validation.
- Keep a simple progress table of assigned, completed, failed, retried, imported, and merged files.

## Phase 3: Improve Import Diagnostics

Status: implemented with aggregated `import-agent-work` diagnostics and `go run ./cmd/jpstories import-agent-work -story <story> -check`.

Make `import-agent-work` better at helping repair bad sheets.

- Add a check-only mode, for example `import-agent-work -story <story> -check`.
- Aggregate all sheet failures instead of returning only the first.
- Include file, sentence ID, level, and exact failure reason.
- Distinguish:
  - changed English text
  - missing block
  - empty translation
  - malformed fence
  - duplicate label
  - extra unknown content
- Keep import strict, but make diagnostics broad enough to repair in one pass.

## Phase 4: Harden Sheet Repair Scripts

Status: implemented in `repair_agent_sheets.ps1` and `repair_agent_sheets.sh`.

Expand the existing repair scripts so common generated-output defects are handled consistently.

- Keep BOM removal and smart-quote repair.
- Add detection or repair for missing closing `JPSTORIES>>>` fences before a new label/header.
- Flag duplicate labels within a sentence.
- Flag or repair mojibake in translation blocks.
- Refuse to silently accept empty requested blocks.
- Detect extra files in `agent-done/`.
- Produce a clear summary: fixed, ok, missing, invalid, extra.

## Phase 5: Strengthen Worker Instructions

Status: implemented in `SUBAGENT_SKILL.md`, the coordinator skill, and `AGENTS.md`.

Update `SUBAGENT_SKILL.md` and coordinator prompts to reduce malformed output.

- Require workers to compare expected IDs and labels from the source sheet against the completed output.
- Tell workers not to report success unless every requested block is present and non-empty.
- Include a small validation checklist in the worker prompt.
- For retries, include the precise prior failure and assign only one file.
- Use smaller batches for long or dialogue-heavy chunks.

## Phase 6: Add Reusable Repair Utilities

Status: implemented with shared `agent_sheet_tools.py` plus thin PowerShell and sh wrappers.

Avoid ad hoc repair scripts in the repo root.

- Add reusable scripts under `skills/jpstories-workitem-translator/scripts/`.
- Support single-file repair/check modes.
- Keep temporary outputs under an ignored temp directory if needed.
- Prefer source-shaped rewrites for badly malformed sheets: preserve source metadata/English and fill only translations.

## Phase 7: Reliability Policy For Subagents

Status: implemented with explicit retry/quarantine policy, repair logging, and updated coordinator and worker instructions.

Make worker failures explicit and recoverable.

- Treat stream disconnects and content-filter interruptions as incomplete outputs.
- Delete or quarantine partial generated files before retrying.
- Retry failed files one at a time.
- Escalate repeated failures to the coordinator or a stronger model.
- Record files that required repair so future runs can spot fragile patterns.

## Phase 8: End-To-End Acceptance Criteria

Status: implemented as `go run ./cmd/jpstories accept-story -story <story>`.

A story translation run is complete only when all gates pass.

- `agent-done/` has exactly one completed sheet per source sheet.
- Strict agent-sheet validation passes for all sheets.
- `import-agent-work` writes all completed JSON work items.
- `validate_workitems.ps1 -FixBom` reports all work items valid.
- `merge-work` merges the expected number of translations.
- `validate -story <story>` passes.
- `validate -complete -story <story>` passes.
