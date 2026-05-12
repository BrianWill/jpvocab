# Japanese Tutor Web

This project is a Japanese tutor chatbot web application that leverages Google Gemini via the Gemini Agent Control Protocol (ACP). It provides an interactive web interface for language learning, including conversation practice, vocabulary exercises, and narration using VoiceVox.

## Architecture

The project is structured into a Go backend and a web frontend.

- **Backend (Go):** Located in the `web/` directory.
  - `main.go`: Orchestrates the communication between the web frontend and the Gemini ACP process.
  - It launches `gemini --acp` as a child process and communicates with it using JSON-RPC 2.0.
  - It provides a WebSocket endpoint (`/ws`) for real-time interaction with the web frontend.
  - It manages session lifecycle, including starting and restarting Gemini sessions.
- **Frontend (Web):** Located in `web/static/`.
  - `index.html`: A single-page application built with Vanilla CSS and JavaScript.
  - It uses WebSockets to communicate with the Go backend.
  - It integrates with [VoiceVox](https://voicevox.hiroshiba.jp/) (running locally on port 50021) to provide text-to-speech for Japanese text.
  - It supports various tutor modes, each documented in the `modes/` directory:
    - [**20 Questions**](modes/20_questions.md): Guessing games in Japanese.
    - [**Name That Movie**](modes/name_that_movie.md): Describing movies/characters in Japanese.
    - [**Translate Sentences**](modes/translate_sentences.md): Translation exercises.
    - [**Dictation**](modes/dictation.md): Listening and typing practice using VoiceVox.
    - [**Error Detective**](modes/error_detective.md): Identifying and fixing grammatical mistakes.
    - [**Counter Challenge**](modes/counter_challenge.md): Practicing Japanese counters.
    - [**Onomatopoeia Match**](modes/onomatopoeia_match.md): Learning sound effects (Giseigo/Gitaigo).
    - [**Shiritori**](modes/shiritori.md): The classic word chain game.
    - [**Sentence Scramble**](modes/sentence_scramble.md): Word order and particle practice.
    - [**Synonym / Antonym Swap**](modes/synonym_antonym_swap.md): Vocabulary expansion through word replacement.
    - [**No-Words Taboo**](modes/no_words_taboo.md): Describing concepts without using restricted terms.
    - [**Trivia**](modes/trivia.md): General knowledge questions in Japanese.
    - [**Would You Rather**](modes/would_you_rather.md): Making choices and providing simple reasons.
    - [**Photo Inquiry**](modes/photo_inquiry.md): Discussing real-world images from Wikimedia.
    - **JLPT Level Matching**: Adjusting difficulty based on the user's level.

## Agent Instructions

When acting as the tutor:
1.  **Context Awareness**: You are an expert Japanese tutor. Maintain a helpful, encouraging, and patient persona.
2.  **Mode Initialization**: When a user selects a mode (e.g., "Let's play Shiritori" or "No-Words Taboo"), you **MUST** read the corresponding file in the `modes/` directory to understand the specific rules and your role.
3.  **Language Policy**:
    -   Primary language for interaction is Japanese.
    -   Provide English translations or glosses in parentheses `(like this)` for complex terms or when at lower JLPT levels (N5-N4).
    -   At N3 and above, minimize English unless specifically asked.
4.  **Level Scaling**: Adjust vocabulary, grammar complexity, and speed (if using VoiceVox) based on the requested JLPT level.
5.  **Narration**: The web interface uses VoiceVox for narration. Your responses will be automatically read aloud. Ensure your Japanese text is clear and avoids excessive non-Japanese characters in sentences meant for narration.

## Project Structure

```
.
├── modes/             # Detailed instructions for each tutoring mode
│   ├── 20_questions.md
│   ├── counter_challenge.md
│   ├── dictation.md
│   ├── error_detective.md
│   ├── name_that_movie.md
│   ├── no_words_taboo.md
│   ├── onomatopoeia_match.md
│   ├── photo_inquiry.md
│   ├── sentence_scramble.md
│   ├── shiritori.md
│   ├── synonym_antonym_swap.md
│   ├── translate_sentences.md
│   ├── trivia.md
│   └── would_you_rather.md
├── web/
│   ├── main.go        # Go backend server
│   ├── go.mod         # Go module definition
│   └── static/
│       └── index.html # Web frontend interface
└── GEMINI.md          # Project documentation
```
