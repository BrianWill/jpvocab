# Drill Refactor Plan

## Goals

- Reduce the mental overhead of `backend/static/drill.js`.
- Make drill state transitions easier to reason about.
- Separate durable session state from render-only UI concerns.
- Remove duplicated helpers that already exist elsewhere.

## Current Status
The current drill frontend structure is:

- `backend/static/drill.js`
  Bootstrap, event wiring, restart-modal orchestration, and tooltip wiring only.

- `backend/static/drill-state.js`
  State creation, filtering, round building, reveal transitions, completion checks, session serialization, and drill API helpers.

- `backend/static/drill-view.js`
  DOM lookup, filter-chip syncing, filter-hint rendering, tooltip positioning, and drill rendering helpers.

The persisted drill session snapshot now excludes derived completion flags and copied settings-only values.

## Specific Cleanup Targets

- Consider moving the local `shuffle()` helper into shared utilities if another page ends up needing the same behavior.

## Shared Logic Opportunities

- Consider moving `timeAgo()` into a shared utility module if other pages will need the same formatting.
