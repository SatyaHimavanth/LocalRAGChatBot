package ingest

import (
	"strings"
)

type Chunk struct {
	Content string
	Ord     int
}

// SplitText breaks a long string into overlapping chunks based on character length.
func SplitText(text string, chunkSize int, overlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}

	words := strings.Fields(text)
	var chunks []Chunk
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

			// Overlap logic: keep the last N words
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

	// Catch remaining text
	if len(currentWords) > 0 {
		chunks = append(chunks, Chunk{
			Content: strings.Join(currentWords, " "),
			Ord:     ord,
		})
	}

	return chunks
}
