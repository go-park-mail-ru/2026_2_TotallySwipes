package model

import "time"

// Chat связывает переписку с мэтчем; данные собеседника принадлежат Profile.
type Chat struct {
	ID        int64
	MatchID   int64
	CreatedAt time.Time
}

// ChatMember хранит состояние открытия и прочтения чата одним участником.
type ChatMember struct {
	ChatID      int64
	UserID      int64
	SeenAt      *time.Time
	LastReadSeq int64
}

// StoredMessage содержит сохранённые данные, включая скрываемое после удаления содержимое.
// Перед выдачей преобразуется в ChatMessage; ключ хранилища не передаётся клиенту.
type StoredMessage struct {
	ID              int64
	ChatID          int64
	ClientMessageID *string
	Seq             int64
	SenderID        *int64
	Type            ChatMessageType
	Text            *string
	StorageKey      *string
	DurationSeconds *float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	EditedAt        *time.Time
	DeletedAt       *time.Time
	Version         int64
}

type ChatState struct {
	Chat   Chat
	Viewer ChatMember
	Peer   ChatMember

	ViewerDisabled bool
	PeerDisabled   bool

	MatchExists       bool
	ViewerBlockedPeer bool
	PeerBlockedViewer bool

	LastSeq     int64
	UnreadCount int64
	LastMessage *ChatLastMessage
	CardVersion int64
}

type ChatProfile struct {
	UserID    int64
	Name      *string
	Age       *int
	AvatarURL *string
	Tags      []string
}
