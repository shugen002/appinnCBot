package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	appmodels "github.com/shugen002/appinnCbot/models"
	"github.com/shugen002/appinnCbot/storage"
)

var seenRepo appmodels.SeenRepository

var regexs = []regexp.Regexp{}

var regexsLock *sync.RWMutex

// Send any text message to the bot after the bot has been started
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment
	transport.TLSHandshakeTimeout = 30 * time.Second

	// Create a new HTTP client with a proxy
	opts := []bot.Option{
		bot.WithHTTPClient(30*time.Second, &http.Client{
			Timeout:   35 * time.Second,
			Transport: transport,
		}),
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			tgmodels.AllowedUpdateMessage,
			tgmodels.AllowedUpdateEditedMessage,
		}),
		bot.WithDefaultHandler(func(ctx context.Context, bot *bot.Bot, update *tgmodels.Update) {
			jsonData, err := json.MarshalIndent(update, "", "  ")
			if err != nil {
				log.Printf("Error marshalling update %d: %v", update.ID, err)
				return
			}
			// write the update to a file at ./updates/%d.json
			filePath := fmt.Sprintf("./updates/%d.json", update.ID)
			if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
				log.Printf("Error writing update %d to file: %v", update.ID, err)
				return
			}

			if update.Message != nil {
				createMessageHandler(ctx, bot, update.Message)
			} else if update.EditedMessage != nil {
				editMessageHandler(ctx, bot, update.EditedMessage)
			}
		}),
	}

	b, err := bot.New(os.Getenv("BOT_TOKEN"), opts...)
	if err != nil {
		panic(err)
	}
	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "appinn.db"
	}
	db, err := storage.OpenSQLite(dbPath)
	if err != nil {
		log.Panicf("Error opening sqlite database: %v", err)
	}
	defer db.Close()
	seenRepo = storage.NewSQLiteSeenRepository(db)

	if _, err := os.Stat("regexs.hujson"); err == nil {
		data, err := os.ReadFile("regexs.hujson")
		if err != nil {
			log.Panicf("Error reading regexs.hujson: %v", err)
		} else {
			var patterns []string
			if err := json.Unmarshal(data, &patterns); err != nil {
				log.Panicf("Error unmarshalling regexs.hujson: %v", err)
			} else {
				for _, pattern := range patterns {
					re, err := regexp.Compile(pattern)
					if err != nil {
						log.Printf("Error compiling regex pattern %s: %v", pattern, err)
					} else {
						regexs = append(regexs, *re)
					}
				}
			}
		}
	}
	regexsLock = &sync.RWMutex{}

	// watch regexs.hujson for changes and reload regexs
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Panicf("Error creating file watcher: %v", err)
	}
	defer watcher.Close()
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("Detected change in regexs.hujson, reloading...")
					data, err := os.ReadFile("regexs.hujson")
					if err != nil {
						log.Printf("Error reading regexs.hujson: %v", err)
						continue
					}
					var patterns []string
					if err := json.Unmarshal(data, &patterns); err != nil {
						log.Printf("Error unmarshalling regexs.hujson: %v", err)
						continue
					}
					var newRegexs []regexp.Regexp
					for _, pattern := range patterns {
						re, err := regexp.Compile(pattern)
						if err != nil {
							log.Printf("Error compiling regex pattern %s: %v", pattern, err)
						} else {
							newRegexs = append(newRegexs, *re)
						}
					}
					regexsLock.Lock()
					regexs = newRegexs
					regexsLock.Unlock()
					log.Printf("Reloaded %d regex patterns", len(newRegexs))
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("File watcher error: %v", err)
			case <-ctx.Done():
				return
			}
		}
	}()
	if err := watcher.Add("regexs.hujson"); err != nil {
		log.Panicf("Error adding regexs.hujson to watcher: %v", err)
	}
	me, err := b.GetMe(context.Background())
	if err != nil {
		log.Fatalf("Error getting bot info: %v", err)
	}
	log.Printf("Bot started as %s (ID: %d)", me.Username, me.ID)

	b.Start(ctx)
}

