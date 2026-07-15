package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type DocumentProfile struct {
	Filename       string
	Extension      string
	Title          string
	Summary        string
	NormalizedText string
	ContentHash    string
	WordCount      int
	CharCount      int
	LineCount      int
	ParagraphCount int
	HeadingCount   int
	ListCount      int
	CodeBlockCount int
}

type ChunkSpec struct {
	Content      string
	Ord          int
	Title        string
	SectionPath  string
	HeadingLevel int
	Summary      string
	ContentHash  string
	TokenCount   int
	CharCount    int
}

var headingPrefix = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// NormalizeDocumentText collapses noise while preserving headings and paragraph breaks.
func NormalizeDocumentText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")

	var out []string
	var current strings.Builder
	flush := func() {
		if s := strings.TrimSpace(current.String()); s != "" {
			out = append(out, strings.Join(strings.Fields(s), " "))
		}
		current.Reset()
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			out = append(out, strings.Join(strings.Fields(line), " "))
			continue
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(strings.Join(strings.Fields(line), " "))
	}
	flush()
	return strings.TrimSpace(strings.Join(out, "\n\n"))
}

// BuildDocumentProfile returns lightweight metadata for a document.
func BuildDocumentProfile(filename, rawText string) DocumentProfile {
	normalized := NormalizeDocumentText(rawText)
	ext := strings.ToLower(filepath.Ext(filename))
	title := deriveDocumentTitle(filename, normalized)
	summary := SummarizeText(normalized, 3)
	hash := sha256.Sum256([]byte(normalized))

	lines := strings.Split(normalized, "\n")
	wordCount := len(strings.Fields(normalized))
	charCount := len([]rune(normalized))
	paraCount := 0
	headingCount := 0
	listCount := 0
	codeCount := 0
	inCode := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "```") {
			inCode = !inCode
			codeCount++
			continue
		}
		if inCode {
			continue
		}
		if strings.HasPrefix(t, "#") {
			headingCount++
		}
		if isListLine(t) {
			listCount++
		}
		paraCount++
	}

	return DocumentProfile{
		Filename:       strings.TrimSpace(filename),
		Extension:      ext,
		Title:          title,
		Summary:        summary,
		NormalizedText: normalized,
		ContentHash:    hex.EncodeToString(hash[:]),
		WordCount:      wordCount,
		CharCount:      charCount,
		LineCount:      len(lines),
		ParagraphCount: paraCount,
		HeadingCount:   headingCount,
		ListCount:      listCount,
		CodeBlockCount: codeCount / 2,
	}
}

// BuildChunkSpecs attaches simple metadata to chunks split from a document.
func BuildChunkSpecs(filename, rawText string, chunkSize, overlap int) ([]ChunkSpec, DocumentProfile) {
	profile := BuildDocumentProfile(filename, rawText)
	base := SplitText(profile.NormalizedText, chunkSize, overlap)
	specs := make([]ChunkSpec, 0, len(base))

	var activeSections []string
	activeTitle := profile.Title
	activeLevel := 0
	for _, chunk := range base {
		title, sectionPath, level := deriveChunkSection(chunk.Content, activeTitle, activeSections, profile.Title)
		if title != "" {
			activeTitle = title
		}
		if sectionPath != "" {
			activeSections = parseSectionPath(sectionPath)
		}
		if level > 0 {
			activeLevel = level
		}

		content := strings.TrimSpace(chunk.Content)
		specs = append(specs, ChunkSpec{
			Content:      content,
			Ord:          chunk.Ord,
			Title:        chooseChunkTitle(content, activeTitle, profile.Title),
			SectionPath:  sectionPath,
			HeadingLevel: activeLevel,
			Summary:      SummarizeText(content, 2),
			ContentHash:  hashString(content),
			TokenCount:   len(strings.Fields(content)),
			CharCount:    len([]rune(content)),
		})
	}
	return specs, profile
}

func deriveDocumentTitle(filename, normalized string) string {
	if title := firstHeading(normalized); title != "" {
		return title
	}
	base := filepath.Base(strings.TrimSpace(filename))
	if base == "" {
		return "Document"
	}
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.Join(strings.Fields(base), " ")
	if base == "" {
		return "Document"
	}
	return strings.Title(base)
}

func firstHeading(text string) string {
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "#") {
			continue
		}
		m := headingPrefix.FindStringSubmatch(t)
		if len(m) == 3 {
			return strings.TrimSpace(m[2])
		}
	}
	return ""
}

func chooseChunkTitle(content, activeTitle, fallback string) string {
	if title := firstHeading(content); title != "" {
		return title
	}
	if activeTitle != "" {
		return activeTitle
	}
	if fallback != "" {
		return fallback
	}
	if s := firstLine(content); s != "" {
		return s
	}
	return "Chunk"
}

func deriveChunkSection(content, activeTitle string, activeSections []string, fallback string) (title, sectionPath string, level int) {
	lines := strings.Split(content, "\n")
	sections := append([]string(nil), activeSections...)
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if m := headingPrefix.FindStringSubmatch(t); len(m) == 3 {
			level = len(m[1])
			title = strings.TrimSpace(m[2])
			if level > 0 {
				if len(sections) >= level {
					sections = sections[:level-1]
				}
				sections = append(sections, title)
			}
			break
		}
	}
	if len(sections) > 0 {
		sectionPath = strings.Join(sections, " > ")
	} else if activeTitle != "" {
		sectionPath = activeTitle
	} else {
		sectionPath = fallback
	}
	return title, sectionPath, level
}

func parseSectionPath(path string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	parts := strings.Split(path, " > ")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if t != "" {
			return truncateText(t, 80)
		}
	}
	return ""
}

func hashString(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func isListLine(line string) bool {
	if line == "" {
		return false
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	if len(line) > 2 && unicode.IsDigit(rune(line[0])) && strings.Contains(line, ". ") {
		return true
	}
	return false
}

func truncateText(text string, limit int) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return text[:limit-3] + "..."
}
