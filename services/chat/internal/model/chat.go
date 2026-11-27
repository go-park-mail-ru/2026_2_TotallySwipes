package model

import "time"

type ChatEvent struct {
	EventID string
	Payload ChatEventPayload
}

type ChatEventPayload interface {
	isChatEventPayload()
}

type MessageCreated struct {
	ChatID          int64
	ClientMessageID *string
	Message         ChatMessage
}

func (MessageCreated) isChatEventPayload() {}

type MessageUpdated struct {
	ChatID  int64
	Message ChatMessage
}

func (MessageUpdated) isChatEventPayload() {}

type MessageDeleted struct {
	ChatID  int64
	Message ChatMessage
}

func (MessageDeleted) isChatEventPayload() {}

type ChatCreated struct {
	Chat ChatCard
}

func (ChatCreated) isChatEventPayload() {}

type ChatUpdated struct {
	Chat ChatCard
}

func (ChatUpdated) isChatEventPayload() {}

type ChatsSummaryUpdated struct {
	Summary ChatSummary
}

func (ChatsSummaryUpdated) isChatEventPayload() {}

type ChatReadUpdated struct {
	ChatID      int64
	UserID      int64
	LastReadSeq int64
}

func (ChatReadUpdated) isChatEventPayload() {}

type ChatMessageType string

const (
	ChatMessageTypeText      ChatMessageType = "text"
	ChatMessageTypeVoice     ChatMessageType = "voice"
	ChatMessageTypeVideoNote ChatMessageType = "video_note"
	ChatMessageTypeSystem    ChatMessageType = "system"
)

type ChatMessage struct {
	MessageID  int64
	Seq        int64
	SenderID   *int64
	Type       ChatMessageType
	Text       *string
	Attachment *ChatAttachment
	CreatedAt  time.Time
	EditedAt   *time.Time
	DeletedAt  *time.Time
	Version    int64
}

type ChatAttachment struct {
	URL             string
	DurationSeconds float64
}

type ChatCard struct {
	ChatID         int64
	MatchID        int64
	Peer           ChatPeer
	IsNew          bool
	UnreadCount    int64
	LastMessage    *ChatLastMessage
	CreatedAt      time.Time
	LastActivityAt time.Time
	Version        int64
}

type ChatPeer struct {
	UserID    int64
	Name      *string
	Age       *int
	AvatarURL *string
}

type ChatLastMessage struct {
	MessageID       int64
	Seq             int64
	SenderID        *int64
	Type            ChatMessageType
	TextPreview     *string
	DurationSeconds *float64
	CreatedAt       time.Time
	IsReadByPeer    *bool
}

type ChatSummary struct {
	NewChatsCount       int64
	UnreadChatsCount    int64
	UnreadMessagesCount int64
	Version             int64
}

type ChatSendMessage struct {
	ChatID          int64
	ClientMessageID string
	Text            string
}

type ChatEditMessage struct {
	ChatID    int64
	MessageID int64
	Text      string
}

type ChatDeleteMessage struct {
	ChatID    int64
	MessageID int64
}

type ChatRead struct {
	ChatID      int64
	LastReadSeq int64
}

type ChatDetails struct {
	Chat            ChatCard
	IsEmpty         bool
	CommonTags      []string
	LastReadSeq     int64
	PeerLastReadSeq int64
	Messaging       ChatMessaging
}

type ChatMessaging struct {
	CanSend     bool
	Restriction *string
}

const (
	MessagingRestrictionPeerDisabled  = "PEER_DISABLED"
	MessagingRestrictionPairBlocked   = "PAIR_BLOCKED"
	MessagingRestrictionMatchNotFound = "MATCH_NOT_FOUND"
	MessagingRestrictionNotAllowed    = "MESSAGING_NOT_ALLOWED"
)

type ChatMessagesQuery struct {
	Limit     *int
	BeforeSeq *int64
	AfterSeq  *int64
}

type ChatPageDirection string

const (
	ChatPageDirectionOlder ChatPageDirection = "older"
	ChatPageDirectionNewer ChatPageDirection = "newer"
)

type ChatMessagesPage struct {
	Items      []ChatMessage
	Direction  ChatPageDirection
	NextCursor *int64
	HasMore    bool
}
