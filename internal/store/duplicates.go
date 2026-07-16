package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// DuplicateMatch describes a likely duplicate document found during staging.
type DuplicateMatch struct {
	DocumentID   int64   `json:"documentId"`
	CollectionID int64   `json:"collectionId"`
	Filename     string  `json:"filename"`
	Kind         string  `json:"kind"`
	Score        float64 `json:"score"`
	Reason       string  `json:"reason"`
}

// FindPotentialDuplicates compares the incoming document against existing documents.
// The function is intentionally conservative: it prefers false negatives over false positives.
func FindPotentialDuplicates(db *sql.DB, collectionID int64, filename, contentHash string, chunkHashes []string, limit int) ([]DuplicateMatch, error) {
	if limit <= 0 {
		limit = 10
	}

	seen := map[int64]struct{}{}
	var matches []DuplicateMatch

	if contentHash != "" {
		if exact, err := GetDocumentByHash(db, contentHash, collectionID); err == nil && exact != nil {
			matches = append(matches, DuplicateMatch{
				DocumentID:   exact.ID,
				CollectionID: exact.CollectionID,
				Filename:     exact.Filename,
				Kind:         "identical",
				Score:        1.0,
				Reason:       "same content hash",
			})
			seen[exact.ID] = struct{}{}
		}
	}

	if len(chunkHashes) == 0 {
		return matches, nil
	}

	docs, err := getDocumentsForDuplicateCheck(db, collectionID)
	if err != nil {
		return matches, err
	}

	incoming := make([]string, 0, len(chunkHashes))
	for _, h := range chunkHashes {
		h = strings.TrimSpace(h)
		if h != "" {
			incoming = append(incoming, h)
		}
	}
	if len(incoming) == 0 {
		return matches, nil
	}

	for _, doc := range docs {
		if _, ok := seen[doc.ID]; ok {
			continue
		}
		if strings.EqualFold(doc.Filename, filename) && doc.Status == DocStatusReady && contentHash != "" && doc.Hash == contentHash {
			continue
		}
		cands, err := GetChunksByDocument(db, doc.ID)
		if err != nil || len(cands) == 0 {
			continue
		}
		existing := TopChunkHashes(cands, 512)
		if len(existing) == 0 {
			continue
		}
		score := chunkOverlapScore(incoming, existing)
		if score < 0.55 {
			continue
		}
		kind := "partial"
		reason := fmt.Sprintf("%.0f%% chunk overlap", score*100)
		if strings.EqualFold(doc.Filename, filename) {
			kind = "renamed"
			reason = "matching chunk profile with same filename"
		}
		matches = append(matches, DuplicateMatch{
			DocumentID:   doc.ID,
			CollectionID: doc.CollectionID,
			Filename:     doc.Filename,
			Kind:         kind,
			Score:        score,
			Reason:       reason,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].DocumentID < matches[j].DocumentID
		}
		return matches[i].Score > matches[j].Score
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func chunkOverlapScore(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := map[string]struct{}{}
	for _, s := range a {
		set[s] = struct{}{}
	}
	match := 0
	for _, s := range b {
		if _, ok := set[s]; ok {
			match++
		}
	}
	denom := maxInt(1, len(set))
	return float64(match) / float64(denom)
}

func getDocumentsForDuplicateCheck(db *sql.DB, collectionID int64) ([]Document, error) {
	rows, err := db.Query(`
		SELECT d.id, d.collection_id, d.filename, COALESCE(d.summary,''), COALESCE(d.hash,''), COALESCE(d.content,''),
		       COALESCE(d.source_type,''), COALESCE(d.source_size_bytes,0), COALESCE(d.word_count,0), COALESCE(d.line_count,0), COALESCE(d.character_count,0), COALESCE(d.paragraph_count,0), COALESCE(d.title,''),
		       d.created_at,
		       COALESCE(c.cnt, 0),
		       COALESCE(d.status, 'ready'), COALESCE(d.expected_chunks, 0), COALESCE(d.batch_id, ''),
		       COALESCE(d.error_message, ''), COALESCE(d.updated_at, 0)
		FROM documents d
		LEFT JOIN (SELECT document_id, COUNT(*) AS cnt FROM chunks GROUP BY document_id) c ON c.document_id = d.id
		WHERE d.collection_id = ?
		ORDER BY d.updated_at DESC, d.id DESC
	`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocumentRows(rows)
}
