package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"dating-app/services/chat/internal/model"
)

const (
	maxVoiceSize                = 5 << 20
	maxVideoNoteSize            = 10 << 20
	maxVoiceDurationSeconds     = 300
	maxVideoNoteDurationSeconds = 60
	mediaCleanupTimeout         = 5 * time.Second
)

func (s *ChatService) SendMedia(ctx context.Context, userID int64, command model.ChatSendMedia) (*model.ChatMessage, bool, error) {

	if userID <= 0 {
		return nil, false, model.ErrInvalidUserID
	}

	if command.ChatID <= 0 {
		return nil, false, model.ErrInvalidChatID
	}

	if !utf8.ValidString(command.ClientMessageID) || strings.TrimSpace(command.ClientMessageID) == "" ||
		utf8.RuneCountInString(command.ClientMessageID) > maxClientMessageIDLength {
		return nil, false, model.ErrInvalidClientMessageID
	}

	maxSize, maxDuration := maxVoiceSize, float64(maxVoiceDurationSeconds)

	switch command.Type {

	case model.ChatMessageTypeVoice:

	case model.ChatMessageTypeVideoNote:
		maxSize, maxDuration = maxVideoNoteSize, maxVideoNoteDurationSeconds

	default:
		return nil, false, model.ErrInvalidMedia
	}

	if len(command.Data) == 0 {
		return nil, false, model.ErrInvalidMedia
	}

	if len(command.Data) > maxSize {
		return nil, false, model.ErrMediaTooLarge
	}

	state, err := s.repo.GetChatState(ctx, userID, command.ChatID)

	if err != nil {
		return nil, false, err
	}

	if state.ViewerDisabled {
		return nil, false, model.ErrChatMemberBlocked
	}

	sum := sha256.Sum256(command.Data)
	digest := hex.EncodeToString(sum[:])

	existing, err := s.repo.GetMediaByClientID(ctx, userID, command.ChatID, command.ClientMessageID)

	if err == nil {

		if existing.Message.Type != command.Type || existing.SHA256 != digest {
			return nil, false, model.ErrIdempotencyConflict
		}

		message := s.chatMessageFromStored(existing.Message)
		return &message, false, nil
	}

	if !errors.Is(err, model.ErrMessageNotFound) {
		return nil, false, err
	}

	if state.PeerDisabled || state.ViewerBlockedPeer || state.PeerBlockedViewer ||
		!state.MatchExists || !state.MatchAllowsMessaging {
		return nil, false, model.ErrMessagingNotAllowed
	}

	info, err := s.inspector.Inspect(ctx, command.Data, command.Type)

	if err != nil {
		return nil, false, err
	}

	if info == nil || info.DurationSeconds <= 0 || math.IsNaN(info.DurationSeconds) || math.IsInf(info.DurationSeconds, 0) {
		return nil, false, model.ErrInvalidMedia
	}

	if info.DurationSeconds > maxDuration {
		return nil, false, model.ErrMediaTooLong
	}

	key, err := s.storage.Save(ctx, command.Data, command.Type, info.ContentType)

	if err != nil {
		return nil, false, err
	}

	stored, created, err := s.repo.CreateMediaMessage(ctx, userID, model.ChatCreateMedia{
		ChatID:          command.ChatID,
		ClientMessageID: command.ClientMessageID,
		Type:            command.Type,
		StorageKey:      key,
		DurationSeconds: info.DurationSeconds,
		SHA256:          digest,
	})

	if err != nil {

		if errors.Is(err, model.ErrMediaCommitUnknown) {
			checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaCleanupTimeout)
			saved, lookupErr := s.repo.GetMediaByClientID(checkCtx, userID, command.ChatID, command.ClientMessageID)
			cancel()

			if lookupErr != nil {
				slog.ErrorContext(ctx, "media commit remains uncertain; object retained", "storage_key", key, "error", lookupErr)
				return nil, false, err
			}

			if saved.Message.Type != command.Type || saved.SHA256 != digest {
				s.deleteUnusedMedia(ctx, key, saved.Message.StorageKey)
				return nil, false, model.ErrIdempotencyConflict
			}

			stored, created = &saved.Message, saved.Message.StorageKey != nil && *saved.Message.StorageKey == key

		} else {
			s.deleteUnusedMedia(ctx, key, nil)
			return nil, false, err
		}
	}

	s.deleteUnusedMedia(ctx, key, stored.StorageKey)
	message := s.chatMessageFromStored(*stored)

	if created {

		if err := s.publishMessageCreated(ctx, stored.ChatID, stored.ClientMessageID, message); err != nil {
			slog.ErrorContext(ctx, "failed to publish media message created", "chat_id", command.ChatID, "error", err)
		}

		if err := s.publishChatUpdated(ctx, command.ChatID); err != nil {
			slog.ErrorContext(ctx, "failed to publish chat updated", "chat_id", command.ChatID, "error", err)
		}

		if err := s.publishChatMembersSummaryUpdated(ctx, command.ChatID); err != nil {
			slog.ErrorContext(ctx, "failed to publish summary updated", "chat_id", command.ChatID, "error", err)
		}
	}

	return &message, created, nil
}

func (s *ChatService) deleteUnusedMedia(ctx context.Context, key string, savedKey *string) {

	if savedKey != nil && *savedKey == key {
		return
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaCleanupTimeout)
	defer cancel()

	if err := s.storage.Delete(cleanupCtx, key); err != nil {
		slog.ErrorContext(ctx, "failed to delete unused media object", "storage_key", key, "error", fmt.Errorf("cleanup media: %w", err))
	}
}
