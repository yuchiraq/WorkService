package security

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logMutex sync.Mutex
	logFile  = "storage/security.log"
)

func LogEvent(event, details string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s | %s | %s\n", time.Now().Format(time.RFC3339), event, details)
}

func ReadRecent(limit int) []string {
	logMutex.Lock()
	defer logMutex.Unlock()

	content, err := os.ReadFile(logFile)
	if err != nil {
		return []string{}
	}
	lines := []string{}
	start := 0
	for i, b := range content {
		if b == '\n' {
			if i > start {
				lines = append(lines, string(content[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, string(content[start:]))
	}
	if limit <= 0 || len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}
