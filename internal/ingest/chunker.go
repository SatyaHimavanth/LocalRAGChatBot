package ingest

import (
	"strings"
)

// Chunk represents a hierarchical chunk produced during ingestion.
type Chunk struct {
	Content     string
	Summary     string
	HeadingPath []string
	Ord         int
	Level       int
	ParentOrd   int
	PrevOrd     int
	NextOrd     int
	Role        string
}

// SplitText breaks a long string into overlapping leaf chunks, using Markdown-aware splitting if headers/formatting are present.
func SplitText(text string, chunkSize int, overlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}

	// Try markdown-aware splitting first.
	chunks := SplitMarkdownText(text, chunkSize, overlap)
	if len(chunks) > 0 {
		return chunks
	}

	// Fallback to basic word-based splitting.
	words := strings.Fields(text)
	var currentWords []string
	var currentLen int
	ord := 0

	for _, word := range words {
		currentWords = append(currentWords, word)
		currentLen += len(word) + 1

		if currentLen >= chunkSize {
			content := strings.Join(currentWords, " ")
			chunks = append(chunks, newLeafChunk(content, ord))
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
		chunks = append(chunks, newLeafChunk(strings.Join(currentWords, " "), ord))
	}

	return chunks
}

// SplitMarkdownText splits markdown text into leaf chunks, respecting headings and paragraphs.
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
		chunks = append(chunks, newLeafChunk(content, ord))
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

		// If it's a header and we already have content, flush the current chunk first.
		if isHeader && currentChunk.Len() > 0 {
			addChunk(currentChunk.String())
			currentChunk.Reset()
		}

		// If a single line/paragraph is extremely long, split it by words.
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

				if currentChunk.Len()+subLen >= chunkSize {
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

		// Prepend active header context if starting a new chunk.
		if currentChunk.Len() == 0 && currentHeader != "" && !isHeader {
			currentChunk.WriteString("[" + currentHeader + "]\n")
		}

		// If adding this line would exceed chunkSize, flush and start a new chunk.
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

// BuildChunkPlan produces hierarchical chunk nodes with parent and neighbor links.
func BuildChunkPlan(text string, title string, chunkSize int, overlap int) []Chunk {
	norm := NormalizeDocumentText(text)
	if norm == "" {
		return nil
	}

	sections := splitChunkSections(norm, title)
	var plan []Chunk
	ord := 0

	appendChunk := func(c Chunk) int {
		c.Ord = ord
		plan = append(plan, c)
		ord++
		return c.Ord
	}

	for _, sec := range sections {
		leaves := splitSectionIntoLeafChunks(sec.Body, sec.HeadingPath, chunkSize, overlap)
		if len(leaves) == 0 {
			continue
		}

		if len(leaves) == 1 && len(sec.HeadingPath) == 0 {
			leaf := leaves[0]
			leaf.Level = 0
			leaf.Role = "leaf"
			leaf.Summary = SummarizeDocument(leaf.Content, 2)
			leaf.Ord = ord
			plan = append(plan, leaf)
			ord++
			continue
		}

		sectionSummary := sec.Summary
		if sectionSummary == "" {
			sectionSummary = SummarizeDocument(sec.Body, 2)
		}
		if sectionSummary == "" {
			sectionSummary = summarizeHeadingOnly(sec.HeadingPath)
		}

		parentOrd := appendChunk(Chunk{
			Content:     prefixHeadingPath(sectionSummary, sec.HeadingPath),
			Summary:     sectionSummary,
			HeadingPath: append([]string(nil), sec.HeadingPath...),
			Level:       0,
			ParentOrd:   -1,
			Role:        "summary",
		})

		for i := range leaves {
			leaf := leaves[i]
			leaf.Level = 1
			leaf.Role = "leaf"
			leaf.ParentOrd = parentOrd
			leaf.Summary = SummarizeDocument(leaf.Content, 2)
			leaf.HeadingPath = append([]string(nil), sec.HeadingPath...)
			leaf.Ord = ord
			plan = append(plan, leaf)
			ord++
		}
	}

	for i := range plan {
		if i > 0 {
			plan[i].PrevOrd = plan[i-1].Ord
		} else {
			plan[i].PrevOrd = -1
		}
		if i < len(plan)-1 {
			plan[i].NextOrd = plan[i+1].Ord
		} else {
			plan[i].NextOrd = -1
		}
	}

	return plan
}

type chunkSection struct {
	HeadingPath []string
	Body        string
	Summary     string
}

func splitChunkSections(text string, title string) []chunkSection {
	lines := strings.Split(text, "\n")
	var sections []chunkSection
	var cur strings.Builder
	var headingPath []string
	var headingLevel int

	flush := func() {
		body := strings.TrimSpace(cur.String())
		if body == "" {
			cur.Reset()
			return
		}
		sections = append(sections, chunkSection{
			HeadingPath: append([]string(nil), headingPath...),
			Body:        body,
			Summary:     SummarizeDocument(body, 2),
		})
		cur.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if level, heading, ok := parseHeadingLine(trimmed); ok {
			flush()
			if level <= 0 {
				level = 1
			}
			headingLevel = level
			if len(headingPath) >= headingLevel {
				headingPath = append([]string(nil), headingPath[:headingLevel-1]...)
			}
			headingPath = append(headingPath, heading)
			continue
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(trimmed)
	}
	flush()

	if len(sections) == 0 && strings.TrimSpace(text) != "" {
		sections = append(sections, chunkSection{
			HeadingPath: compactHeadingPath(title),
			Body:        strings.TrimSpace(text),
			Summary:     SummarizeDocument(text, 2),
		})
	}
	return sections
}

func splitSectionIntoLeafChunks(body string, headingPath []string, chunkSize int, overlap int) []Chunk {
	chunks := SplitText(prefixHeadingPath(body, headingPath), chunkSize, overlap)
	for i := range chunks {
		chunks[i].Role = "leaf"
		chunks[i].Level = 0
		chunks[i].Summary = SummarizeDocument(chunks[i].Content, 2)
		chunks[i].HeadingPath = append([]string(nil), headingPath...)
	}
	return chunks
}

func parseHeadingLine(line string) (level int, heading string, ok bool) {
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}
	level = 0
	for _, r := range line {
		if r == '#' {
			level++
			continue
		}
		break
	}
	heading = strings.TrimSpace(strings.TrimLeft(line, "#"))
	if heading == "" {
		return 0, "", false
	}
	return level, heading, true
}

func prefixHeadingPath(body string, headingPath []string) string {
	body = strings.TrimSpace(body)
	if len(headingPath) == 0 {
		return body
	}
	prefix := "[" + strings.Join(headingPath, " > ") + "]"
	if strings.HasPrefix(body, prefix) {
		return body
	}
	if body == "" {
		return prefix
	}
	return prefix + "\n" + body
}

func summarizeHeadingOnly(headingPath []string) string {
	if len(headingPath) == 0 {
		return ""
	}
	return strings.Join(headingPath, " / ")
}

func compactHeadingPath(title string) []string {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	return []string{title}
}

func newLeafChunk(content string, ord int) Chunk {
	content = strings.TrimSpace(content)
	return Chunk{
		Content: content,
		Ord:     ord,
		Level:   0,
		Role:    "leaf",
	}
}
