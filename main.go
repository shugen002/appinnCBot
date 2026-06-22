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

var usernameRegexes = []regexp.Regexp{}

var regexsLock = &sync.RWMutex{}

type appConfig struct {
	UsernamePatterns    []string `json:"username_patterns"`
	MeaninglessPatterns []string `json:"meaningless_patterns"`
	WhitelistDomains    []string `json:"whitelist_domains"`
}

func compilePatterns(patterns []string) []regexp.Regexp {
	compiled := make([]regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("Error compiling regex pattern %s: %v", pattern, err)
			continue
		}
		compiled = append(compiled, *re)
	}
	return compiled
}

func loadAppConfig(path string) ([]regexp.Regexp, []regexp.Regexp, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}

	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		var legacyPatterns []string
		if legacyErr := json.Unmarshal(data, &legacyPatterns); legacyErr != nil {
			return nil, nil, nil, err
		}
		cfg.UsernamePatterns = legacyPatterns
	}

	return compilePatterns(cfg.UsernamePatterns), compilePatterns(cfg.MeaninglessPatterns), cfg.WhitelistDomains, nil
}

func replaceAppConfig(usernameRegexesCfg []regexp.Regexp, meaninglessRegexesCfg []regexp.Regexp, whitelist []string) {
	regexsLock.Lock()
	defer regexsLock.Unlock()

	usernameRegexes = usernameRegexesCfg
	meaninglessRegexes = meaninglessRegexesCfg
	whitelistDomain = append([]string(nil), whitelist...)
}

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
		log.Panic("SQLITE_PATH is required")
	}
	db, err := storage.OpenSQLite(dbPath)
	if err != nil {
		log.Panicf("Error opening sqlite database: %v", err)
	}
	defer db.Close()
	seenRepo = storage.NewSQLiteSeenRepository(db)

	usernameRegexesCfg, meaninglessRegexesCfg, whitelist, err := loadAppConfig("config.json")
	if err != nil {
		log.Panicf("Error loading config.json: %v", err)
	}
	replaceAppConfig(usernameRegexesCfg, meaninglessRegexesCfg, whitelist)

	// watch config.json for changes and reload config
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
					log.Println("Detected change in config.json, reloading...")
					usernameRegexesCfg, meaninglessRegexesCfg, whitelist, err := loadAppConfig("config.json")
					if err != nil {
						log.Printf("Error loading config.json: %v", err)
						continue
					}
					replaceAppConfig(usernameRegexesCfg, meaninglessRegexesCfg, whitelist)
					log.Printf("Reloaded %d username regex patterns, %d meaningless regex patterns, and %d whitelist domains", len(usernameRegexesCfg), len(meaninglessRegexesCfg), len(whitelist))
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
	if err := watcher.Add("config.json"); err != nil {
		log.Panicf("Error adding config.json to watcher: %v", err)
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

	if m.NewChatMembers != nil || m.LeftChatMember != nil {
		// ignore service messages
		log.Printf("Ignoring service message No.%d from user %d in chat %d", m.ID, m.From.ID, m.Chat.ID)
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
		// and increment the count by 1
		_, err := seenRepo.Increment(ctx, m.Chat.ID, m.From.ID)
		if err != nil {
			log.Printf("Error incrementing seen count for user %d in chat %d: %v", m.From.ID, m.Chat.ID, err)
			return
		}
		return
	}

	if strings.HasPrefix(m.Text, "/start ") {
		// has 3 mention without info
		count := 0
		for _, entity := range m.Entities {
			if entity.Type == tgmodels.MessageEntityTypeMention && entity.User == nil {
				count++
			}
		}
		if count >= 3 {
			b.BanChatMember(ctx, &bot.BanChatMemberParams{
				ChatID:         m.Chat.ID,
				UserID:         m.From.ID,
				RevokeMessages: true,
			})
			return
		}
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
