package ingest

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// ParseFile extracts text content from a file based on its extension.
func ParseFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	return ParseFileBytes(data, path)
}

// ParseFileBytes extracts text from file bytes based on extension and filename.
func ParseFileBytes(data []byte, filename string) (string, error) {
	ext := strings.ToLower(extOnly(filename))
	switch ext {
	case ".txt":
		return normalizeExtractedText(string(data)), nil
	case ".pdf":
		return parsePDFBytes(data)
	case ".docx":
		return parseDOCXBytes(data)
	default:
		return "", fmt.Errorf("unsupported format: %s (supported: .txt, .pdf, .docx)", ext)
	}
}

func extOnly(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return ""
}

func parsePDFBytes(data []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "rag-*.pdf")
	if err != nil {
		return normalizeExtractedText(fallbackPDFExtract(data)), nil
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return normalizeExtractedText(fallbackPDFExtract(data)), nil
	}
	tmpFile.Close()

	f, err := os.Open(tmpFile.Name())
	if err != nil {
		return normalizeExtractedText(fallbackPDFExtract(data)), nil
	}
	defer f.Close()

	r, err := pdf.NewReader(f, int64(len(data)))
	if err != nil {
		return normalizeExtractedText(fallbackPDFExtract(data)), nil
	}

	var buf strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		text, err := p.GetPlainText(nil)
		if err == nil && strings.TrimSpace(text) != "" {
			buf.WriteString(text)
			buf.WriteString("\n\n")
			continue
		}

		// Fallback to row-based extraction
		rows, err := p.GetTextByRow()
		if err == nil && len(rows) > 0 {
			for _, row := range rows {
				var rowBuf strings.Builder
				for _, text := range row.Content {
					rowBuf.WriteString(text.S)
				}
				line := strings.TrimSpace(rowBuf.String())
				if line != "" {
					buf.WriteString(line)
					buf.WriteByte('\n')
				}
			}
			buf.WriteByte('\n')
		}
	}

	result := normalizeExtractedText(buf.String())
	if result == "" {
		return normalizeExtractedText(fallbackPDFExtract(data)), nil
	}
	return result, nil
}

// fallbackPDFExtract tries to extract readable ASCII text from PDF binary data.
func fallbackPDFExtract(data []byte) string {
	var buf strings.Builder
	var current strings.Builder

	for _, b := range data {
		if b >= 32 && b <= 126 {
			current.WriteByte(b)
		} else {
			if current.Len() > 5 {
				s := strings.TrimSpace(current.String())
				if isReadableText(s) {
					buf.WriteString(s)
					buf.WriteByte(' ')
				}
			}
			current.Reset()
		}
	}
	if current.Len() > 5 {
		s := strings.TrimSpace(current.String())
		if isReadableText(s) {
			buf.WriteString(s)
		}
	}

	return normalizeExtractedText(buf.String())
}

func isReadableText(s string) bool {
	if len(s) < 10 {
		return false
	}

	letters := 0
	alnum := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == ' ' {
			letters++
			alnum++
		} else if r >= '0' && r <= '9' {
			alnum++
		}
	}

	if len(s) == 0 {
		return false
	}
	letterRatio := float64(letters) / float64(len(s))
	alnumRatio := float64(alnum) / float64(len(s))
	return letterRatio > 0.4 && alnumRatio > 0.5
}

func parseDOCXBytes(data []byte) (string, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("opening docx: %w", err)
	}

	var docFile *zip.File
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", fmt.Errorf("word/document.xml not found")
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", fmt.Errorf("opening document.xml: %w", err)
	}
	defer rc.Close()

	var xmlBuf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, err := rc.Read(tmp)
		if n > 0 {
			xmlBuf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}

	return extractDOCXText(xmlBuf.String()), nil
}

type docxDocument struct {
	XMLName xml.Name   `xml:"document"`
	Body    docxBody   `xml:"body"`
}

type docxBody struct {
	Paras []docxPara `xml:"p"`
}

type docxPara struct {
	PPr   *docxPPr   `xml:"pPr"`
	Runs  []docxRun  `xml:"r"`
}

