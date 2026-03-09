package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/shugen002/appinnCbot/storage"
)

func main() {
	jsonPath := flag.String("json", "seens.json", "path to legacy seens json file")
	dbPath := flag.String("db", "appinn.db", "path to sqlite db file")
	flag.Parse()

	data, err := os.ReadFile(*jsonPath)
	if err != nil {
		log.Fatalf("read %s: %v", *jsonPath, err)
	}

	legacy := make(map[int64]map[int64]int64)
	if err := json.Unmarshal(data, &legacy); err != nil {
		log.Fatalf("unmarshal %s: %v", *jsonPath, err)
	}

	db, err := storage.OpenSQLite(*dbPath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	repo := storage.NewSQLiteSeenRepository(db)
	ctx := context.Background()
	migrated := 0

	for chatID, users := range legacy {
		for userID := range users {
			if _, err := repo.EnsureAtLeast(ctx, chatID, userID, 1); err != nil {
				log.Fatalf("migrate chat=%d user=%d: %v", chatID, userID, err)
			}
			migrated++
		}
	}

	log.Printf("Migration complete: %d user records imported into %s", migrated, *dbPath)
}
