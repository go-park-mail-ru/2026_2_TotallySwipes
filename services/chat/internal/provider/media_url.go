package provider

import "fmt"

type MediaLinkBuilder struct{}

func NewMediaLinkBuilder() *MediaLinkBuilder {
	return &MediaLinkBuilder{}
}

func (p *MediaLinkBuilder) BuildMessageMediaURL(chatID, messageID int64) string {
	return fmt.Sprintf("/api/v1/chats/%d/messages/%d/media", chatID, messageID)
}
