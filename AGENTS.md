# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview


Japanese vocabulary drilling desktop app built with **Wails v2** (Go backend + web frontend). For now, assume a single user with no user login or need for security. The app has three core views:

- **Lexicon** — displays the user's full vocabulary set with word info (reading, part of speech, meaning, example sentences) and per-word correct/incorrect drill counts accumulated over all sessions.
- **Drill** — a round-based flashcard drill. Each round presents 10 words randomly drawn from the lexicon. The user marks each word as known or unknown; unknown words carry over to the next round alongside fresh picks. Drill state is transient and not persisted to the database.
- **Activity** — displays headline stats and a week-by-week calendar of recent drill activity. Each day cell shows how many words were drilled, added, and cleared. Clicking a day opens a detail modal listing the words involved.

Word data (lexicon) is stored in a SQLite database. Per-word drill counts (correct/incorrect) will also be persisted there. Drill session state (current round, remaining words, redo queue) is in-memory only.

Each word has two integers:

- "current drill count" (total number of times the word has been marked correct across all prior drills)
- "target drill count" (the number of times the word is intended to be drilled) 

When words are randomly selected from the lexicon to drill, only active words are chosen.

Incorrect answers are not penalised beyond incrementing the word's lifetime incorrect count. There is no spaced-repetition penalty, cooldown, or demotion mechanic — a wrong answer simply carries the word forward to the next round as normal.

A word tracks three timestamps:

- "creation date": date and time when the word was added to the database
- "last drill date": date and time when the word was last drilled (updated not when drill starts but when the user gives answer for the word)
- "target last reached date": date and time when the word's current drill count matched or exceeded its target drill count (for new words, this starts out null)

The frontend is being wired up to the prototype backend (`prototype/backend/`), which is the primary development target and will eventually replace the Wails app as the main application.

## Terminology

- **Lexicon** — the user's full set of vocabulary words (stored in SQLite)
- **Pool** — the working set of words for the current drill session (transient, in-memory)
- **Round** — one cycle of up to 10 words within a drill session

## Tech Stack

- **Backend:** Go 1.24, Chi router, Datastar (SSE)
- **Frontend:** Vanilla JS, Vite 3.0.7
- **Desktop:** Wails v2 (WebView2 on Windows, WKWebKit on macOS)
- **Database:** SQLite

## Commands

```bash
# --- Prototype backend (primary development target) ---

# Run with hot-reload (from prototype/backend/)
cd prototype/backend && air

# Run backend tests
cd prototype/backend && go test ./...

# --- Legacy Wails app ---

# Development (hot reload for both frontend and backend)
wails dev

# Production build (outputs to build/bin/)
wails build

# Frontend only
cd frontend && npm install && npm run dev

# Go dependencies
go mod tidy
```

The Go backend has a test suite; the frontend does not. When writing tests, only write them for Go backend code — do not write tests for frontend JS or HTML.

## Lexicon Features

- **Add words flow** — the user pastes Japanese words into an "add words" modal; the backend streams results back via SSE, adding words one by one and displaying them in the add-result modal. Implemented:
  - Words are normalised to their dictionary base form via `morphology.go` (e.g. conjugated verbs → dictionary form) to prevent duplicates across inflections
  - Duplicates (same base form already in lexicon) are silently skipped with a reason badge
  - AI auto-generates reading (hiragana), meaning (English), and example sentence (JP + EN) — see `ai.go` and the `/api/words/{id}/autofill` endpoint
  - All generated fields are editable inline in the add-result modal; changes are auto-saved on blur via `PATCH /api/words/{id}`

- **Edit words** — clicking the ✎ button on any lexicon row opens the same add-result modal with just that word, allowing the user to edit reading, part of speech, meaning, and example sentences. Changes auto-save on blur.

- **Part of speech (POS)** — the current category set (`godan-verb`, `ichidan-verb`, `noun`, `i-adjective`, `na-adjective`, `adverb`, `other`) may need revisiting: check whether the categories cover all desired word types and that AI autofill is classifying words accurately. The canonical list lives in `typeLabels` in `lexicon.js`.

- **Audio** — not yet implemented. Audio of the word and example sentence to be generated via VoiceVox (`tts-demo.html` exists as a sandbox for this).

- **Note:** `/api/words/{id}/reroll-meaning` and `/api/words/{id}/reroll-examples` may be dead code — the old edit modal that used them was removed (commit `f119e10`). Confirm before adding new callers.

## Frontend Pages

The HTML/CSS/JS frontend files live in `prototype/backend/static/` and are served by the prototype backend. They are being progressively wired up to real backend data, replacing dummy data with live responses.

- **drill.html** — the drill view
- **lexicon.html** — the lexicon/word management view
- **activity.html** — the activity/stats view
- **tts-demo.html** — sandbox page for testing VoiceVox TTS audio generation (not a production view)

### Backend prototype

`prototype/backend/` is a standalone Go module (separate `go.mod`) that runs a SQLite-backed HTTP server on port **1338**. It is the primary development target and will eventually replace the Wails app in the project root.

