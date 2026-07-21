# LocalRAG ChatBot

A fully offline, private, and high-performance **Local RAG (Retrieval-Augmented Generation) ChatBot** built with **Go**, **Wails v3 (Alpha)**, and **Llama-Go**. 

This application runs deep-learning language models and embedding pipelines 100% locally on your machine with zero external API dependencies or data leaks. It implements a hybrid-retrieval RAG engine using **SQLite** with combined **FTS5 (BM25 keyword search)** and **sqlite-vec (Vector KNN search)** merged via **RRF (Reciprocal Rank Fusion)**.

---

## 💻 System Prerequisites

Before starting, ensure your system has the following build tools installed:

1. **Go Compiler**: Go **v1.26.4** or newer installed.
2. **Node.js & npm**: Node.js **v18+** installed for building the React-TypeScript frontend.
3. **Wails v3 CLI**: Installed globally via:
   ```cmd
   go install github.com/wailsapp/wails/v3/cmd/wails3@latest
   ```
4. **C++ Compiler (MSYS2)**:
   * Download and install **[MSYS2](https://www.msys2.org/)**.
   * Run the **MSYS2 UCRT64** terminal from the Start Menu and install the GCC/G++ compilation toolchain:
     ```bash
     pacman -S --needed base-devel mingw-w64-ucrt-x86_64-toolchain
     ```
   * Add `C:\msys64\ucrt64\bin` to your system environment variables `PATH`.

---

## 📂 Project Structure

```text
LocalRAGChatBot/                       # Main application module
├── go.mod                             # Replaces llama-go with local third_party fork
├── main.go                            # Wails v3 entry point, DB & engine initialization
├── .env                               # Automatic local environment variable configuration
├── third_party/
│   └── llama-go/                      # Llama-Go static binding library (CGO C++ wrapper)
├── frontend/                          # React + TypeScript + Tailwind CSS Frontend
│   └── src/
│       ├── hooks/
│       │   └── useChatStream.ts       # Hook for token event streams
│       ├── components/
│       │   └── ChatWindow.tsx         # Chat Window React Component
│       └── App.tsx
├── internal/
│   ├── app/
│   │   └── chatservice_live.go        # Wails service for Chat & Ingestion pipelines
│   ├── engine/
│   │   └── engine.go                  # Llama-Go runner for LLM Chat & Text Embeddings
│   ├── ingest/
│   │   └── chunker.go                 # Text Chunking and splitting engine
│   └── store/
│       ├── db.go                      # SQLite Connection and migration registry
│       ├── documents.go               # Collections, Documents, and Chunks DB service
│       ├── retrieval.go               # Vector search + BM25 keyword search + RRF Fusion
│       └── migrations/                # Database schemas (0001_init, 0002_fts5, 0003_vec)
└── models/                            # Large Model Files (Do NOT push to Git!)
    ├── chat/
    │   └── qwen2.5-3b-instruct-q4_k_m.gguf
    └── embed/
        └── nomic-embed-text-v1.5.Q4_K_M.gguf
```

---

## 🛠️ First-Time Setup Instructions

Follow these steps precisely to compile the required local dependencies.

### Step 1: Compile the C++ Llama-Go Bindings
Because `llama-go` leverages high-performance C++ code internally, you must compile its static bindings using your MSYS2 toolchain:

Open a standard **Windows Command Prompt (CMD)** and run:
```cmd
:: Spin up the MSYS2 UCRT64 environment and compile the C++ binaries
C:\msys64\msys2_shell.cmd -defterm -here -no-start -ucrt64 -c "cd third_party/llama-go && make clean && make libbinding.a"
```

### Step 2: Configure your `.env` file
Create a file named `.env` in the root folder (`LocalRAGChatBot/.env`) to set the environment paths automatically:

```ini
# Model Configurations
CHAT_MODEL_PATH=<>\LocalRAGChatBot\models\chat\qwen2.5-3b-instruct-q4_k_m.gguf
EMBED_MODEL_PATH=<>\LocalRAGChatBot\models\embed\nomic-embed-text-v1.5.Q4_K_M.gguf
DB_FILE_NAME=ragapp.db

# Windows Compiler/CGO Settings
CGO_ENABLED=1
CC=gcc
CXX=g++
LIBRARY_PATH=<>\LocalRAGChatBot\third_party\llama-go
C_INCLUDE_PATH=<>\LocalRAGChatBot\third_party\llama-go
```
*(Be sure to adjust the absolute paths if your project directory name differs!)*

### Step 3: Configure Go's Global Env Settings
Go needs persistent user-level declarations to compile using the Windows MSYS2 toolchain. Run these once in CMD:
```cmd
go env -w CGO_ENABLED=1
go env -w CC=gcc
go env -w CXX=g++
go env -w GOFLAGS="-tags=fts5,libsqlite3"
```

---

## 🚀 Running the Project

To launch the project in live-development mode (with hot-reloading for both Go and React), run the following command sequence in your **Windows Command Prompt (CMD)**:

```cmd
:: 1. Force CGO on
set CGO_ENABLED=1

:: 2. Point Go's CGO compiler at MSYS2's GCC (resolved via PATH - see step 4)
go env -w CC=gcc
go env -w CXX=g++
go env -w GOFLAGS="-tags=fts5,libsqlite3"

:: 3. Clear Go's package build cache
go clean -cache

:: 4. Add the ucrt64 binary directory to the system PATH
set PATH=C:\msys64\ucrt64\bin;%PATH%

:: 5. Define library paths
set LIBRARY_PATH=<>\LocalRAGChatBot\third_party\llama-go
set C_INCLUDE_PATH=<>\LocalRAGChatBot\third_party\llama-go

:: 6. Set LLM paths
set CHAT_MODEL_PATH=<>\LocalRAGChatBot\models\chat\chat-model.gguf
set EMBED_MODEL_PATH=<>\LocalRAGChatBot\models\embed\embed-model.gguf

:: 7. Launch development build
wails3 dev
```

---

## 📦 Building for Production

`wails3 build` does **not** pick up the `GOFLAGS` you set in Step 3 above.
`wails3 build`/`wails3 package` only forward one build-time flag - `--tags` -
which becomes the `EXTRA_TAGS` variable the underlying Taskfile passes to
`go build`. `GOFLAGS` only affects bare `go build`/`go run` invocations, not
whatever tags Wails' own Taskfile constructs, so `fts5,libsqlite3` never
reaches the compiler unless you pass it explicitly:

```cmd
wails3 build --tags fts5,libsqlite3
```

(same for `wails3 package` if you use it to produce the distributable).
Skipping this compiles a binary that silently can't create the FTS5 search
table - see Troubleshooting below for what that looks like.

---

## 📖 Database Schema & Hybrid RAG Retrieval

This project uses an offline SQLite schema split into three migration steps:
1. **`0001_init.sql`**: Relational structure for Collections, Documents, standard Chunks, and Chat History.
2. **`0002_fts5.sql`**: Virtual SQLite Full-Text-Search (FTS5) table mapping chunks with triggers that automatically sync additions/deletions.
3. **`0003_vec.sql`**: High-performance virtual vector table (`vec0` via `sqlite-vec`) representing chunk text embeddings natively in SQLite.

### Reciprocal Rank Fusion (RRF)
The RAG pipeline retrieves the top 20 matches using FTS5 keyword similarity, and the top 20 matches using vector KNN distance. It then blends the results using **RRF** ($k=60$):

$$\text{RRF Score}(d) = \sum_{m \in M} \frac{1}{60 + \text{Rank}_m(d)}$$

The fused list is sorted, and the top 5 most relevant contexts are injected directly into the LLM system prompt dynamically on every message.