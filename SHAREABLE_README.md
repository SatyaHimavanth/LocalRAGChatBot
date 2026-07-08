## Build the Production Binary

First, build a distributable executable (not dev mode):

```
wails3 build
```

This produces a single `.exe` at a path like:
```
build/bin/LocalRAGChatBot.exe
```

The frontend is **embedded inside the binary** via `//go:embed all:frontend/dist`, so you only need the `.exe` plus the model files.

---

## Recommended Folder Structure

Place everything in a single folder that users can extract anywhere:

```
LocalRAGChatBot/
├── LocalRAGChatBot.exe          ← compiled binary
├── .env                         ← optional configuration
├── models/
│   ├── chat/
│   │   └── qwen2.5-3b-instruct-q4_k_m.gguf
│   └── embed/
│       └── nomic-embed-text-v1.5.Q4_K_M.gguf
└── README.txt                   ← instructions for the user
```

### Why this works

The `main.go` resolves model paths relative to the **executable's own location** (not the current working directory):

```go
exePath, _ := os.Executable()
baseDir := filepath.Dir(exePath)

chatModelPath := filepath.Join(baseDir, "models", "chat", "qwen2.5-3b-instruct-q4_k_m.gguf")
embedModelPath := filepath.Join(baseDir, "models", "embed", "nomic-embed-text-v1.5.Q4_K_M.gguf")
```

So as long as the user keeps the folder structure intact, the app finds everything automatically — no matter where they extract it.

---

## The `.env` File (Optional)

You can override defaults with a `.env` file in the same folder as the `.exe`:

```env
# Custom model paths (relative or absolute)
CHAT_MODEL_PATH=models/chat/qwen2.5-3b-instruct-q4_k_m.gguf
EMBED_MODEL_PATH=models/embed/nomic-embed-text-v1.5.Q4_K_M.gguf

# DB filename (stored in %AppData%/LocalRAGChatBot/data/ by default)
DB_FILE_NAME=ragapp.db

# Override context window (default: auto-detected from RAM)
# CHAT_CONTEXT_SIZE=8192
```

If you omit `.env`, the app uses the defaults next to the executable — it still works.

---

## Database Location (Persistence)

The SQLite database is stored at a **stable OS path**, not in the app folder:

| OS | DB Location |
|----|-------------|
| **Windows** | `%AppData%/LocalRAGChatBot/data/ragapp.db` |
| **Linux** | `~/.local/share/LocalRAGChatBot/data/ragapp.db` |
| **macOS** | `~/.local/share/LocalRAGChatBot/data/ragapp.db` |

This means:
- ✅ User data survives app updates (just replace the `.exe`)
- ✅ Multiple users on the same machine each have their own DB
- ✅ The DB won't be accidentally deleted when cleaning up the app folder

---

## README.txt to Ship

```text
LocalRAG ChatBot
================

HOW TO RUN:
1. Extract this folder anywhere on your computer
2. Double-click LocalRAGChatBot.exe
3. The app will open and create its database automatically

REQUIREMENTS:
- Windows 10 or later (or Linux/macOS)
- ~4GB free RAM
- The GGUF model files in the models/ folder
  (chat model ~2GB, embedding model ~200MB)

FIRST TIME:
- The app creates a default "Knowledge Base" collection
- Upload documents in the Collections tab
- Ask questions in the Chat tab

CONFIGURATION:
- Edit .env to change model paths or context size
- Delete %AppData%/LocalRAGChatBot/data/ragapp.db to reset
```

---

## If You Want a Zip Distributable

PowerShell one-liner to package:

```powershell
Compress-Archive -Path "LocalRAGChatBot\LocalRAGChatBot.exe", "LocalRAGChatBot\models", "LocalRAGChatBot\.env", "LocalRAGChatBot\README.txt" -DestinationPath "LocalRAGChatBot-v1.0.zip"
```

---

## Model Download Notes

If users need to download models separately, point them to:

- **Chat model** (3B Q4): Search for `qwen2.5-3b-instruct-q4_k_m.gguf` on Hugging Face
- **Embedding model**: Search for `nomic-embed-text-v1.5.Q4_K_M.gguf` on Hugging Face

These go into `models/chat/` and `models/embed/` respectively.