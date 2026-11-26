package service

import (
	"context"
	"dating-app/services/chat/internal/model"
	"strings"
	"unicode/utf8"
)

const (
	defaultLimit             = 50
	maxClientMessageIDLength = 128
	maxMessageTextLength     = 10000
)

type ChatRepository interface {
	GetChat(ctx context.Context, chatID int64) (*model.Chat, error)
	GeyMembers(ctx context.Context, chatID int64) ([]model.ChatMember, error)
	GetSummary(ctx context.Context, userID int64) (*model.ChatSummary, error)
	GetChatState(ctx context.Context, userID, chatID int64) (*model.ChatState, error)
	CreateMessage(ctx context.Context, userID int64, command model.ChatSendMessage) (*model.StoredMessage, bool, error)
	GetMessages(ctx context.Context, userID, chatID int64, query model.ChatMessagesQuery) (*model.StoredMessagesPage, error)

	GetMessage(ctx context.Context, chatID, messageID int64) (*model.StoredMessage, error)

	ReadChat(ctx context.Context, userID int64, command model.ChatRead) (*model.ChatReadUpdated, error)
	MarkSeen(ctx context.Context, userID, chatID int64) error
}

type MediaStorage interface {
	GetPresignedDownloadURL(ctx context.Context, key string) (string, error)
}

type MediaLinkBuilder interface {
	BuildMessageMediaURL(chatID, messageID int64) string
}

type ProfileProvider interface {
	GetChatProfiles(ctx context.Context, userIDs []int64) (map[int64]model.ChatProfile, error)
}

type ChatService struct {
	repo       ChatRepository
	profile    ProfileProvider
	storage    MediaStorage
	mediaLinks MediaLinkBuilder
}

func NewChatService(repo ChatRepository, profile ProfileProvider, storage MediaStorage, mediaLinks MediaLinkBuilder) *ChatService {
	return &ChatService{repo: repo, profile: profile, storage: storage, mediaLinks: mediaLinks}
}

func (s *ChatService) GetSummary(ctx context.Context, userID int64) (*model.ChatSummary, error) {

	if userID <= 0 {
		return nil, model.ErrInvalidUserID
	}

	return s.repo.GetSummary(ctx, userID)
}

func (s *ChatService) GetChat(ctx context.Context, userID, chatID int64) (*model.ChatDetails, error) {

	state, err := s.repo.GetChatState(ctx, userID, chatID)

	if err != nil {
		return nil, err
	}

	if state.ViewerDisabled {
		return nil, model.ErrChatMemberBlocked
	}

	messaging := model.ChatMessaging{
		CanSend: true,
	}

	var restriction string

	switch {

	case state.PeerDisabled:
		restriction = model.MessagingRestrictionPeerDisabled

	case state.ViewerBlockedPeer || state.PeerBlockedViewer:
		restriction = model.MessagingRestrictionPairBlocked

	case !state.MatchExists:
		restriction = model.MessagingRestrictionMatchNotFound
	}

	if restriction != "" {
		messaging.CanSend = false
		messaging.Restriction = &restriction
	}

	details := &model.ChatDetails{
		Chat: model.ChatCard{
			ChatID:      state.Chat.ID,
			MatchID:     state.Chat.MatchID,
			IsNew:       state.Viewer.SeenAt == nil,
			UnreadCount: state.UnreadCount,
			LastMessage: state.LastMessage,
			CreatedAt:   state.Chat.CreatedAt,
			Version:     state.CardVersion,
		},
		IsEmpty:         state.LastSeq == 0,
		LastReadSeq:     state.Viewer.LastReadSeq,
		PeerLastReadSeq: state.Peer.LastReadSeq,
		Messaging:       messaging,
	}

	ids := []int64{state.Viewer.UserID, state.Peer.UserID}

	profiles, err := s.profile.GetChatProfiles(ctx, ids)

	if err != nil {
		return details, err
	}

	viewerProfile := profiles[state.Viewer.UserID]
	peerProfile := profiles[state.Peer.UserID]

	details.CommonTags = commonTags(viewerProfile.Tags, peerProfile.Tags)

	details.Chat.Peer = model.ChatPeer{
		UserID:    state.Peer.UserID,
		Name:      peerProfile.Name,
		Age:       peerProfile.Age,
		AvatarURL: peerProfile.AvatarURL,
	}

	return details, nil
}

func commonTags(viewerTags, peerTags []string) []string {

	peerSet := make(map[string]struct{}, len(peerTags))
	for _, tag := range peerTags {

		peerSet[tag] = struct{}{}
	}

	result := make([]string, 0)

	for _, tag := range viewerTags {

		if _, ok := peerSet[tag]; ok {
			result = append(result, tag)
			delete(peerSet, tag)
		}
	}

	return result
}

