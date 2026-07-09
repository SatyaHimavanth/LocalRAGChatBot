package ingest

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

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
		return string(data), nil
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

// ════════════════════════════════════════════════
//  PDF Parser (using ledongthuc/pdf)
// ════════════════════════════════════════════════

func parsePDFBytes(data []byte) (string, error) {
	// Write to temp file for the pdf library (it reads from io.ReaderSeeker)
	tmpFile, err := os.CreateTemp("", "rag-*.pdf")
	if err != nil {
		return fallbackPDFExtract(data), nil
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fallbackPDFExtract(data), nil
	}
	tmpFile.Close()

	f, err := os.Open(tmpFile.Name())
	if err != nil {
		return fallbackPDFExtract(data), nil
	}
	defer f.Close()

	r, err := pdf.NewReader(f, int64(len(data)))
	if err != nil {
		return fallbackPDFExtract(data), nil
	}

	var buf strings.Builder
	totalPages := r.NumPage()
	for i := 1; i <= totalPages; i++ {
		p := r.Page(i)
		text, err := p.GetPlainText(nil)
		if err == nil {
			buf.WriteString(text)
			buf.WriteByte('\n')
		}
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return fallbackPDFExtract(data), nil
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
	// Don't forget last segment
	if current.Len() > 5 {
		s := strings.TrimSpace(current.String())
		if isReadableText(s) {
			buf.WriteString(s)
		}
	}

	return strings.TrimSpace(buf.String())
}

func isReadableText(s string) bool {
	if len(s) < 10 {
		return false
	}

	// Count ratio of letters, spaces, common punctuation
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

	// Must be mostly letters, with reasonable alphanumeric ratio
	return letterRatio > 0.4 && alnumRatio > 0.5
}

// ════════════════════════════════════════════════
//  DOCX Parser
// ════════════════════════════════════════════════

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

type docxBody struct {
	XMLName xml.Name  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main body"`
	Paras   []docxPara `xml:"p"`
}

type docxPara struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text string `xml:"t"`
}

func extractDOCXText(xmlContent string) string {
	var body docxBody
	if err := xml.Unmarshal([]byte(xmlContent), &body); err != nil {
		return stripXMLTags(xmlContent)
	}

	var buf strings.Builder
	for _, p := range body.Paras {
		for _, r := range p.Runs {
			buf.WriteString(r.Text)
		}
		buf.WriteByte('\n')
	}
	return strings.TrimSpace(buf.String())
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
