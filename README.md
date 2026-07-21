# LocalRAGChatBot

> A fully offline Retrieval-Augmented Generation (RAG) desktop
> application built with **Go**, **Wails v3**, **React**, **SQLite**,
> **FTS5**, **sqlite-vec**, and **local GGUF models**.

LocalRAGChatBot allows you to chat with your own documents without
sending data to external AI services. Documents, embeddings, vector
indexes, and chat history remain on your machine.

## Highlights

-   100% offline inference
-   Desktop UI built with Wails v3
-   Hybrid retrieval (BM25 + Vector Search)
-   SQLite database with FTS5 and sqlite-vec
-   Local GGUF chat and embedding models
-   Document ingestion pipeline
-   Cross-platform source code (Windows binaries provided through
    releases)

## Technology Stack

  Component   Technology
  ----------- --------------------
  Backend     Go
  Desktop     Wails v3
  Frontend    React + TypeScript
  Database    SQLite
  Search      FTS5 + sqlite-vec
  Models      GGUF via llama-go

## Repository Structure

``` text
frontend/          React UI
internal/          Application logic
third_party/       llama-go bindings
models/            Local models (not committed)
```

## Prerequisites

-   Go 1.26.4+
-   Node.js 18+
-   Wails v3 CLI
-   MSYS2 UCRT64 (Windows builds)

## Building

Install Wails:

``` bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Build:

``` bash
wails3 build --tags fts5,libsqlite3
```

Development:

``` bash
wails3 dev
```

## Runtime Configuration

Configure local model paths using a `.env` file.

Example:

``` ini
CHAT_MODEL_PATH=...
EMBED_MODEL_PATH=...
DB_FILE_NAME=ragapp.db
```

## Windows Release

The GitHub Release contains:

-   LocalRAGChatBot.exe
-   Required DLL files

Keep every DLL in the same directory as the executable.

Linux and macOS users should build from source.

## Contributing

Issues and pull requests are welcome.

## License

Add the appropriate license for this repository.
