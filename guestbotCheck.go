package main

import (
	tgmodels "github.com/go-telegram/bot/models"
)

// inlineKeyboardCheck
// 如果消息包含内联键盘按钮，用满30个按钮，且每个按钮的URL相同，那么判定为true
func inlineKeyboardCheck(m *tgmodels.Message) bool {
	if m.ReplyMarkup != nil && m.ReplyMarkup.InlineKeyboard != nil {
		buttonCount := 0
		var firstURL string
		for _, row := range m.ReplyMarkup.InlineKeyboard {
			for _, button := range row {
				if button.URL != "" {
					if firstURL == "" {
						firstURL = button.URL
					} else if button.URL != firstURL {
						return false
					}
					buttonCount++
				}
			}
		}
		if buttonCount >= 30 {
			return true
		}
	}
	return false
}
