package service

import (
	"context"
	"dating-app/services/chat/internal/model"
)

type ChatRepository interface {
	GetChat(ctx context.Context, chatID int64) (*model.Chat, error)
	GeyMembers(ctx context.Context, chatID int64) ([]model.ChatMember, error)
	GetSummary(ctx context.Context, userID int64) (*model.ChatSummary, error)
	GetChatState(ctx context.Context, userID, chatID int64) (*model.ChatState, error)

	CreateMessage(ctx context.Context, userID int64, command model.ChatSendMessage) (*model.StoredMessage, bool, error)
}

type ProfileProvider interface {
	GetChatProfiles(ctx context.Context, userIDs []int64) (map[int64]model.ChatProfile, error)
}

type ChatService struct {
	repo    ChatRepository
	profile ProfileProvider
}

func NewChatService(repo ChatRepository, profile ProfileProvider) *ChatService {
	return &ChatService{repo: repo, profile: profile}
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

}

// Снять новизну чата; сообщения прочитанными не делает.
func (s *ChatService) MarkSeen(ctx context.Context, userID, chatID int64) error {

}

// Отправка текста и подтверждение прочтения.
func (s *ChatService) SendMessage(ctx context.Context, userID int64, command model.ChatSendMessage) (*model.ChatMessage, error) {

}

func (s *ChatService) ReadChat(ctx context.Context, userID int64, command model.ChatRead) (*model.ChatReadUpdated, error) {

}

// Получение внутренней ссылки на медиа после проверки доступа.
func (s *ChatService) GetMedia(ctx context.Context, userID, chatID, messageID int64) (string, error) {

}
