# CLAUDE.md

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

The frontend is currently being refined as static HTML prototypes (`prototype/`) before being wired up to the backend.

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
# Development (hot reload for both frontend and backend)
wails dev

# Production build (outputs to build/bin/)
wails build

# Frontend only
cd frontend && npm install && npm run dev

# Go dependencies
go mod tidy

# Run backend tests (from prototype/backend/)
cd prototype/backend && go test ./...
```

The Go backend has a test suite; the frontend does not. When writing tests, only write them for Go backend code — do not write tests for frontend JS or HTML.

## Planned Lexicon Features (not yet implemented)

- **Add words flow** — the lexicon has an "add words" modal where the user pastes Japanese words (one per line). When wired to the backend:
  - Words are normalised to their dictionary base form via grammatical analysis (e.g. conjugated verbs → dictionary form) to prevent duplicates across inflections
  - Duplicates (same base form already in lexicon) are silently skipped
  - AI is used to auto-generate: reading (hiragana), meaning (English), example sentence (Japanese + English translation)
  - Optionally, audio of the word and example sentence is generated via VoiceVox and stored alongside the word

## Frontend Prototypes

`prototype/` contains standalone HTML/CSS/JS prototypes that define the UI design. They use hardcoded word data and have no backend connection yet.

- **drill.html** — the drill view
- **lexicon.html** — the lexicon/word management view
- **activity.html** — the activity/stats view

### Dummy data

All prototype dummy data lives in `dummy_data.js`, which is loaded before each page's own script. It exports:

- `lexiconWords` — word list used by the lexicon page (includes correct/incorrect/target/createdAt/lastDrilled fields)
- `drillWords` — word list used by the drill page (leaner shape, no stat fields)
- `W` — word dictionary used by the activity page (`word → [reading, meaning]`)
- `dr()` / `wr()` — helpers for building activity entries
- `activityData` — date-keyed drill/add/clear history for the activity calendar
- `stats` — headline stat numbers for the activity stats section
- `kanjiData` — maps kanji character → `{ on: [...katakana], kun: [...hiragana], meanings: [...] }`; used by the drill page to render per-kanji reading and meaning breakdowns

When adding or changing dummy data, edit `dummy_data.js` only — do not put data back into the page JS files.

### Backend prototype

`prototype/backend/` is a standalone Go module (separate `go.mod`) that runs a SQLite-backed HTTP server on port **1338**. It is developed and run independently from the Wails app.

- **`main.go`** — entry point; opens the DB and starts the server
- **`db.go`** — all database access: `initDB`, `migrate`, `seedDB`, and one function per query or write operation. No SQL appears outside this file.
- **`routes.go`** — Chi router and HTTP handlers only; no direct DB access. Handlers call functions from `db.go` and pass results to `renderTemplate`.
- **`templates/`** — HTML templates parsed from disk on every request (live-editable without restart); `base.html` is the shared shell, each page has its own file
- **`static/`** — CSS and other static assets, also served from disk
- **`seed.json`** — fixture data loaded on first startup (or after a DB reset); contains `words` and `sessions` arrays

Run with hot-reload from the `backend/` directory:

```bash
cd prototype/backend && air
```

#### Database schema

> **No migration compatibility required.** During development it is fine to reset the database (`/admin` → Reset DB) whenever the schema changes. Do not spend effort on backwards-compatible migrations or backfill logic at this stage.

Table definitions live in the `migrate()` function in `db.go`. Schema is versioned via `PRAGMA user_version` — each entry in the migrations slice runs exactly once. Current tables:

- **`words`** — the lexicon; one row per word with reading, part of speech, meaning, example sentences, audio paths (`audio_word_path`, `audio_example_path`), drill counts, target, timestamps. `word` column has a unique index.
- **`drill_sessions`** — one row per drill session with a `started_at` timestamp.
- **`drill_answers`** — one row per answer within a session; references `words` and `drill_sessions`; stores `correct` (0/1) and `answered_at`.

The admin UI at `http://localhost:1338/admin` shows live table schemas (column names, types, PK/UNIQUE/NOT NULL flags) and row counts, and links through to full table data views.

### CSS organisation

Styles shared across pages belong in `common.css`, which is loaded first by all pages. Page-specific files only contain styles unique to that page. When adding new styles, prefer extending `common.css` over duplicating rules across page stylesheets. Current shared styles include: CSS reset, `body` base, page header, nav link, `.btn-header` (the header icon button), and the full modal system.

## Working conventions

- **Scope changes to this project directory.** Do not read or write files outside `D:\code\jpvocab\` without explicit instruction.
- **Ask before touching unfamiliar files.** If a file has not been part of the current conversation and has not been recently discussed, confirm with the user before editing it. This applies especially to Go source files, config files, and anything outside `prototype/`.

## Architecture

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
