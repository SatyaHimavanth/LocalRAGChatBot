package ingest

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// DocumentProfile captures coarse document classification and readability stats.
type DocumentProfile struct {
	Language           string
	LanguageConfidence float64
	ReadingLevel       float64
	TokenEstimate      int
	Class              string
}

type markdownLoader struct{}
type htmlLoader struct{}
type csvLoader struct{}
type jsonLoader struct{}
type zipLoader struct{}

func (markdownLoader) Name() string { return "markdown" }
func (markdownLoader) Supports(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown" || ext == ".mkd"
}
func (markdownLoader) Load(data []byte, filename string) (string, error) {
	return normalizeExtractedText(string(data)), nil
}

func (htmlLoader) Name() string { return "html" }
func (htmlLoader) Supports(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".html" || ext == ".htm" || ext == ".xhtml"
}
func (htmlLoader) Load(data []byte, filename string) (string, error) {
	txt := stripXMLTags(string(data))
	return normalizeExtractedText(txt), nil
}

func (csvLoader) Name() string { return "csv" }
func (csvLoader) Supports(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".csv")
}
func (csvLoader) Load(data []byte, filename string) (string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.ReuseRecord = true
	r.FieldsPerRecord = -1

	var out strings.Builder
	rowNum := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// fall back to raw text if CSV parsing is broken
			return normalizeExtractedText(string(data)), nil
		}
		rowNum++
		if len(rec) == 0 {
			continue
		}
		if rowNum == 1 {
			out.WriteString(strings.Join(trimAll(rec), " | "))
			out.WriteByte('\n')
			out.WriteString(strings.Repeat("-", minInt(72, len(out.String()))))
			out.WriteByte('\n')
			continue
		}
		out.WriteString(strings.Join(trimAll(rec), " | "))
		out.WriteByte('\n')
	}
	return normalizeExtractedText(out.String()), nil
}

func (jsonLoader) Name() string { return "json" }
func (jsonLoader) Supports(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".json")
}
func (jsonLoader) Load(data []byte, filename string) (string, error) {
	var anyValue any
	if err := json.Unmarshal(data, &anyValue); err != nil {
		return normalizeExtractedText(string(data)), nil
	}
	buf, err := json.MarshalIndent(anyValue, "", "  ")
	if err != nil {
		return normalizeExtractedText(string(data)), nil
	}
	return normalizeExtractedText(string(buf)), nil
}

func (zipLoader) Name() string { return "zip" }
func (zipLoader) Supports(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".zip")
}
func (zipLoader) Load(data []byte, filename string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}

	type entry struct {
		name string
		text string
	}
	entries := make([]entry, 0, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !isSupportedArchiveEntry(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		payload, _ := io.ReadAll(rc)
		rc.Close()
		loaded, err := LoadDocumentBytes(payload, f.Name)
		if err != nil {
			continue
		}
		txt := strings.TrimSpace(loaded.Text)
		if txt == "" {
			continue
		}
		entries = append(entries, entry{name: f.Name, text: txt})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	var out strings.Builder
	for _, e := range entries {
		out.WriteString("## ")
		out.WriteString(filepath.Base(e.name))
		out.WriteByte('\n')
		out.WriteString(e.text)
		out.WriteString("\n\n")
	}
	return normalizeExtractedText(out.String()), nil
}

// LoadDocumentPath loads a file, directory, or zip archive from disk and concatenates supported contents.
func LoadDocumentPath(path string) (LoadedDocument, error) {
	info, err := os.Stat(path)
	if err != nil {
		return LoadedDocument{}, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return loadDirectory(path)
	}
	data, err := osReadFile(path)
	if err != nil {
		return LoadedDocument{}, fmt.Errorf("reading file: %w", err)
	}
	return LoadDocumentBytes(data, path)
}

func loadDirectory(root string) (LoadedDocument, error) {
	var parts []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d == nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		if !isSupportedPath(path) {
			return nil
		}
		data, err := osReadFile(path)
		if err != nil {
			return nil
		}
		loaded, err := LoadDocumentBytes(data, path)
		if err != nil {
			return nil
		}
		if txt := strings.TrimSpace(loaded.Text); txt != "" {
			rel, _ := filepath.Rel(root, path)
			parts = append(parts, "## "+filepath.ToSlash(rel)+"\n"+txt)
		}
		return nil
	})
	if err != nil {
		return LoadedDocument{}, err
	}
	combined := normalizeExtractedText(strings.Join(parts, "\n\n"))
	return LoadedDocument{
		Filename: filepath.Base(root),
		RawText:  combined,
		Text:     combined,
		Metadata: BuildDocumentMetadata(filepath.Base(root), []byte(combined), combined, "directory"),
	}, nil
}

func isSupportedPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md", ".markdown", ".mkd", ".html", ".htm", ".xhtml", ".csv", ".json", ".pdf", ".docx", ".zip":
		return true
	default:
		return false
	}
}

func isSupportedArchiveEntry(name string) bool {
	return isSupportedPath(name) || filepath.Ext(name) == ""
}

func trimAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.TrimSpace(v)
	}
	return out
}

func ProfileDocument(filename, text string, meta DocumentMetadata) DocumentProfile {
	text = NormalizeDocumentText(text)
	words := strings.Fields(text)
	tokens := len(words)
	language, langConfidence := detectLanguage(text)
	class := detectDocumentClass(filename, text)
	return DocumentProfile{
		Language:           language,
		LanguageConfidence: langConfidence,
		ReadingLevel:       estimateReadingLevel(text),
		TokenEstimate:      tokens,
		Class:              class,
	}
}

func DetectAuthor(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	// Explicit labels first.
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?im)^\s*(?:author|prepared by|written by|by)\s*[:\-]\s*(.+)$`),
		regexp.MustCompile(`(?im)\b(?:author|prepared by|written by)\s+([A-Z][A-Za-z.'-]+(?:\s+[A-Z][A-Za-z.'-]+){0,3})`),
	}
	for _, rx := range patterns {
		if m := rx.FindStringSubmatch(text); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	// Email local-part heuristic.
	if m := regexp.MustCompile(`(?i)\b([A-Za-z0-9._%+-]+)@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`).FindStringSubmatch(text); len(m) > 1 {
		return strings.ReplaceAll(m[1], ".", " ")
	}
	return ""
}

func ExtractKeywords(text string, limit int) []string {
	return topTerms(text, limit, keywordStopwords(), false)
}

func ExtractEntities(text string, limit int) []string {
	// Capitalized terms and acronyms; not a true NER, but enough for metadata.
	candidates := []string{}
	candidates = append(candidates, regexp.MustCompile(`\b(?:[A-Z][a-z]+(?:\s+[A-Z][a-z]+){0,3}|[A-Z]{2,}(?:\d+)?)\b`).FindAllString(text, -1)...)
	return dedupeAndLimit(candidates, limit)
}

func ExtractTags(filename, text string) []string {
	tags := []string{}
	class := detectDocumentClass(filename, text)
	if class != "" {
		tags = append(tags, class)
	}
	for _, kw := range ExtractKeywords(text, 8) {
		tags = append(tags, kw)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if ext != "" {
		tags = append(tags, ext)
	}
	return dedupeAndLimit(tags, 12)
}

func ExtractHeadings(text string) []string {
	lines := strings.Split(NormalizeDocumentText(text), "\n")
	var headings []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			headings = append(headings, strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
			continue
		}
		if looksLikeHeading(trimmed) {
			headings = append(headings, trimmed)
		}
	}
	return dedupeAndLimit(headings, 24)
}

func ExtractPages(text string) []int {
	text = NormalizeDocumentText(text)
	if text == "" {
		return nil
	}
	pages := []int{1}
	for i, r := range text {
		if r == '\f' {
			pages = append(pages, len(pages)+1)
		}
		if i > 0 && strings.HasPrefix(text[i:], "\n--- page ") {
			pages = append(pages, len(pages)+1)
		}
	}
	return dedupeInts(pages)
}

func ExtractTables(text string) []string {
	lines := strings.Split(NormalizeDocumentText(text), "\n")
	var tables []string
	var cur []string
	for _, line := range lines {
		if strings.Count(line, "|") >= 2 || strings.Count(line, "\t") >= 2 {
			cur = append(cur, strings.TrimSpace(line))
			continue
		}
		if len(cur) > 1 {
			tables = append(tables, strings.Join(cur, "\n"))
		}
		cur = nil
	}
	if len(cur) > 1 {
		tables = append(tables, strings.Join(cur, "\n"))
	}
	return dedupeAndLimit(tables, 12)
}

func ExtractImages(text string) []string {
	var images []string
	rx := regexp.MustCompile(`(?i)!\[[^\]]*\]\(([^)]+)\)|\b(?:figure|image|diagram|screenshot|chart)\b[:\s-]+([^\n]+)`)
	for _, m := range rx.FindAllStringSubmatch(text, -1) {
		for i := 1; i < len(m); i++ {
			if strings.TrimSpace(m[i]) != "" {
				images = append(images, strings.TrimSpace(m[i]))
			}
		}
	}
	return dedupeAndLimit(images, 12)
}

func ExtractLinks(text string) []string {
	rx := regexp.MustCompile(`(?i)\bhttps?://[^\s<>"')\]]+|\[[^\]]+\]\(([^)]+)\)`)
	var links []string
	for _, m := range rx.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			links = append(links, strings.TrimSpace(m[1]))
		} else {
			links = append(links, strings.TrimSpace(m[0]))
		}
	}
	return dedupeAndLimit(links, 24)
}

