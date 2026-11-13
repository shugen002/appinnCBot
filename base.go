package main

import (
	"regexp"
	"strings"

	"github.com/forPelevin/gomoji"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rivo/uniseg"
)

var whitelistStickerSets = []string{}

func mentionCheck(m *tgmodels.Message) bool {
	hasMention := false
	if m.Entities != nil {
		for _, entity := range m.Entities {
			if entity.Type == tgmodels.MessageEntityTypeMention || entity.Type == tgmodels.MessageEntityTypeTextMention {
				hasMention = true
				break
			}
		}
		for _, entity := range m.CaptionEntities {
			if entity.Type == tgmodels.MessageEntityTypeMention || entity.Type == tgmodels.MessageEntityTypeTextMention {
				hasMention = true
				break
			}
		}
	}
	return hasMention
}

func usernameCheck(m *tgmodels.Message) bool {
	firstname := strings.Join(strings.Fields(m.From.FirstName), "")
	lastname := strings.Join(strings.Fields(m.From.LastName), "")
	fullname := firstname + lastname
	regexsLock.RLock()
	defer regexsLock.RUnlock()
	for _, re := range usernameRegexes {
		if re.MatchString(firstname) || re.MatchString(lastname) || re.MatchString(fullname) {
			return true
		}
	}
	return false
}

func stickerCheck(m *tgmodels.Message) bool {
	if m.Sticker == nil {
		return false
	}

	setName := m.Sticker.SetName
	if setName == "" {
		return true
	}

	regexsLock.RLock()
	defer regexsLock.RUnlock()
	for _, allowedSet := range whitelistStickerSets {
		if strings.EqualFold(allowedSet, setName) {
			return false
		}
	}

	return true
}

func contactCheck(m *tgmodels.Message) bool {
	if m.Contact != nil {
		return true
	}
	return false
}

func simpleEmojiCheck(m *tgmodels.Message) bool {
	count := uniseg.GraphemeClusterCount(m.Text)
	if gomoji.ContainsEmoji(m.Text) && count <= 2 {
		return true
	}
	return false
}

func viaBotCheck(m *tgmodels.Message) bool {
	if m.ViaBot != nil {
		return true
	}
	return false
}

var meaninglessRegexes []regexp.Regexp

func meaninglessCheck(m *tgmodels.Message) bool {
	text := m.Text
	if text == "" && m.Caption != "" {
		text = m.Caption
	}
	if text == "" {
		return false
	}

	regexsLock.RLock()
	defer regexsLock.RUnlock()

	for _, re := range meaninglessRegexes {
		if re.MatchString(text) {
			return true
		}
	}

	return false
}
