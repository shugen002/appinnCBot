package main

import (
	"net/url"
	"strings"

	tgmodels "github.com/go-telegram/bot/models"
)

var whitelistDomain = []string{}

func linkCheck(m *tgmodels.Message) bool {
	if m.Entities != nil {
		for _, entity := range m.Entities {
			if entity.Type == tgmodels.MessageEntityTypeURL || entity.Type == tgmodels.MessageEntityTypeTextLink {
				return urlCheck(entity.URL)
			}
		}
	}
	if m.CaptionEntities != nil {
		for _, entity := range m.CaptionEntities {
			if entity.Type == tgmodels.MessageEntityTypeURL || entity.Type == tgmodels.MessageEntityTypeTextLink {
				return urlCheck(entity.URL)
			}
		}
	}
	return false
}

func urlCheck(urlStr string) bool {
	// parse the url and check the domain against the whitelist
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return true
	}
	regexsLock.RLock()
	defer regexsLock.RUnlock()
	for _, domain := range whitelistDomain {
		if urlObj.Host == domain || strings.HasSuffix(urlObj.Host, "."+domain) {
			return false
		}
	}
	return true
}
