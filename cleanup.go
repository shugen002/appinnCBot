package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const updatesDir = "./updates"
const updatesMaxAge = 168 * time.Hour

func cleanupOldUpdates(maxAge time.Duration) {
	entries, err := os.ReadDir(updatesDir)
	if err != nil {
		log.Printf("Error reading updates dir: %v", err)
		return
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(updatesDir, entry.Name())); err != nil {
			log.Printf("Error removing %s: %v", entry.Name(), err)
			continue
		}
		removed++
	}
	log.Printf("Cleaned %d update files older than %s", removed, maxAge)
}

// 每天本地时间 0 点清理过期的 updates json
func startUpdatesCleanup(ctx context.Context) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer := time.NewTimer(next.Sub(now))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				cleanupOldUpdates(updatesMaxAge)
			}
		}
	}()
}
