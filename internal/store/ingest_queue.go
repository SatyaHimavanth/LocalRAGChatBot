package store

import (
	"database/sql"
	"time"
)

type IngestionJob struct {
	ID                 int64  `json:"id"`
	BatchID            string `json:"batchId"`
	CollectionID       int64  `json:"collectionId"`
	Status             string `json:"status"`
	Stage              string `json:"stage"`
	ProgressPct        int    `json:"progressPct"`
	TotalDocuments     int    `json:"totalDocuments"`
	CompletedDocuments int    `json:"completedDocuments"`
	FailedDocuments    int    `json:"failedDocuments"`
	Message            string `json:"message"`
	RetryCount         int    `json:"retryCount"`
	CreatedAt          int64  `json:"createdAt"`
	UpdatedAt          int64  `json:"updatedAt"`
	PausedAt           int64  `json:"pausedAt"`
	ResumedAt          int64  `json:"resumedAt"`
	CancelledAt        int64  `json:"cancelledAt"`
}

type IngestionLog struct {
	ID           int64   `json:"id"`
	JobID        int64   `json:"jobId"`
	DocumentID   int64   `json:"documentId"`
	CollectionID int64   `json:"collectionId"`
	BatchID      string  `json:"batchId"`
	Level        string  `json:"level"`
	Stage        string  `json:"stage"`
	Message      string  `json:"message"`
	DurationMs   int64   `json:"durationMs"`
	MemoryBytes  int64   `json:"memoryBytes"`
	Throughput   float64 `json:"throughput"`
	CreatedAt    int64   `json:"createdAt"`
}

func CreateIngestionJob(db *sql.DB, batchID string, collectionID int64, totalDocs int) (int64, error) {
	now := time.Now().Unix()
	res, err := db.Exec(`INSERT INTO ingestion_jobs (batch_id, collection_id, status, stage, progress_pct, total_documents, completed_documents, failed_documents, message, retry_count, created_at, updated_at) VALUES (?, ?, 'queued', 'loading', 0, ?, 0, 0, '', 0, ?, ?)`, batchID, collectionID, totalDocs, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateIngestionJob(db *sql.DB, jobID int64, status, stage string, progress, completed, failed int, message string) error {
	_, err := db.Exec(`UPDATE ingestion_jobs SET status = ?, stage = ?, progress_pct = ?, completed_documents = ?, failed_documents = ?, message = ?, updated_at = ? WHERE id = ?`,
		status, stage, progress, completed, failed, message, time.Now().Unix(), jobID)
	return err
}

func MarkIngestionPaused(db *sql.DB, jobID int64, message string) error {
	_, err := db.Exec(`UPDATE ingestion_jobs SET status = 'paused', message = ?, paused_at = ?, updated_at = ? WHERE id = ?`, message, time.Now().Unix(), time.Now().Unix(), jobID)
	return err
}

func MarkIngestionResumed(db *sql.DB, jobID int64, message string) error {
	_, err := db.Exec(`UPDATE ingestion_jobs SET status = 'running', message = ?, resumed_at = ?, updated_at = ? WHERE id = ?`, message, time.Now().Unix(), time.Now().Unix(), jobID)
	return err
}

func MarkIngestionCancelled(db *sql.DB, jobID int64, message string) error {
	_, err := db.Exec(`UPDATE ingestion_jobs SET status = 'cancelled', message = ?, cancelled_at = ?, updated_at = ? WHERE id = ?`, message, time.Now().Unix(), time.Now().Unix(), jobID)
	return err
}

func RetryIngestionJob(db *sql.DB, jobID int64) error {
	_, err := db.Exec(`UPDATE ingestion_jobs SET status = 'retrying', retry_count = retry_count + 1, updated_at = ? WHERE id = ?`, time.Now().Unix(), jobID)
	return err
}

func GetIngestionJobs(db *sql.DB, collectionID int64) ([]IngestionJob, error) {
	rows, err := db.Query(`SELECT id, batch_id, collection_id, status, stage, progress_pct, total_documents, completed_documents, failed_documents, message, retry_count, created_at, updated_at, paused_at, resumed_at, cancelled_at FROM ingestion_jobs WHERE collection_id = ? ORDER BY updated_at DESC`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IngestionJob
	for rows.Next() {
		var j IngestionJob
		if err := rows.Scan(&j.ID, &j.BatchID, &j.CollectionID, &j.Status, &j.Stage, &j.ProgressPct, &j.TotalDocuments, &j.CompletedDocuments, &j.FailedDocuments, &j.Message, &j.RetryCount, &j.CreatedAt, &j.UpdatedAt, &j.PausedAt, &j.ResumedAt, &j.CancelledAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func GetIngestionJobByBatchID(db *sql.DB, batchID string) (*IngestionJob, error) {
	var j IngestionJob
	err := db.QueryRow(`SELECT id, batch_id, collection_id, status, stage, progress_pct, total_documents, completed_documents, failed_documents, message, retry_count, created_at, updated_at, paused_at, resumed_at, cancelled_at FROM ingestion_jobs WHERE batch_id = ? ORDER BY updated_at DESC LIMIT 1`, batchID).
		Scan(&j.ID, &j.BatchID, &j.CollectionID, &j.Status, &j.Stage, &j.ProgressPct, &j.TotalDocuments, &j.CompletedDocuments, &j.FailedDocuments, &j.Message, &j.RetryCount, &j.CreatedAt, &j.UpdatedAt, &j.PausedAt, &j.ResumedAt, &j.CancelledAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func AddIngestionLog(db *sql.DB, jobID, docID, collectionID int64, batchID, level, stage, message string, durationMs, memoryBytes int64, throughput float64) error {
	_, err := db.Exec(`INSERT INTO ingestion_logs (job_id, document_id, collection_id, batch_id, level, stage, message, duration_ms, memory_bytes, throughput, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID, docID, collectionID, batchID, level, stage, message, durationMs, memoryBytes, throughput, time.Now().Unix())
	return err
}

func GetIngestionLogs(db *sql.DB, jobID int64, limit int) ([]IngestionLog, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := db.Query(`SELECT id, job_id, document_id, collection_id, batch_id, level, stage, message, duration_ms, memory_bytes, throughput, created_at FROM ingestion_logs WHERE job_id = ? ORDER BY created_at DESC LIMIT ?`, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IngestionLog
	for rows.Next() {
		var l IngestionLog
		if err := rows.Scan(&l.ID, &l.JobID, &l.DocumentID, &l.CollectionID, &l.BatchID, &l.Level, &l.Stage, &l.Message, &l.DurationMs, &l.MemoryBytes, &l.Throughput, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
