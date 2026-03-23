# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Japanese vocabulary drilling desktop app built with **Wails v2** (Go backend + web frontend). The app has two core views:

- **Lexicon** — displays the user's full vocabulary set with word info (reading, part of speech, meaning, example sentences) and per-word correct/incorrect drill counts accumulated over all sessions.
- **Drill** — a round-based flashcard drill. Each round presents 10 words randomly drawn from the lexicon. The user marks each word as known or unknown; unknown words carry over to the next round alongside fresh picks. Drill state is transient and not persisted to the database.

Word data (lexicon) is stored in a SQLite database. Per-word drill counts (correct/incorrect) will also be persisted there. Drill session state (current round, remaining words, redo queue) is in-memory only.

The frontend is currently being refined as static HTML prototypes (`frontend/html/`) before being wired up to the backend.

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
```

There are no test suites or linting configurations set up.

## Frontend Prototypes

`frontend/html/` contains standalone HTML/CSS/JS prototypes that define the UI design. They use hardcoded word data and have no backend connection yet.

- **drill-compact.html** — the drill view
- **lexicon.html** — the lexicon/word management view

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
