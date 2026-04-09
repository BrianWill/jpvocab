# DEVS

This file is the developer-facing companion to `README.md`. It collects project structure, implementation details, commands, and maintenance notes.

## Stack

- Go 1.25
- Chi router
- SQLite via `modernc.org/sqlite`
- Vanilla JS + HTML/CSS
- Wails v3 alpha for the desktop shell
- Kagome for Japanese tokenization and morphology

## Project Layout

- `src/`: Go module, server, DB code, templates, static assets, tests
- `src/static/`: frontend JS, CSS, HTML, browser-side utility tests
- `src/templates/`: Go HTML templates
- `src/seed.json`: initial seed data

## Run Commands

Run desktop app:

```bash
cd src
go run .
```

Run server only:

```bash
cd src
go run . --server-only
```

Run with hot reload:

```bash
cd src
air
```

Default local URLs:

- App: `http://localhost:49200/welcome`
- Admin UI: `http://localhost:49200/admin`

## Tests

Backend tests:

```bash
cd src
go test ./...
```

Frontend utility tests:

```bash
node --test src/static/tests/*.test.js
```

AI integration tests make real API calls and should only be run intentionally:

```bash
cd src
go test -tags integration ./...
```

## Optional Integrations

AI provider env vars:

- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`
- `GOOGLE_API_KEY`
- `MISTRAL_API_KEY`
- `GLM_API_KEY`

VoiceVox:

- local endpoint: `http://localhost:50021`
- used for generated word and story audio

## Product Model

The app is local-first and single-user. There is no login or multi-user security model.

Top-level views:

- `Lexicon`
- `Drill`
- `Activity`
- `Stories`
- `Tutor`

Important behavior:

- words have a current drill count and target drill count
- correct answers increase progress
- incorrect answers only affect lifetime incorrect stats
- only active words are selected for drilling
- the current drill session is persisted in SQLite and restored after reload/restart
- words are normalized to dictionary form when added

## Main Features

Lexicon:

- stores readings, part of speech, meanings, examples, images, audio flags, and drill stats
- supports AI autofill and inline edits
- supports local image upload

Drill:

- round-based flashcards
- default round size is 10
- missed words carry forward to the next round

Stories:

- stores title plus sentence/token data rather than one raw text blob
- supports noted words, translation, generated audio, and add-all-to-lexicon flow

Tutor:

- AI chat endpoint with multiple prompt modes

## Persistence

SQLite DB file:

- `src/jpvocab.db`

Core tables:

- `words`
- `kanji`
- `drill_sessions`
- `drill_answers`
- `user_settings`
- `stories`
- `story_sentences`
- `tutor_prompts`

The app seeds initial data when starting from an empty database.

## Main Endpoints

- `GET /api/providers`
- `GET /api/words`
- `PATCH /api/words/{id}`
- `DELETE /api/words/{id}`
- `POST /api/words/autofill-batch`
- `POST /api/words/{id}/generate-audio`
- `GET /api/drill/sessions/current`
- `POST /api/drill/sessions`
- `POST /api/drill/sessions/{id}/answers`
- `GET /api/settings/drill`
- `PUT /api/settings/drill`
- `GET /api/stories`
- `POST /api/stories`
- `DELETE /api/stories/{id}`
- `POST /api/stories/{id}/generate-translation`
- `POST /api/stories/{id}/generate-audio`
- `POST /api/tutor/chat`

## Developer Notes

- SQL is kept in the `db_*.go` files.
- Templates are parsed from disk on each request.
- Schema compatibility is not preserved across every development change; resetting the DB from the admin UI is an accepted workflow.

If `src/static/favicon.ico` changes, regenerate the checked-in Windows resource file:

```bash
cd src
wails3 generate syso -icon static/favicon.ico -manifest wails.exe.manifest -out wails_windows.syso -arch amd64
```