func (s *ChatService) GetMessages(ctx context.Context, userID, chatID int64, query model.ChatMessagesQuery) (*model.ChatMessagesPage, error) {

	if userID <= 0 {
		return nil, model.ErrInvalidUserID
	}

	if chatID <= 0 {
		return nil, model.ErrInvalidChatID
	}

	if query.BeforeSeq != nil && query.AfterSeq != nil {
		return nil, model.ErrInvalidMessagesQuery
	}

	if query.BeforeSeq != nil && *query.BeforeSeq <= 0 {
		return nil, model.ErrInvalidMessagesQuery
	}

	if query.AfterSeq != nil && *query.AfterSeq < 0 {
		return nil, model.ErrInvalidMessagesQuery
	}

	limit := defaultLimit

	if query.Limit != nil {
		limit = *query.Limit
	}

	if limit < 1 {
		return nil, model.ErrInvalidMessagesQuery
	}

	query.Limit = &limit

	state, err := s.repo.GetChatState(ctx, userID, chatID)

	if err != nil {
		return nil, err
	}

	if state.ViewerDisabled {
		return nil, model.ErrChatMemberBlocked
	}

	storedPage, err := s.repo.GetMessages(ctx, userID, chatID, query)

	if err != nil {
		return nil, err
	}

	page := &model.ChatMessagesPage{
		Items:      make([]model.ChatMessage, 0, len(storedPage.Items)),
		Direction:  storedPage.Direction,
		NextCursor: storedPage.NextCursor,
		HasMore:    storedPage.HasMore,
	}

	for _, stored := range storedPage.Items {
		page.Items = append(page.Items, s.chatMessageFromStored(stored))
	}

	return page, nil
}

func (s *ChatService) MarkSeen(ctx context.Context, userID, chatID int64) error {

	if userID <= 0 {
		return model.ErrInvalidUserID
	}

	if chatID <= 0 {
		return model.ErrInvalidChatID
	}

	return s.repo.MarkSeen(ctx, userID, chatID)

}

func (s *ChatService) SendMessage(ctx context.Context, userID int64, command model.ChatSendMessage) (*model.ChatMessage, error) {

	if userID <= 0 {
		return nil, model.ErrInvalidUserID
	}

	if command.ChatID <= 0 {
		return nil, model.ErrInvalidChatID
	}

	if !utf8.ValidString(command.ClientMessageID) ||
		strings.TrimSpace(command.ClientMessageID) == "" ||
		utf8.RuneCountInString(command.ClientMessageID) > maxClientMessageIDLength {
		return nil, model.ErrInvalidClientMessageID
	}

	if !utf8.ValidString(command.Text) ||
		strings.TrimSpace(command.Text) == "" ||
		utf8.RuneCountInString(command.Text) > maxMessageTextLength {
		return nil, model.ErrInvalidMessageText
	}

	stored, _, err := s.repo.CreateMessage(ctx, userID, command)

	if err != nil {
		return nil, err
	}

	message := s.chatMessageFromStored(*stored)

	return &message, nil
}

func (s *ChatService) ReadChat(ctx context.Context, userID int64, command model.ChatRead) (*model.ChatReadUpdated, error) {

	if userID <= 0 {
		return nil, model.ErrInvalidUserID
	}

	if command.ChatID <= 0 {
		return nil, model.ErrInvalidChatID
	}

	if command.LastReadSeq < 0 {
		return nil, model.ErrInvalidReadSeq
	}

	return s.repo.ReadChat(ctx, userID, command)
}

func (s *ChatService) GetMedia(ctx context.Context, userID, chatID, messageID int64) (string, error) {
	if userID <= 0 {
		return "", model.ErrInvalidUserID
	}

	if chatID <= 0 {
		return "", model.ErrInvalidChatID
	}

	if messageID <= 0 {
		return "", model.ErrInvalidMessageID
	}

	state, err := s.repo.GetChatState(ctx, userID, chatID)

	if err != nil {
		return "", err
	}

	if state.ViewerDisabled {
		return "", model.ErrChatMemberBlocked
	}

	message, err := s.repo.GetMessage(ctx, chatID, messageID)

	if err != nil {
		return "", err
	}

	if message.DeletedAt != nil || message.StorageKey == nil || *message.StorageKey == "" {
		return "", model.ErrMediaNotFound
	}

	return s.storage.GetPresignedDownloadURL(ctx, *message.StorageKey)
}

func (s *ChatService) chatMessageFromStored(stored model.StoredMessage) model.ChatMessage {

	message := model.ChatMessage{
		MessageID: stored.ID,
		Seq:       stored.Seq,
		SenderID:  stored.SenderID,
		Type:      stored.Type,
		CreatedAt: stored.CreatedAt,
		EditedAt:  stored.EditedAt,
		DeletedAt: stored.DeletedAt,
		Version:   stored.Version,
	}

	if stored.DeletedAt != nil {
		return message
	}

	message.Text = stored.Text

	if stored.StorageKey != nil && stored.DurationSeconds != nil {
		message.Attachment = &model.ChatAttachment{
			URL:             s.mediaLinks.BuildMessageMediaURL(stored.ChatID, stored.ID),
			DurationSeconds: *stored.DurationSeconds,
		}
	}

	return message
}
