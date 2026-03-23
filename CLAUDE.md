# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Japanese vocabulary drilling desktop app built with **Wails v2** (Go backend + web frontend). Currently in early stages with scaffolding and a Datastar SSE demo in place.

The vocabulary words will be tracked in a sqlite database.

## Tech Stack

- **Backend:** Go 1.24, Chi router, Datastar (SSE)
- **Frontend:** Vanilla JS, Vite 3.0.7
- **Desktop:** Wails v2 (WebView2 on Windows, WKWebKit on macOS)

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

## Architecture

The app runs as a Wails desktop window. On startup, a Go HTTP server starts on port **1337** and the frontend redirects the WebView to it.

Be clear that Wails is used only as a means to serve a web interface in a dedicated window, sparing users from having to manually connect to the app via a localhost url in their browser. By not relying on Wails's normal frontend / backend communication, the app can use data-star with the conventional setup of an http backend and browser frontend. This also leaves open the possibility of deploying the app as a conventional web app on a remote server (though this is not currently a goal of the project).

- **main.go** — Wails app entry point; embeds `frontend/dist` and `hello-world.html` via Go `embed`; launches HTTP server goroutine on port 1337
- **app.go** — `App` struct bridging frontend↔backend via Wails IPC bindings
- **routes.go** — Chi router with HTTP endpoints; Datastar SSE streaming handlers
- **frontend/src/main.js** — Entry point; redirects to `localhost:1337` and exposes Wails-bound functions globally
- **frontend/wailsjs/** — Auto-generated Wails bindings (do not edit manually)

### Data flow

The project setup allows two possible ways to communicate between Frontend and Go backend:

1. **Wails IPC** — typed function calls generated in `frontend/wailsjs/go/main/`
2. **SSE via Datastar** — real-time streaming updates from Go HTTP handlers to the browser

However, we will avoid Wails IPC and just stick to SSE via Datastar.
