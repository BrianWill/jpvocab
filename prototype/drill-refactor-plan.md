# Drill Refactor Plan

## Goals

- Reduce the mental overhead of `backend/static/drill.js`.
- Make drill state transitions easier to reason about.
- Separate durable session state from render-only UI concerns.
- Remove duplicated helpers that already exist elsewhere.

## Recommended Refactor Sequence

1. Gather all DOM nodes once into an `els` object. Done.
   This removes repeated `document.getElementById(...)` and `querySelector(...)` calls and makes render code easier to scan.

2. Replace module-level globals with a single `state` object. Done.
   Use nested sections like `state.session`, `state.settings`, and `state.ui` if helpful, but keep one clear source of truth.

3. Introduce a single `renderDrill(state)` entrypoint. Done.
   Have it call small render helpers for prompt, stats, sidebar, last-answered card, and completion state.

4. Extract pure drill progression logic into a small state module or pure helper section. Done in-file for now.
   Functions like round building, answer application, completion checks, and restart/reset logic should not touch the DOM.

5. Trim persisted session shape to durable state only. Done.
   Persist semantic state, not transient render details or animation classes.

## Current Status

Steps 1 through 5 have now been implemented.

The current `backend/static/drill.js` structure is:

- `els` for cached DOM lookups
- `state` for module-level drill state
- Pure helper functions for sidebar updates, round construction, reveal transitions, and completion checks
- `renderDrill()` as the main rendering entrypoint

The persisted drill session snapshot now excludes derived completion flags and copied settings-only values.

## Specific Cleanup Targets

- Remove `maxPoolSize` from runtime state entirely if it remains only a temporary clamp value. Done.
- Keep `sidebarItems` semantic and move flash classes fully into render-only behavior if a later cleanup pass touches sidebar rendering again. Done.
- Consider moving the in-file pure drill helpers into a separate `drill-state.js` module once the shape feels stable.

## Shared Logic Opportunities

- Reuse or extract the numeric stepper helpers duplicated in `backend/static/drill.js` and `backend/static/common.js`.
- Move shared filter constants to one place.
- Consider moving `timeAgo()` into a shared utility module if other pages will need the same formatting.

## Proposed File Split

- `drill-data.js`
  Fetch and persistence helpers for words, kanji, settings, and session snapshots.

- `drill-state.js`
  Pure state creation and transitions like `createInitialState`, `restoreState`, `answerCurrentWord`, `restartDrill`, and `isComplete`.

- `drill-view.js`
  DOM rendering helpers for prompt, sidebar, stats, tooltip, and last answered card.

- `drill.js`
  Bootstrap, event wiring, and orchestration only.

## First Increment

Start with the lowest-risk cleanup:

1. Centralize DOM references in an `els` object. Done.
2. Replace scattered globals with a single `state` object. Done.

Those two changes should improve readability immediately without changing behavior.
