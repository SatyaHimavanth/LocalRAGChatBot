package main

import (
	"bufio"
	"database/sql"
	"embed"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	appsvc "changeme/internal/app"
	"changeme/internal/engine"
	"changeme/internal/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
}

// loadDotEnv reads a .env file from the given path if it exists,
// parsing simple KEY=VALUE pairs and loading them into the Go process environment.
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return // Silently skip if no .env is found
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
		// Strip wrapping quotes if any
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
	// 1. Resolve base directory and load .env automatically
	exePath, err := os.Executable()
	var baseDir string
	if err == nil {
		baseDir = filepath.Dir(exePath)
	} else {
		baseDir = "."
	}

	// First, check for .env in current working directory
	loadDotEnv(".env")
	// Also check for .env next to the binary if they are different
	if baseDir != "." {
		loadDotEnv(filepath.Join(baseDir, ".env"))
	}

	// 2. Load model paths with fallbacks
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

	// Log resolved paths to assist debugging
	log.Printf("Resolved chatModelPath: %s", chatModelPath)
	log.Printf("Resolved embedModelPath: %s", embedModelPath)

	// 3. Initialize the Database and Engine
	var dbConn *sql.DB
	var dbErr error
	var eng *engine.Engine
	var engErr error

	// We attempt to initialize the engine if the models are found on disk.
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

	// Open the SQLite Database
	dbConn, dbErr = store.Open(dbFileName)
	if dbErr != nil {
		log.Printf("Warning: Failed to initialize database: %v", dbErr)
	}

	// Make sure we release engine resources on shutdown
	if eng != nil {
		defer eng.Close()
	}

	// 4. Create the Wails application and register services.
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

	// Create a new window with the necessary options.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
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

	// Create a goroutine that emits an event containing the current time every second.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	// Run the application. This blocks until the application has been exited.
	appErr := app.Run()
	if appErr != nil {
		log.Fatal(appErr)
	}
}
