package main

import (
	"regexp"

	"github.com/forPelevin/gomoji"
	"github.com/go-telegram/bot/models"
	"github.com/rivo/uniseg"
)

func mentionCheck(m *models.Message) bool {
	hasMention := false
	if m.Entities != nil {
		for _, entity := range m.Entities {
			if entity.Type == models.MessageEntityTypeMention || entity.Type == models.MessageEntityTypeTextMention {
				hasMention = true
				break
			}
		}
		for _, entity := range m.CaptionEntities {
			if entity.Type == models.MessageEntityTypeMention || entity.Type == models.MessageEntityTypeTextMention {
				hasMention = true
				break
			}
		}
	}
	return hasMention
}

func usernameCheck(m *models.Message) bool {
	firstname := m.From.FirstName
	lastname := m.From.LastName
	fullname := firstname + lastname
	regexsLock.RLock()
	defer regexsLock.RUnlock()
	for _, re := range regexs {
		if re.MatchString(firstname) || re.MatchString(lastname) || re.MatchString(fullname) {
			return true
		}
	}
	return false
}

func stickerCheck(m *models.Message) bool {
	if m.Sticker != nil {
		return true
	}
	return false
}

func contactCheck(m *models.Message) bool {
	if m.Contact != nil {
		return true
	}
	return false
}

func simpleEmojiCheck(m *models.Message) bool {
	count := uniseg.GraphemeClusterCount(m.Text)
	if count == 1 && gomoji.ContainsEmoji(m.Text) {
		return true
	}
	return false
}

func viaBotCheck(m *models.Message) bool {
	if m.ViaBot != nil {
		return true
	}
	return false
}

func linkCheck(m *models.Message) bool {
	if m.Entities != nil {
		for _, entity := range m.Entities {
			if entity.Type == models.MessageEntityTypeURL || entity.Type == models.MessageEntityTypeTextLink {
				return true
			}
		}
	}
	if m.CaptionEntities != nil {
		for _, entity := range m.CaptionEntities {
			if entity.Type == models.MessageEntityTypeURL || entity.Type == models.MessageEntityTypeTextLink {
				return true
			}
		}
	}
	return false
}

var meaninglessRegexs []regexp.Regexp

func init() {
	meaninglessPatterns := []string{
		`^(大家|你|您)?(早上|中午|晚上)?好`,
		`^(早安|午安|晚安)$`,
		`^谢谢(你|您)?$`,
		`^哈+$`,
		`^点?赞$`,
		`^\d+$`,
		`偷拍`,
	}
	for _, pattern := range meaninglessPatterns {
		re := regexp.MustCompile(pattern)
		meaninglessRegexs = append(meaninglessRegexs, *re)
	}
}

func meaninglessCheck(m *models.Message) bool {
	text := m.Text
	if text == "" && m.Caption != "" {
		text = m.Caption
	}
	if text == "" {
		return false
	}

	for _, re := range meaninglessRegexs {
		if re.MatchString(text) {
			return true
		}
	}

	return false
}
