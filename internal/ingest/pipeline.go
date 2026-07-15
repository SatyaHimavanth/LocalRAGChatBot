package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Loader extracts raw text from a supported file type.
type Loader interface {
	Name() string
	Supports(filename string) bool
	Load(data []byte, filename string) (string, error)
}

// DocumentMetadata captures ingestion-time metadata derived from the raw file and extracted text.
type DocumentMetadata struct {
	Loader             string   `json:"loader"`
	Extension          string   `json:"extension"`
	SizeBytes          int64    `json:"sizeBytes"`
	WordCount          int      `json:"wordCount"`
	LineCount          int      `json:"lineCount"`
	CharacterCount     int      `json:"characterCount"`
	ParagraphCount     int      `json:"paragraphCount"`
	Title              string   `json:"title"`
	Summary            string   `json:"summary"`
	Author             string   `json:"author"`
	Language           string   `json:"language"`
	LanguageConfidence float64  `json:"languageConfidence"`
	ReadingLevel       float64  `json:"readingLevel"`
	TokenEstimate      int      `json:"tokenEstimate"`
	DocumentClass      string   `json:"documentClass"`
	Keywords           []string `json:"keywords"`
	Entities           []string `json:"entities"`
	Tags               []string `json:"tags"`
	Headings           []string `json:"headings"`
	Pages              []int    `json:"pages"`
	Tables             []string `json:"tables"`
	Images             []string `json:"images"`
	Links              []string `json:"links"`
	CodeBlocks         []string `json:"codeBlocks"`
}

// LoadedDocument is the normalized result of running a file through a loader.
type LoadedDocument struct {
	Filename string
	RawText  string
	Text     string
	Metadata DocumentMetadata
}

var defaultLoaders []Loader

func ensureDefaultLoaders() {
	if len(defaultLoaders) > 0 {
		return
	}
	defaultLoaders = []Loader{
		txtLoader{},
		pdfLoader{},
		docxLoader{},
		markdownLoader{},
		htmlLoader{},
		csvLoader{},
		jsonLoader{},
		zipLoader{},
	}
}

// LoadDocumentBytes extracts and normalizes a file, returning a structured ingest envelope.
func LoadDocumentBytes(data []byte, filename string) (LoadedDocument, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "document.txt"
	}

	loader := pickLoader(filename)
	if loader == nil {
		return LoadedDocument{}, fmt.Errorf("unsupported format: %s (supported: .txt, .md, .html, .csv, .json, .pdf, .docx, .zip)", strings.ToLower(filepath.Ext(filename)))
	}

	rawText, err := loader.Load(data, filename)
	if err != nil {
		return LoadedDocument{}, err
	}
	text := NormalizeDocumentText(rawText)
	meta := BuildDocumentMetadata(filename, data, text, loader.Name())

	return LoadedDocument{
		Filename: filename,
		RawText:  rawText,
		Text:     text,
		Metadata: meta,
	}, nil
}

// LoadDocumentText extracts and normalizes a file from disk.
func LoadDocumentText(path string) (LoadedDocument, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return LoadDocumentPath(path)
	}
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		return LoadDocumentPath(path)
	}
	data, err := osReadFile(path)
	if err != nil {
		return LoadedDocument{}, fmt.Errorf("reading file: %w", err)
	}
	return LoadDocumentBytes(data, path)
}

func pickLoader(filename string) Loader {
	ensureDefaultLoaders()
	for _, loader := range defaultLoaders {
		if loader.Supports(filename) {
			return loader
		}
	}
	return nil
}