- **`main.go`** — entry point; opens the DB and starts the server
- **`db_schema.go`** — `initDB`, `migrate`, `resetDB`, `seedDB`, and schema-introspection helpers (`listTableInfos`, `queryTable`, etc.). No SQL appears outside the `db_*.go` files.
- **`db_words.go`** — all word and kanji database operations: insert, update, delete, list, upsert kanji.
- **`db_activity.go`** — drill session and answer recording (`createDrillSession`, `recordDrillAnswer`), plus activity stats and calendar queries.
- **`routes.go`** — `serverInit` (router setup), activity/drill/admin HTTP handlers, and `renderTemplate`. No direct DB access; handlers call functions from the `db_*.go` files.
- **`routes_words.go`** — word and kanji API handlers: GET/PATCH/DELETE words, autofill, reroll meaning/examples, GET kanji.
- **`ai.go`** — Shared AI types, prompts, few-shot examples, and provider-dispatch functions (`autoFillWord`, `rerollMeaning`, `rerollExamples`). No direct DB access.
- **`ai_anthropic.go`** — Anthropic Messages API: `callAnthropic` HTTP helper + autofill/reroll implementations.
- **`ai_openai.go`** — OpenAI Chat Completions API: `callOpenAI` HTTP helper + autofill/reroll implementations.
- **`morphology.go`** — word normalisation to dictionary base form (used in the add-words flow).
- **`templates/`** — HTML templates parsed from disk on every request (live-editable without restart); `base.html` is the shared shell, each page has its own file
- **`static/`** — HTML pages, CSS, and JS, served from disk (live-editable without restart)
- **`seed.json`** — fixture data loaded on first startup (or after a DB reset); contains `words` and `sessions` arrays

Key API endpoints (beyond CRUD on `/api/words` and `/api/kanji`):

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/providers` | Check which AI providers are configured/available |
| `PATCH` | `/api/words/{id}` | Update a word's reading, type, meaning, and example sentences |
| `PATCH` | `/api/words/{id}/target` | Update a word's target drill count |
| `POST` | `/api/words/{id}/autofill` | AI-generate reading, meaning, and examples for a word |
| `POST` | `/api/words/{id}/reroll-meaning` | Regenerate just the meaning via AI *(may be unused — see Lexicon Features note)* |
| `POST` | `/api/words/{id}/reroll-examples` | Regenerate just the example sentences via AI *(may be unused — see Lexicon Features note)* |
| `POST` | `/api/drill/sessions` | Start a new drill session |
| `POST` | `/api/drill/sessions/{id}/answers` | Record an answer within a session |

Run with hot-reload from the `backend/` directory:

```bash
cd prototype/backend && air
```

#### Database schema

> **No migration compatibility required.** During development it is fine to reset the database (`/admin` → Reset DB) whenever the schema changes. Do not spend effort on backwards-compatible migrations or backfill logic at this stage.

Table definitions live in the `migrate()` function in `db.go`. Schema is versioned via `PRAGMA user_version` — each entry in the migrations slice runs exactly once. Current tables:

- **`words`** — the lexicon; one row per word with reading, part of speech, meaning, example sentences, audio paths (`audio_word_path`, `audio_example_path`), drill counts, target, timestamps, and a `kanji_data` JSON column (array of `{id, reading}` linking to the `kanji` table). `word` column has a unique index.
- **`kanji`** — one row per kanji character with `character` and `meanings` (JSON array of English meanings). Readings (on/kun) are stored per-word in the `kanji_data` column of `words`, not on the kanji row itself. Served via `/api/kanji`.
- **`drill_sessions`** — one row per drill session with a `started_at` timestamp.
- **`drill_answers`** — one row per answer within a session; references `words` and `drill_sessions`; stores `correct` (0/1) and `answered_at`.

The admin UI at `http://localhost:1338/admin` shows live table schemas (column names, types, PK/UNIQUE/NOT NULL flags) and row counts, and links through to full table data views.

### CSS organisation

Styles shared across pages belong in `common.css`, which is loaded first by all pages. Page-specific files only contain styles unique to that page. When adding new styles, prefer extending `common.css` over duplicating rules across page stylesheets. Current shared styles include: CSS reset, `body` base, page header, nav link, `.btn-header` (the header icon button), and the full modal system.

## Working conventions

- **Scope changes to this project directory.** Do not read or write files outside `D:\code\jpvocab\` without explicit instruction.
- **Ask before touching unfamiliar files.** If a file has not been part of the current conversation and has not been recently discussed, confirm with the user before editing it. This applies especially to Go source files, config files, and anything outside `prototype/backend/`.

## Architecture (Legacy Wails app)

> This section describes the root-level Wails application. The active development target is `prototype/backend/`, which runs as a plain Go HTTP server (no Wails). The sections above cover its architecture.

The app runs as a Wails desktop window. On startup, a Go HTTP server starts on port **1337** and the frontend redirects the WebView to it.

Wails is used only as a means to serve a web interface in a dedicated window, sparing users from having to manually connect to the app via a localhost URL in their browser. By not relying on Wails's normal frontend/backend communication, the app can use Datastar with the conventional setup of an HTTP backend and browser frontend. This also leaves open the possibility of deploying as a conventional web app on a remote server (not a current goal).

- **main.go** — Wails app entry point; embeds `frontend/dist` and `hello-world.html` via Go `embed`; launches HTTP server goroutine on port 1337
- **app.go** — `App` struct bridging frontend↔backend via Wails IPC bindings
- **routes.go** — Chi router with HTTP endpoints; Datastar SSE streaming handlers
- **frontend/src/main.js** — Entry point; redirects to `localhost:1337` and exposes Wails-bound functions globally
- **frontend/wailsjs/** — Auto-generated Wails bindings (do not edit manually)

### Data flow

The project setup allows two possible ways to communicate between frontend and Go backend:

1. **Wails IPC** — typed function calls generated in `frontend/wailsjs/go/main/`
2. **SSE via Datastar** — real-time streaming updates from Go HTTP handlers to the browser

We avoid Wails IPC and use SSE via Datastar exclusively.
