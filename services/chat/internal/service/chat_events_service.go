package service

import (
	"context"
	"dating-app/services/chat/internal/model"
	"errors"

	"github.com/google/uuid"
)

type ChatEventsStream interface {
	Recv() (*model.ChatEvent, error)
	Close() error
}

type ChatEventBroker interface {
	Subscribe(ctx context.Context, userID int64) (ChatEventsStream, error)
	Publish(ctx context.Context, userID int64, event model.ChatEvent) error
}

func (s *ChatService) SubscribeEvents(ctx context.Context, userID int64) (ChatEventsStream, error) {

	if userID <= 0 {
		return nil, model.ErrInvalidUserID
	}

	stream, err := s.broker.Subscribe(ctx, userID)

	if err != nil {
		return nil, err
	}

	return stream, nil
}

func (s *ChatService) publishMessageCreated(
	ctx context.Context,
	chatID int64,
	clientMessageID *string,
	message model.ChatMessage,
) error {

	members, err := s.repo.GeyMembers(ctx, chatID)

	if err != nil {
		return err
	}

	event := model.ChatEvent{
		EventID: uuid.NewString(),
		Payload: model.MessageCreated{
			ChatID:          chatID,
			ClientMessageID: clientMessageID,
			Message:         message,
		},
	}

	var publishErr error
	for _, member := range members {
		if err := s.broker.Publish(ctx, member.UserID, event); err != nil {
			publishErr = errors.Join(publishErr, err)
		}
	}

	return publishErr
}

func (s *ChatService) publishChatCreated(ctx context.Context, chatID int64) error {

	members, err := s.repo.GeyMembers(ctx, chatID)

	if err != nil {
		return err
	}

	var publishErr error
	for _, member := range members {

		details, err := s.GetChat(ctx, member.UserID, chatID)

		if err != nil {
			publishErr = errors.Join(publishErr, err)
			continue
		}

		event := model.ChatEvent{
			EventID: uuid.NewString(),
			Payload: model.ChatCreated{
				Chat: details.Chat,
			},
		}

		if err := s.broker.Publish(ctx, member.UserID, event); err != nil {
			publishErr = errors.Join(publishErr, err)
		}
	}

	return publishErr
}

func (s *ChatService) publishChatUpdated(ctx context.Context, chatID int64) error {

	members, err := s.repo.GeyMembers(ctx, chatID)

	if err != nil {
		return err
	}

	var publishErr error
	for _, member := range members {

		details, err := s.GetChat(ctx, member.UserID, chatID)

		if err != nil {
			publishErr = errors.Join(publishErr, err)
			continue
		}

		event := model.ChatEvent{
			EventID: uuid.NewString(),
			Payload: model.ChatUpdated{
				Chat: details.Chat,
			},
		}

		if err := s.broker.Publish(ctx, member.UserID, event); err != nil {
			publishErr = errors.Join(publishErr, err)
		}
	}

	return publishErr
}

func (s *ChatService) publishSummaryUpdated(ctx context.Context, userID int64) error {
	summary, err := s.GetSummary(ctx, userID)

	if err != nil {
		return err
	}

	event := model.ChatEvent{
		EventID: uuid.NewString(),
		Payload: model.ChatsSummaryUpdated{
			Summary: *summary,
		},
	}

	return s.broker.Publish(ctx, userID, event)
}

func (s *ChatService) publishChatMembersSummaryUpdated(ctx context.Context, chatID int64) error {

	members, err := s.repo.GeyMembers(ctx, chatID)

	if err != nil {
		return err
	}

	var publishErr error
	for _, member := range members {
		if err := s.publishSummaryUpdated(ctx, member.UserID); err != nil {
			publishErr = errors.Join(publishErr, err)
		}
	}

	return publishErr
}

func (s *ChatService) publishChatReadUpdated(ctx context.Context, updated model.ChatReadUpdated) error {

	members, err := s.repo.GeyMembers(ctx, updated.ChatID)

	if err != nil {
		return err
	}

	event := model.ChatEvent{
		EventID: uuid.NewString(),
		Payload: updated,
	}

	var publishErr error

	for _, member := range members {
		if err := s.broker.Publish(ctx, member.UserID, event); err != nil {
			publishErr = errors.Join(publishErr, err)
		}
	}

	return publishErr
}