func ExtractCodeBlocks(text string) []string {
	lines := strings.Split(NormalizeDocumentText(text), "\n")
	var blocks []string
	var cur []string
	in := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if in && len(cur) > 0 {
				blocks = append(blocks, strings.Join(cur, "\n"))
			}
			cur = nil
			in = !in
			continue
		}
		if in {
			cur = append(cur, line)
		}
	}
	return dedupeAndLimit(blocks, 12)
}

func HashChunkContent(text string) string {
	norm := NormalizeDocumentText(text)
	sum := fnv.New64a()
	_, _ = sum.Write([]byte(norm))
	return fmt.Sprintf("%016x", sum.Sum64())
}

func topTerms(text string, limit int, stop map[string]struct{}, requireAlpha bool) []string {
	if limit <= 0 {
		limit = 12
	}
	counts := map[string]int{}
	for _, raw := range strings.Fields(strings.ToLower(text)) {
		t := strings.Trim(raw, ".,:;!?()[]{}<>\"'`~*/\\|")
		if t == "" || len([]rune(t)) < 3 {
			continue
		}
		if requireAlpha && !hasLetter(t) {
			continue
		}
		if _, ok := stop[t]; ok {
			continue
		}
		if isNoiseTerm(t) {
			continue
		}
		counts[t]++
	}
	type kv struct {
		K string
		V int
	}
	items := make([]kv, 0, len(counts))
	for k, v := range counts {
		items = append(items, kv{k, v})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].V == items[j].V {
			return items[i].K < items[j].K
		}
		return items[i].V > items[j].V
	})
	out := make([]string, 0, minInt(limit, len(items)))
	for _, it := range items {
		out = append(out, it.K)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func dedupeAndLimit(values []string, limit int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func dedupeInts(values []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func looksLikeHeading(line string) bool {
	if len([]rune(line)) > 90 {
		return false
	}
	if strings.Count(line, " ") > 10 {
		return false
	}
	words := strings.Fields(line)
	if len(words) < 2 {
		return false
	}
	if strings.HasSuffix(line, ":") {
		return true
	}
	upper := 0
	for _, r := range line {
		if unicode.IsUpper(r) {
			upper++
		}
	}
	return upper > 0 && float64(upper)/float64(len([]rune(line))) > 0.45
}

func keywordStopwords() map[string]struct{} {
	return map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "that": {}, "this": {}, "from": {}, "have": {}, "your": {}, "are": {}, "was": {}, "were": {}, "will": {}, "would": {}, "there": {}, "their": {}, "about": {}, "into": {}, "through": {}, "using": {}, "been": {}, "shall": {}, "should": {}, "could": {}, "may": {}, "can": {}, "not": {}, "but": {}, "you": {}, "our": {}, "they": {}, "them": {},
	}
}

func isNoiseTerm(t string) bool {
	if len(t) <= 2 {
		return true
	}
	if strings.HasPrefix(t, "http") || strings.Contains(t, "://") {
		return true
	}
	return false
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func detectLanguage(text string) (string, float64) {
	if strings.TrimSpace(text) == "" {
		return "unknown", 0
	}
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return "unknown", 0
	}
	stop := 0
	for _, w := range words {
		switch w {
		case "the", "and", "of", "to", "in", "is", "for", "on", "with", "this", "that", "it", "as", "are":
			stop++
		}
	}
	ratio := float64(stop) / float64(len(words))
	if ratio > 0.03 {
		return "en", math.Min(1, 0.5+ratio*4)
	}
	return "unknown", 0.35
}

func detectDocumentClass(filename, text string) string {
	norm := strings.ToLower(filename + " " + text)
	score := map[string]int{
		"technical":     0,
		"legal":         0,
		"financial":     0,
		"research":      0,
		"documentation": 0,
		"source code":   0,
	}

	inc := func(class string, terms ...string) {
		for _, term := range terms {
			if strings.Contains(norm, term) {
				score[class]++
			}
		}
	}
	inc("legal", "agreement", "contract", "clause", "herein", "warranty", "liability", "governing law", "terms and conditions")
	inc("financial", "invoice", "balance sheet", "cash flow", "revenue", "expense", "profit", "fiscal", "financial", "budget")
	inc("research", "abstract", "references", "methodology", "experiment", "results", "dataset", "doi", "citation")
	inc("documentation", "readme", "guide", "manual", "documentation", "how to", "api", "overview")
	inc("source code", "package ", "import ", "function ", "class ", "def ", "public static", "```")
	inc("technical", "architecture", "system", "implementation", "deploy", "vector", "embedding", "api", "database", "pipeline")
	bestClass := "documentation"
	bestScore := -1
	for cls, sc := range score {
		if sc > bestScore {
			bestScore = sc
			bestClass = cls
		}
	}
	if bestScore <= 0 {
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".cs", ".rs", ".sh", ".sql":
			return "source code"
		}
	}
	return bestClass
}

func estimateReadingLevel(text string) float64 {
	text = NormalizeDocumentText(text)
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		sentences = []string{text}
	}
	avgWordsPerSentence := float64(len(words)) / float64(len(sentences))
	syllables := 0
	for _, w := range words {
		syllables += estimateSyllables(w)
	}
	if syllables == 0 {
		syllables = len(words)
	}
	// Simplified readability score mapped to 0..12-ish reading level.
	flesch := 206.835 - 1.015*avgWordsPerSentence - 84.6*(float64(syllables)/float64(len(words)))
	level := (100 - flesch) / 10
	if level < 0 {
		level = 0
	}
	if level > 12 {
		level = 12
	}
	return math.Round(level*10) / 10
}

func estimateSyllables(word string) int {
	word = strings.ToLower(strings.TrimSpace(word))
	if word == "" {
		return 0
	}
	count := 0
	prevVowel := false
	for _, r := range word {
		v := strings.ContainsRune("aeiouy", r)
		if v && !prevVowel {
			count++
		}
		prevVowel = v
	}
	if strings.HasSuffix(word, "e") && count > 1 {
		count--
	}
	if count == 0 {
		count = 1
	}
	return count
}
