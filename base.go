package main

import (
	"regexp"

	"github.com/forPelevin/gomoji"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rivo/uniseg"
)

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

func stickerCheck(m *tgmodels.Message) bool {
	if m.Sticker != nil {
		return true
	}
	return false
}

func contactCheck(m *tgmodels.Message) bool {
	if m.Contact != nil {
		return true
	}
	return false
}

func simpleEmojiCheck(m *tgmodels.Message) bool {
	count := uniseg.GraphemeClusterCount(m.Text)
	if count == 1 && gomoji.ContainsEmoji(m.Text) {
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

func linkCheck(m *tgmodels.Message) bool {
	if m.Entities != nil {
		for _, entity := range m.Entities {
			if entity.Type == tgmodels.MessageEntityTypeURL || entity.Type == tgmodels.MessageEntityTypeTextLink {
				return true
			}
		}
	}
	if m.CaptionEntities != nil {
		for _, entity := range m.CaptionEntities {
			if entity.Type == tgmodels.MessageEntityTypeURL || entity.Type == tgmodels.MessageEntityTypeTextLink {
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

func meaninglessCheck(m *tgmodels.Message) bool {
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