func BuildDocumentMetadata(filename string, data []byte, text string, loader string) DocumentMetadata {
	ext := strings.ToLower(filepath.Ext(filename))
	profile := ProfileDocument(filename, text, DocumentMetadata{})
	return DocumentMetadata{
		Loader:             loader,
		Extension:          ext,
		SizeBytes:          int64(len(data)),
		WordCount:          len(strings.Fields(text)),
		LineCount:          countMeaningfulLines(text),
		CharacterCount:     len([]rune(text)),
		ParagraphCount:     countParagraphs(text),
		Title:              DetectTitle(filename, text),
		Summary:            SummarizeDocument(text, 4),
		Author:             DetectAuthor(text),
		Language:           profile.Language,
		LanguageConfidence: profile.LanguageConfidence,
		ReadingLevel:       profile.ReadingLevel,
		TokenEstimate:      profile.TokenEstimate,
		DocumentClass:      profile.Class,
		Keywords:           ExtractKeywords(text, 12),
		Entities:           ExtractEntities(text, 12),
		Tags:               ExtractTags(filename, text),
		Headings:           ExtractHeadings(text),
		Pages:              ExtractPages(text),
		Tables:             ExtractTables(text),
		Images:             ExtractImages(text),
		Links:              ExtractLinks(text),
		CodeBlocks:         ExtractCodeBlocks(text),
	}
}

func NormalizeDocumentText(raw string) string {
	return normalizeExtractedText(raw)
}

func HashNormalizedText(text string) string {
	sum := sha256.Sum256([]byte(NormalizeDocumentText(text)))
	return hex.EncodeToString(sum[:])
}

func countMeaningfulLines(text string) int {
	lines := strings.Split(text, "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func countParagraphs(text string) int {
	parts := strings.Split(strings.TrimSpace(text), "\n\n")
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			count++
		}
	}
	return count
}

func DetectTitle(filename, text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		}
		if len([]rune(trimmed)) <= 120 {
			return trimmed
		}
		break
	}
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	base = strings.TrimSpace(strings.ReplaceAll(base, "_", " "))
	if base == "" {
		return "Untitled document"
	}
	return strings.Title(base)
}

func trimSentence(s string) string {
	return strings.TrimSpace(strings.Trim(s, " \t\r\n"))
}

func splitSentences(text string) []string {
	var sentences []string
	var cur strings.Builder
	for _, r := range text {
		cur.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			s := trimSentence(cur.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			cur.Reset()
		}
	}
	if tail := trimSentence(cur.String()); tail != "" {
		sentences = append(sentences, tail)
	}
	return sentences
}

func SummarizeDocument(text string, maxSentences int) string {
	text = NormalizeDocumentText(text)
	if text == "" {
		return ""
	}
	paras := strings.Split(text, "\n\n")
	for _, para := range paras {
		para = strings.TrimSpace(para)
		if para != "" && len([]rune(para)) <= 260 {
			return para
		}
	}

	sentences := splitSentences(text)
	if len(sentences) == 0 {
		words := strings.Fields(text)
		if len(words) == 0 {
			return ""
		}
		limit := minInt(35, len(words))
		return strings.Join(words[:limit], " ")
	}
	if maxSentences <= 0 || maxSentences > len(sentences) {
		maxSentences = minInt(3, len(sentences))
	}
	if maxSentences <= 0 {
		maxSentences = 1
	}
	return strings.Join(sentences[:maxSentences], " ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// osReadFile is defined as a shim so tests can swap it if needed.
var osReadFile = func(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// --- loader implementations ---

type txtLoader struct{}

func (txtLoader) Name() string { return "text" }
func (txtLoader) Supports(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".txt") || filepath.Ext(filename) == ""
}
func (txtLoader) Load(data []byte, filename string) (string, error) {
	return string(data), nil
}

type pdfLoader struct{}

func (pdfLoader) Name() string { return "pdf" }
func (pdfLoader) Supports(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".pdf")
}
func (pdfLoader) Load(data []byte, filename string) (string, error) { return parsePDFBytes(data) }

type docxLoader struct{}

func (docxLoader) Name() string { return "docx" }
func (docxLoader) Supports(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".docx")
}
func (docxLoader) Load(data []byte, filename string) (string, error) { return parseDOCXBytes(data) }