type docxPPr struct {
	PStyle *docxPStyle `xml:"pStyle"`
}

type docxPStyle struct {
	Val string `xml:"val,attr"`
}

type docxRun struct {
	RPr  *docxRPr `xml:"rPr"`
	Text string   `xml:"t"`
}

type docxRPr struct {
	Bold *struct{} `xml:"b"`
}

func extractDOCXText(xmlContent string) string {
	var doc docxDocument
	if err := xml.Unmarshal([]byte(xmlContent), &doc); err != nil {
		return normalizeExtractedText(stripXMLTags(xmlContent))
	}

	var buf strings.Builder
	for _, p := range doc.Body.Paras {
		var style string
		if p.PPr != nil && p.PPr.PStyle != nil {
			style = p.PPr.PStyle.Val
		}

		var runTexts []string
		for _, r := range p.Runs {
			t := r.Text
			if r.RPr != nil && r.RPr.Bold != nil {
				trimmed := strings.TrimSpace(t)
				if trimmed != "" {
					leadingSpace := ""
					trailingSpace := ""
					if strings.HasPrefix(t, " ") {
						leadingSpace = " "
					}
					if strings.HasSuffix(t, " ") {
						trailingSpace = " "
					}
					t = leadingSpace + "**" + trimmed + "**" + trailingSpace
				}
			}
			runTexts = append(runTexts, t)
		}
		text := strings.Join(runTexts, "")

		// If style is heading, prefix it
		if strings.HasPrefix(strings.ToLower(style), "heading") {
			level := "##" // default
			if strings.HasSuffix(style, "1") {
				level = "#"
			} else if strings.HasSuffix(style, "2") {
				level = "##"
			} else if strings.HasSuffix(style, "3") {
				level = "###"
			} else if strings.HasSuffix(style, "4") {
				level = "####"
			}
			text = level + " " + strings.TrimSpace(text)
		} else if strings.EqualFold(style, "sourcecode") || strings.Contains(strings.ToLower(style), "code") {
			text = "```\n" + text + "\n```"
		}

		buf.WriteString(text)
		buf.WriteByte('\n')
	}
	return normalizeExtractedText(buf.String())
}

func stripXMLTags(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	result := strings.Fields(buf.String())
	return strings.Join(result, " ")
}

func normalizeExtractedText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")

	shortLines := 0
	nonEmpty := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++
		if len([]rune(line)) <= 3 {
			shortLines++
		}
	}
	charByChar := nonEmpty >= 12 && float64(shortLines)/float64(nonEmpty) > 0.65

	var paragraphs []string
	var current strings.Builder
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			paragraphs = append(paragraphs, strings.Join(strings.Fields(text), " "))
		}
		current.Reset()
	}

	for _, rawLine := range lines {
		line := strings.Join(strings.Fields(strings.TrimSpace(rawLine)), " ")
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			paragraphs = append(paragraphs, line)
			continue
		}
		if current.Len() == 0 {
			current.WriteString(line)
			continue
		}
		cur := current.String()
		if strings.HasSuffix(cur, "-") {
			current.Reset()
			current.WriteString(strings.TrimSuffix(cur, "-"))
			current.WriteString(line)
			continue
		}
		if charByChar && len([]rune(line)) <= 3 && shouldJoinTightly(cur, line) {
			current.WriteString(line)
			continue
		}
		if startsNewListItem(line) {
			flush()
			current.WriteString(line)
			continue
		}
		current.WriteByte(' ')
		current.WriteString(line)
	}
	flush()
	return strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
}

func shouldJoinTightly(previous, next string) bool {
	if previous == "" || next == "" {
		return false
	}
	prevRunes := []rune(previous)
	nextRunes := []rune(next)
	return unicode.IsLetter(prevRunes[len(prevRunes)-1]) && unicode.IsLetter(nextRunes[0])
}

func startsNewListItem(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ") {
		return true
	}
	if len(line) < 3 {
		return false
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(line) && (line[i] == '.' || line[i] == ')') && line[i+1] == ' '
}