func createMessageHandler(ctx context.Context, b *bot.Bot, m *tgmodels.Message) {
	if m.From == nil {
		log.Printf("Received message No.%d with no From field", m.ID)
		return
	}

	// chat type should be group or supergroup
	if m.Chat.Type != tgmodels.ChatTypeGroup && m.Chat.Type != tgmodels.ChatTypeSupergroup {
		log.Printf("Received message No.%d with unsupported chat type: %s", m.ID, m.Chat.Type)
		return
	}

	if m.From == nil || m.From.ID == 0 || m.From.ID == b.ID() || m.From.ID == 777000 {
		return
	}

	if isPingCommand(m.Text) {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: m.Chat.ID,
			Text:   "pong",
		})
		if err != nil {
			log.Printf("Error replying pong to user %d in chat %d: %v", m.From.ID, m.Chat.ID, err)
		}
		return
	}

	count, err := seenRepo.GetCount(ctx, m.Chat.ID, m.From.ID)
	if err != nil {
		log.Printf("Error loading seen count for user %d in chat %d: %v", m.From.ID, m.Chat.ID, err)
		return
	}

	if count > 0 {
		log.Printf("Message No.%d from user %d in chat %d has count %d, ignoring", m.ID, m.From.ID, m.Chat.ID, count)
		return
	}

	if usernameCheck(m) ||
		viaBotCheck(m) ||
		stickerCheck(m) ||
		simpleEmojiCheck(m) ||
		mentionCheck(m) ||
		contactCheck(m) ||
		linkCheck(m) ||
		meaninglessCheck(m) {
		success, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    m.Chat.ID,
			MessageID: m.ID,
		})
		if err != nil {
			log.Printf("Error deleting message No.%d from user %d: %v", m.ID, m.From.ID, err)
			return
		} else if !success {
			log.Printf("Failed to delete message No.%d from user %d", m.ID, m.From.ID)
			return
		}
		log.Printf("Deleted message No.%d from user %d in chat %d", m.ID, m.From.ID, m.Chat.ID)
		return
	}

	if strings.HasPrefix(m.Text, "/") {
		// if the message is a command, ignore it
		log.Printf("Ignoring command message No.%d from user %d in chat %d: %s", m.ID, m.From.ID, m.Chat.ID, m.Text)
		return
	}

	log.Printf("Received message No.%d from user %d %s %s (%s) in chat %d: %s", m.ID, m.From.ID, m.From.FirstName, m.From.LastName, m.From.Username, m.Chat.ID, m.Text)
	newCount, err := seenRepo.EnsureAtLeast(ctx, m.Chat.ID, m.From.ID, 1)
	if err != nil {
		log.Printf("Error updating seen count for user %d in chat %d: %v", m.From.ID, m.Chat.ID, err)
		return
	}
	log.Printf("Updated user %d in chat %d count to %d", m.From.ID, m.Chat.ID, newCount)
}

func editMessageHandler(ctx context.Context, b *bot.Bot, m *tgmodels.Message) {
	if m.From == nil {
		log.Printf("Received message No.%d with no From field", m.ID)
		return
	}

	// chat type should be group or supergroup
	if m.Chat.Type != tgmodels.ChatTypeGroup && m.Chat.Type != tgmodels.ChatTypeSupergroup {
		log.Printf("Received message No.%d with unsupported chat type: %s", m.ID, m.Chat.Type)
		return
	}

	if m.From == nil || m.From.ID == 0 || m.From.ID == b.ID() || m.From.ID == 777000 {
		return
	}

	count, err := seenRepo.GetCount(ctx, m.Chat.ID, m.From.ID)
	if err != nil {
		log.Printf("Error loading seen count for edited message user %d in chat %d: %v", m.From.ID, m.Chat.ID, err)
		return
	}

	if count > 1 {
		return
	}

	if usernameCheck(m) ||
		viaBotCheck(m) ||
		stickerCheck(m) ||
		simpleEmojiCheck(m) ||
		mentionCheck(m) ||
		contactCheck(m) ||
		linkCheck(m) ||
		meaninglessCheck(m) {
		success, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    m.Chat.ID,
			MessageID: m.ID,
		})
		if err != nil {
			log.Printf("Error deleting message No.%d from user %d : %v", m.ID, m.From.ID, err)
			return
		} else if !success {
			log.Printf("Failed to delete message No.%d from user %d", m.ID, m.From.ID)
			return
		}
		newCount, err := seenRepo.Decrement(ctx, m.Chat.ID, m.From.ID)
		if err != nil {
			log.Printf("Error decrementing seen count for user %d in chat %d: %v", m.From.ID, m.Chat.ID, err)
			return
		}
		log.Printf("Deleted message No.%d from user %d in chat %d, count now %d", m.ID, m.From.ID, m.Chat.ID, newCount)
	}
}

func isPingCommand(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return false
	}

	command := strings.ToLower(parts[0])
	if command == "/ping" {
		return true
	}

	return strings.HasPrefix(command, "/ping@")
}
