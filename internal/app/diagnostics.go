package app

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"changeme/internal/store"
)

type DiagnosticsSnapshot struct {
	DBReady                bool   `json:"dbReady"`
	GoVersion              string `json:"goVersion"`
	GOOS                   string `json:"goos"`
	GOARCH                 string `json:"goarch"`
	NumCPU                 int    `json:"numCpu"`
	TotalRAMGB             int    `json:"totalRamGb"`
	RecommendedContextSize int    `json:"recommendedContextSize"`
	MemoryAllocMB          int    `json:"memoryAllocMb"`
	MemorySysMB            int    `json:"memorySysMb"`
	ActiveGenerations      int    `json:"activeGenerations"`
	IngestActive           bool   `json:"ingestActive"`
	Collections            int    `json:"collections"`
	Chats                  int    `json:"chats"`
	Messages               int    `json:"messages"`
	Documents              int    `json:"documents"`
	ReadyDocuments         int    `json:"readyDocuments"`
	IncompleteDocuments    int    `json:"incompleteDocuments"`
	Chunks                 int    `json:"chunks"`
	MessageSources         int    `json:"messageSources"`
	DBPageSize             int64  `json:"dbPageSize"`
	DBPageCount            int64  `json:"dbPageCount"`
	DBApproxBytes          int64  `json:"dbApproxBytes"`
	CollectedAtUnix        int64  `json:"collectedAtUnix"`
}

func (s *ChatService) GetDiagnostics() (DiagnosticsSnapshot, error) {
	snap := DiagnosticsSnapshot{
		GoVersion:              runtime.Version(),
		GOOS:                   runtime.GOOS,
		GOARCH:                 runtime.GOARCH,
		NumCPU:                 runtime.NumCPU(),
		TotalRAMGB:             getTotalRAMGB(),
		RecommendedContextSize: getOptimalContextSize(),
		IngestActive:           s.IsIngesting(),
		ActiveGenerations:      s.activeGenerationCount(),
		CollectedAtUnix:        time.Now().Unix(),
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	snap.MemoryAllocMB = int(mem.Alloc / 1024 / 1024)
	snap.MemorySysMB = int(mem.Sys / 1024 / 1024)

	if bi, ok := debug.ReadBuildInfo(); ok && bi != nil && bi.GoVersion != "" {
		snap.GoVersion = bi.GoVersion
	}

	if s.DB == nil {
		return snap, nil
	}
	snap.DBReady = true

	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&snap.Collections); err != nil {
		return snap, fmt.Errorf("counting collections: %w", err)
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM chat_sessions`).Scan(&snap.Chats); err != nil {
		return snap, fmt.Errorf("counting chats: %w", err)
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM chat_messages`).Scan(&snap.Messages); err != nil {
		return snap, fmt.Errorf("counting messages: %w", err)
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&snap.Documents); err != nil {
		return snap, fmt.Errorf("counting documents: %w", err)
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM documents WHERE status = 'ready' OR status IS NULL OR status = ''`).Scan(&snap.ReadyDocuments); err != nil {
		return snap, fmt.Errorf("counting ready documents: %w", err)
	}
	if incomplete, err := store.GetIncompleteDocuments(s.DB); err == nil {
		snap.IncompleteDocuments = len(incomplete)
	} else {
		return snap, fmt.Errorf("counting incomplete documents: %w", err)
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&snap.Chunks); err != nil {
		return snap, fmt.Errorf("counting chunks: %w", err)
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM chat_message_sources`).Scan(&snap.MessageSources); err != nil {
		return snap, fmt.Errorf("counting sources: %w", err)
	}
	var pageCount, pageSize int64
	if err := s.DB.QueryRow(`PRAGMA page_count`).Scan(&pageCount); err == nil {
		snap.DBPageCount = pageCount
	}
	if err := s.DB.QueryRow(`PRAGMA page_size`).Scan(&pageSize); err == nil {
		snap.DBPageSize = pageSize
	}
	if snap.DBPageCount > 0 && snap.DBPageSize > 0 {
		snap.DBApproxBytes = snap.DBPageCount * snap.DBPageSize
	}
	return snap, nil
}

func (s *ChatService) activeGenerationCount() int {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	return len(s.cancelFuncs)
}
