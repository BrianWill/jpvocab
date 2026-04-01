# Drill Refactor Plan

## Goals

- Reduce the mental overhead of `backend/static/drill.js`.
- Make drill state transitions easier to reason about.
- Separate durable session state from render-only UI concerns.
- Remove duplicated helpers that already exist elsewhere.

## Recommended Refactor Sequence

1. Gather all DOM nodes once into an `els` object.
   This removes repeated `document.getElementById(...)` and `querySelector(...)` calls and makes render code easier to scan.

2. Replace module-level globals with a single `state` object.
   Use nested sections like `state.session`, `state.settings`, and `state.ui` if helpful, but keep one clear source of truth.

3. Introduce a single `renderDrill(state)` entrypoint.
   Have it call small render helpers for prompt, stats, sidebar, last-answered card, and completion state.

4. Extract pure drill progression logic into a small state module or pure helper section.
   Functions like round building, answer application, completion checks, and restart/reset logic should not touch the DOM.

5. Trim persisted session shape to durable state only.
   Persist semantic state, not transient render details or animation classes.

## Specific Cleanup Targets

- Remove `maxPoolSize` from durable session state if it remains only a temporary clamp value.
- Remove persisted `completed` if it can be derived from the rest of the session state.
- Revisit whether `settingsMaxWords` belongs in saved drill session state instead of being read from settings on boot.
- Keep `sidebarItems` semantic and add flash classes only during rendering.

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

1. Centralize DOM references in an `els` object.
2. Replace scattered globals with a single `state` object.

Those two changes should improve readability immediately without changing behavior.
