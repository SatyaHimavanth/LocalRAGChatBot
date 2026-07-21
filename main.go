package main

import (
	"bufio"
	"database/sql"
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	appsvc "changeme/internal/app"
	"changeme/internal/diagnostic"
	"changeme/internal/engine"
	"changeme/internal/store"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[string]("time")
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	log.Printf("Loading environment variables from: %s", path)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			os.Setenv(key, val)
		}
	}
}

// AppConfig holds the app's runtime configuration: everything that comes
// from .env (or its defaults). Resolved once in loadConfig and passed to
// whatever needs it, instead of each piece separately reading os.Getenv.
type AppConfig struct {
	BaseDir        string // directory containing the executable
	ChatModelPath  string
	EmbedModelPath string
	DBPath         string // fully resolved: <BaseDir>/data/<DB_FILE_NAME>
}

// loadConfig loads .env - checked both in the current working directory and
// next to the executable, so it's found whichever way the app is launched -
// then resolves the 3 configurable paths. An env var always wins; otherwise
// each falls back to a location relative to the executable, so copying the
// exe alongside models/ (and letting data/ be created next to it) is enough
// to run the app standalone, without setting up a shell environment first.
func loadConfig() AppConfig {
	exePath, err := os.Executable()
	baseDir := "."
	if err == nil {
		baseDir = filepath.Dir(exePath)
	}

	loadDotEnv(".env")
	if baseDir != "." {
		loadDotEnv(filepath.Join(baseDir, ".env"))
	}

	chatModelPath := strings.TrimSpace(os.Getenv("CHAT_MODEL_PATH"))
	if chatModelPath == "" {
		chatModelPath = filepath.Join(baseDir, "models", "chat", "qwen2.5-3b-instruct-q4_k_m.gguf")
	}

	embedModelPath := strings.TrimSpace(os.Getenv("EMBED_MODEL_PATH"))
	if embedModelPath == "" {
		embedModelPath = filepath.Join(baseDir, "models", "embed", "nomic-embed-text-v1.5.Q4_K_M.gguf")
	}

	dbFileName := strings.TrimSpace(os.Getenv("DB_FILE_NAME"))
	if dbFileName == "" {
		dbFileName = "ragapp.db"
	}
	// Only the filename is configurable - the db always lives in
	// <exe dir>/data/, even if DB_FILE_NAME itself contains a path.
	dbFileName = filepath.Base(dbFileName)

	return AppConfig{
		BaseDir:        baseDir,
		ChatModelPath:  chatModelPath,
		EmbedModelPath: embedModelPath,
		DBPath:         filepath.Join(baseDir, "data", dbFileName),
	}
}

func main() {
	cfg := loadConfig()
	closeLogs := diagnostic.Configure(cfg.BaseDir)
	defer closeLogs()
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("[panic] unrecovered panic: %v\n%s", recovered, debug.Stack())
			panic(recovered)
		}
	}()
	log.Printf("=== LocalRAGChatBot starting (pid %d) ===", os.Getpid())

	log.Printf("Resolved chatModelPath: %s", cfg.ChatModelPath)
	log.Printf("Resolved embedModelPath: %s", cfg.EmbedModelPath)
	log.Printf("Resolved dbPath: %s", cfg.DBPath)

	var dbConn *sql.DB
	var dbErr error
	var eng *engine.Engine
	var engErr error

	if _, err := os.Stat(cfg.ChatModelPath); err == nil {
		if _, err := os.Stat(cfg.EmbedModelPath); err == nil {
			log.Printf("Loading engine with models:\n- Chat: %s\n- Embed: %s", cfg.ChatModelPath, cfg.EmbedModelPath)
			eng, engErr = engine.New(cfg.ChatModelPath, cfg.EmbedModelPath)
			if engErr != nil {
				log.Printf("Warning: Failed to load Llama-Go engine: %v", engErr)
			}
		} else {
			log.Printf("Embedding model not found at: %s. Engine initialization skipped.", cfg.EmbedModelPath)
		}
	} else {
		log.Printf("Chat model not found at: %s. Engine initialization skipped.", cfg.ChatModelPath)
	}

	dbConn, dbErr = store.Open(cfg.DBPath)
	if dbErr != nil {
		log.Printf("Warning: Failed to initialize database: %v", dbErr)
	}

	if eng != nil {
		defer eng.Close()
	}

	chatService := &appsvc.ChatService{
		Engine:         eng,
		DB:             dbConn,
		EmbedModelPath: cfg.EmbedModelPath,
	}

	app := application.New(application.Options{
		Name:        "LocalRAGChatBot",
		Description: "A fully working Local RAG chat application powered by Go, Wails v3, and Llama-Go.",
		Services: []application.Service{
			application.NewService(&GreetService{}),
			application.NewService(chatService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	var forceClose atomic.Bool
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Local RAG ChatBot",
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// Warn when closing during ingest; staged text is durable and can resume later.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {

		if !chatService.IsIngesting() {
			return
		}

		dialog := app.Dialog.Question().
			SetTitle("Ingestion in progress").
			SetMessage("Close anyway?")

		stay := dialog.AddButton("Stay")
		close := dialog.AddButton("Close")

		stay.OnClick(func() {
			e.Cancel()
		})

		close.OnClick(func() {
			forceClose.Store(true)
			chatService.CancelIngest()
			// NOTHING ELSE
		})

		dialog.Show()
	})

	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	appErr := app.Run()
	if appErr != nil {
		log.Printf("=== LocalRAGChatBot exiting with error: %v ===", appErr)
		log.Fatal(appErr)
	}
	log.Printf("=== LocalRAGChatBot exited cleanly ===")
}
