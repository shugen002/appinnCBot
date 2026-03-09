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
	"github.com/go-telegram/bot/models"
)

var seensIdMap = make(map[int64]map[int64]int64)

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
			models.AllowedUpdateMessage,
			models.AllowedUpdateEditedMessage,
		}),
		bot.WithDefaultHandler(func(ctx context.Context, bot *bot.Bot, update *models.Update) {
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

	// read seens from file
	if _, err := os.Stat("seens.json"); err == nil {
		data, err := os.ReadFile("seens.json")
		if err != nil {
			log.Panicf("Error reading seens.json: %v", err)
		} else {
			if err := json.Unmarshal(data, &seensIdMap); err != nil {
				log.Panicf("Error unmarshalling seens.json: %v", err)
			}
		}
	} else if !os.IsNotExist(err) {
		log.Panicf("Error checking seens.json: %v", err)
	}
	// save seens to file on exit
	defer saveSeen()

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

	// save seens to file every 1 minutes
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ticker.C:
				saveSeen()
			case <-ctx.Done():
				return
			}
		}
	}()

	saveSeen() // Save initial state
	me, err := b.GetMe(context.Background())
	if err != nil {
		log.Fatalf("Error getting bot info: %v", err)
	}
	log.Printf("Bot started as %s (ID: %d)", me.Username, me.ID)

	b.Start(ctx)
}

func saveSeen() {
	data, err := json.MarshalIndent(seensIdMap, "", "  ")
	if err != nil {
		log.Printf("Error marshalling seensId: %v", err)
		return
	}
	if err := os.WriteFile("seens.json", data, 0644); err != nil {
		log.Printf("Error writing seensId to file: %v", err)
	}
}

func createMessageHandler(ctx context.Context, b *bot.Bot, m *models.Message) {
	if m.From == nil {
		log.Printf("Received message No.%d with no From field", m.ID)
		return
	}

	// chat type should be group or supergroup
	if m.Chat.Type != models.ChatTypeGroup && m.Chat.Type != models.ChatTypeSupergroup {
		log.Printf("Received message No.%d with unsupported chat type: %s", m.ID, m.Chat.Type)
		return
	}

	// check if the map for the chat exists
	if _, ok := seensIdMap[m.Chat.ID]; !ok {
		seensIdMap[m.Chat.ID] = make(map[int64]int64)
	}

	chatSeenMap := seensIdMap[m.Chat.ID]

	if m.From == nil || m.From.ID == 0 || m.From.ID == b.ID() || m.From.ID == 777000 {
		return
	}

	if _, seen := chatSeenMap[m.From.ID]; seen {
		log.Printf("Message No.%d from user %d in chat %d has already been seen, ignoring", m.ID, m.From.ID, m.Chat.ID)
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
			log.Printf("Error deleting message No.%d from user %d (%s %s): %v", m.ID, m.From.ID, err)
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
	// Mark the user as seen in the chat
	chatSeenMap[m.From.ID] = int64(m.ID)
}

func editMessageHandler(ctx context.Context, b *bot.Bot, m *models.Message) {
	if m.From == nil {
		log.Printf("Received message No.%d with no From field", m.ID)
		return
	}

	// chat type should be group or supergroup
	if m.Chat.Type != models.ChatTypeGroup && m.Chat.Type != models.ChatTypeSupergroup {
		log.Printf("Received message No.%d with unsupported chat type: %s", m.ID, m.Chat.Type)
		return
	}

	// check if the map for the chat exists
	if _, ok := seensIdMap[m.Chat.ID]; !ok {
		seensIdMap[m.Chat.ID] = make(map[int64]int64)
	}

	chatSeenMap := seensIdMap[m.Chat.ID]

	if m.From == nil || m.From.ID == 0 || m.From.ID == b.ID() || m.From.ID == 777000 {
		return
	}

	if id, seen := chatSeenMap[m.From.ID]; seen {
		needDelete := false
		// If changing the first seen message, 90% is spam, so delete it
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
			log.Printf("Deleted message No.%d from user %d in chat %d", m.ID, m.From.ID, m.Chat.ID)
			return
		}

		if id == int64(m.ID) && needDelete {
			success, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    m.Chat.ID,
				MessageID: m.ID,
			})
			if err != nil {
				log.Printf("Error deleting edited message No.%d: %v", m.ID, err)
			} else if !success {
				log.Printf("Failed to delete edited message No.%d", m.ID)
			} else {
				log.Printf("Deleted edited message No.%d from user %d in chat %d", m.ID, m.From.ID, m.Chat.ID)
			}
			// mark the user as unseen in the chat
			delete(chatSeenMap, m.From.ID)
			log.Printf("User %d in Chat %d was marking as unseen", m.From.ID, m.Chat.ID)
			return
		}
	}
}
