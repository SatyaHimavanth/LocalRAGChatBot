package main

import (
	"bufio"
	"database/sql"
	"embed"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	appsvc "changeme/internal/app"
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

func main() {
	exePath, err := os.Executable()
	var baseDir string
	if err == nil {
		baseDir = filepath.Dir(exePath)
	} else {
		baseDir = "."
	}

	loadDotEnv(".env")
	if baseDir != "." {
		loadDotEnv(filepath.Join(baseDir, ".env"))
	}

	chatModelPath := os.Getenv("CHAT_MODEL_PATH")
	if chatModelPath == "" {
		chatModelPath = filepath.Join(baseDir, "models", "chat", "qwen2.5-3b-instruct-q4_k_m.gguf")
	}

	embedModelPath := os.Getenv("EMBED_MODEL_PATH")
	if embedModelPath == "" {
		embedModelPath = filepath.Join(baseDir, "models", "embed", "nomic-embed-text-v1.5.Q4_K_M.gguf")
	}

	dbFileName := os.Getenv("DB_FILE_NAME")
	if dbFileName == "" {
		dbFileName = "ragapp.db"
	}

	log.Printf("Resolved chatModelPath: %s", chatModelPath)
	log.Printf("Resolved embedModelPath: %s", embedModelPath)

	var dbConn *sql.DB
	var dbErr error
	var eng *engine.Engine
	var engErr error

	if _, err := os.Stat(chatModelPath); err == nil {
		if _, err := os.Stat(embedModelPath); err == nil {
			log.Printf("Loading engine with models:\n- Chat: %s\n- Embed: %s", chatModelPath, embedModelPath)
			eng, engErr = engine.New(chatModelPath, embedModelPath)
			if engErr != nil {
				log.Printf("Warning: Failed to load Llama-Go engine: %v", engErr)
			}
		} else {
			log.Printf("Embedding model not found at: %s. Engine initialization skipped.", embedModelPath)
		}
	} else {
		log.Printf("Chat model not found at: %s. Engine initialization skipped.", chatModelPath)
	}

	dbConn, dbErr = store.Open(dbFileName)
	if dbErr != nil {
		log.Printf("Warning: Failed to initialize database: %v", dbErr)
	}

	if eng != nil {
		defer eng.Close()
	}

	chatService := &appsvc.ChatService{
		Engine: eng,
		DB:     dbConn,
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
		if forceClose.Load() || !chatService.IsIngesting() {
			return
		}
		e.Cancel()

		allowClose := false
		dialog := app.Dialog.Question().
			SetTitle("Ingestion in progress").
			SetMessage("Documents are still being prepared or embedded.\n\nStaged text is saved. You can resume embedding after restart.\n\nClose anyway?")
		stayBtn := dialog.AddButton("Stay")
		closeBtn := dialog.AddButton("Close")
		dialog.SetDefaultButton(stayBtn)
		dialog.SetCancelButton(stayBtn)
		closeBtn.OnClick(func() {
			allowClose = true
		})
		dialog.Show()

		if allowClose {
			chatService.CancelIngest()
			chatService.WaitIngestIdle(5000)
			forceClose.Store(true)
			win.Close()
		}
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
		log.Fatal(appErr)
	}
}
