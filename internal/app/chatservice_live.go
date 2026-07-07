package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"changeme/internal/engine"
	"changeme/internal/ingest"
	"changeme/internal/store"

	llama "github.com/tcpipuk/llama-go"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type ChatService struct {
	Engine *engine.Engine // set in main.go
	DB     *sql.DB        // set in main.go

	app *application.App
}

func (s *ChatService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.app = application.Get()
	return nil
}

// SendMessage handles streaming LLM chat completions, retrieving dynamic database contexts if RAG is set up.
func (s *ChatService) SendMessage(sessionID int64, collectionID int64, prompt string) error {
	if s.app == nil {
		s.app = application.Get()
	}
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("prompt cannot be empty")
	}

	// If engine is not initialized, we fall back to a direct, clean mock message 
	// rather than crashing, to guarantee the app still starts and communicates nicely.
	if s.Engine == nil {
		s.emit("chat:token", map[string]any{
			"sessionId": sessionID,
			"token":     "Engine not initialized. Please set CHAT_MODEL_PATH and EMBED_MODEL_PATH environment variables correctly, or verify your build has CGO enabled.",
		})
		s.emit("chat:done", sessionID)
		return nil
	}

	// 1. Generate query embedding and perform HybridSearch if DB is accessible
	var chunks []store.ScoredChunk
	if s.DB != nil {
		queryEmb, err := s.Engine.Embed(prompt)
		if err == nil {
			// Search for matching chunks
			if searchChunks, err := store.HybridSearch(s.DB, collectionID, prompt, queryEmb, 5); err == nil {
				chunks = searchChunks
			}
		}
	}

	// 2. Build system + user messages
	messages := buildMessages(chunks, prompt)

	// 3. Create context and stream tokens
	chatCtx, err := s.Engine.ChatModel.NewContext(llama.WithContext(4096))
	if err != nil {
		return fmt.Errorf("creating chat context: %w", err)
	}

	deltas, errs := chatCtx.ChatStream(context.Background(), messages, llama.ChatOptions{})

	go func() {
		defer chatCtx.Close()
		for {
			select {
			case delta, ok := <-deltas:
				if !ok {
					s.emit("chat:done", sessionID)
					return
				}
				s.emit("chat:token", map[string]any{
					"sessionId": sessionID,
					"token":     delta.Content,
				})
			case err := <-errs:
				if err != nil {
					s.emit("chat:token", map[string]any{
						"sessionId": sessionID,
						"token":     fmt.Sprintf("\n[Error during generation: %v]\n", err),
					})
					s.emit("chat:done", sessionID)
					return
				}
			}
		}
	}()

	return nil
}

// IngestFile accepts file content, splits it into chunks, embeds them, and saves them to the DB.
func (s *ChatService) IngestFile(collectionID int64, filename string, fileContent string) error {
	if s.Engine == nil || s.DB == nil {
		return fmt.Errorf("engine or database not initialized")
	}

	// 1. Register file entry in DB
	docID, err := store.AddDocument(s.DB, collectionID, filename)
	if err != nil {
		return fmt.Errorf("registering document: %w", err)
	}

	// 2. Perform text chunking (500 chars with 100 overlaps)
	chunks := ingest.SplitText(fileContent, 500, 100)

	// 3. Generate embeddings and save chunks
	for _, chunk := range chunks {
		embedding, err := s.Engine.Embed(chunk.Content)
		if err != nil {
			return fmt.Errorf("generating chunk embedding: %w", err)
		}

		_, err = store.InsertChunk(s.DB, docID, collectionID, chunk.Content, chunk.Ord, embedding)
		if err != nil {
			return fmt.Errorf("inserting chunk: %w", err)
		}
	}

	return nil
}

func (s *ChatService) emit(name string, data ...any) {
	if s.app != nil {
		s.app.Event.Emit(name, data...)
	}
}

func buildMessages(chunks []store.ScoredChunk, prompt string) []llama.ChatMessage {
	var contextBuilder strings.Builder
	for _, chunk := range chunks {
		if chunk.Content == "" {
			continue
		}
		contextBuilder.WriteString("- ")
		contextBuilder.WriteString(chunk.Content)
		contextBuilder.WriteByte('\n')
	}

	systemPrompt := "You are a local RAG assistant. Answer from the provided context when it is relevant, and say when the context is not enough."
	if contextBuilder.Len() > 0 {
		systemPrompt += "\n\nContext:\n" + contextBuilder.String()
	}

	return []llama.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}
}
