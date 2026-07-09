package ingest

import (
	"strings"
)

type Chunk struct {
	Content string
	Ord     int
}

// SplitText breaks a long string into overlapping chunks, using Markdown-aware splitting if headers/formatting are present.
func SplitText(text string, chunkSize int, overlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}

	// Try markdown-aware splitting first
	chunks := SplitMarkdownText(text, chunkSize, overlap)
	if len(chunks) > 0 {
		return chunks
	}

	// Fallback to basic word-based splitting
	words := strings.Fields(text)
	var currentWords []string
	var currentLen int
	ord := 0

	for _, word := range words {
		currentWords = append(currentWords, word)
		currentLen += len(word) + 1

		if currentLen >= chunkSize {
			content := strings.Join(currentWords, " ")
			chunks = append(chunks, Chunk{
				Content: content,
				Ord:     ord,
			})
			ord++

			keepCount := len(currentWords) / 4
			if keepCount > 0 {
				currentWords = currentWords[len(currentWords)-keepCount:]
				currentLen = len(strings.Join(currentWords, " "))
			} else {
				currentWords = nil
				currentLen = 0
			}
		}
	}

	if len(currentWords) > 0 {
		chunks = append(chunks, Chunk{
			Content: strings.Join(currentWords, " "),
			Ord:     ord,
		})
	}

	return chunks
}

// SplitMarkdownText splits markdown text into chunks, respecting headings and paragraphs.
func SplitMarkdownText(text string, chunkSize int, overlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	
	lines := strings.Split(text, "\n")
	var chunks []Chunk
	var currentChunk strings.Builder
	var currentHeader string
	ord := 0

	addChunk := func(content string) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		chunks = append(chunks, Chunk{
			Content: content,
			Ord:     ord,
		})
		ord++
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		isHeader := false
		if strings.HasPrefix(trimmed, "#") {
			isHeader = true
			hText := strings.TrimLeft(trimmed, "# ")
			if hText != "" {
				currentHeader = hText
			}
		}

		// If it's a header and we already have content, flush the current chunk first
		if isHeader && currentChunk.Len() > 0 {
			addChunk(currentChunk.String())
			currentChunk.Reset()
		}

		// If a single line/paragraph is extremely long, split it by words
		if len(trimmed) > chunkSize {
			words := strings.Fields(trimmed)
			var subWords []string
			var subLen int
			
			if currentChunk.Len() == 0 && currentHeader != "" {
				currentChunk.WriteString("[" + currentHeader + "]\n")
			}
			
			for _, w := range words {
				subWords = append(subWords, w)
				subLen += len(w) + 1
				
				if currentChunk.Len() + subLen >= chunkSize {
					if currentChunk.Len() > 0 && !strings.HasSuffix(currentChunk.String(), "\n") {
						currentChunk.WriteByte('\n')
					}
					currentChunk.WriteString(strings.Join(subWords, " "))
					addChunk(currentChunk.String())
					currentChunk.Reset()
					
					subWords = nil
					subLen = 0
					
					if currentHeader != "" {
						currentChunk.WriteString("[" + currentHeader + "]\n")
					}
				}
			}
			if len(subWords) > 0 {
				if currentChunk.Len() > 0 && !strings.HasSuffix(currentChunk.String(), "\n") {
					currentChunk.WriteByte('\n')
				}
				currentChunk.WriteString(strings.Join(subWords, " "))
			}
			continue
		}

		// Prepend active header context if starting a new chunk
		if currentChunk.Len() == 0 && currentHeader != "" && !isHeader {
			currentChunk.WriteString("[" + currentHeader + "]\n")
		}

		// If adding this line would exceed chunkSize, flush and start a new chunk
		if currentChunk.Len() > 0 && currentChunk.Len()+len(trimmed) > chunkSize {
			addChunk(currentChunk.String())
			currentChunk.Reset()
			if currentHeader != "" {
				currentChunk.WriteString("[" + currentHeader + "]\n")
			}
		}

		if currentChunk.Len() > 0 && !strings.HasSuffix(currentChunk.String(), "\n") {
			currentChunk.WriteByte('\n')
		}
		currentChunk.WriteString(trimmed)
	}

	if currentChunk.Len() > 0 {
		addChunk(currentChunk.String())
	}

	return chunks
}
