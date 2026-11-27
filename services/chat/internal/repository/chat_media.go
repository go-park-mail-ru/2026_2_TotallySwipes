package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"dating-app/services/chat/internal/model"
)

const findClientMediaQuery = `
    SELECT
        id, chat_id, client_message_id, seq, sender_id,
        type, body, storage_key, duration_seconds,
        created_at, updated_at, edited_at, deleted_at, version,
        COALESCE(media_sha256, '')
    FROM message
    WHERE chat_id = $1 AND sender_id = $2 AND client_message_id = $3
      AND EXISTS (
          SELECT 1 FROM chat_member
          WHERE chat_member.chat_id = message.chat_id AND user_id = $2
      )`

func (r *ChatRepository) GetMediaByClientID(ctx context.Context, userID, chatID int64, clientMessageID string) (*model.StoredMediaMessage, error) {

	var result model.StoredMediaMessage

	err := r.db.QueryRowContext(ctx, findClientMediaQuery, chatID, userID, clientMessageID).Scan(
		&result.Message.ID,
		&result.Message.ChatID,
		&result.Message.ClientMessageID,
		&result.Message.Seq,
		&result.Message.SenderID,
		&result.Message.Type,
		&result.Message.Text,
		&result.Message.StorageKey,
		&result.Message.DurationSeconds,
		&result.Message.CreatedAt,
		&result.Message.UpdatedAt,
		&result.Message.EditedAt,
		&result.Message.DeletedAt,
		&result.Message.Version,
		&result.SHA256,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrMessageNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find media message: %w", err)
	}

	return &result, nil
}

func findClientMedia(ctx context.Context, tx *sql.Tx, chatID, userID int64, clientMessageID string) (*model.StoredMediaMessage, error) {

	var result model.StoredMediaMessage

	err := tx.QueryRowContext(ctx, findClientMediaQuery, chatID, userID, clientMessageID).Scan(
		&result.Message.ID,
		&result.Message.ChatID,
		&result.Message.ClientMessageID,
		&result.Message.Seq,
		&result.Message.SenderID,
		&result.Message.Type,
		&result.Message.Text,
		&result.Message.StorageKey,
		&result.Message.DurationSeconds,
		&result.Message.CreatedAt,
		&result.Message.UpdatedAt,
		&result.Message.EditedAt,
		&result.Message.DeletedAt,
		&result.Message.Version,
		&result.SHA256,
	)

	if err != nil {
		return nil, err
	}

	return &result, nil
}

const createMediaMessageQuery = `
    WITH next_position AS (
        UPDATE chat_sequence
        SET last_seq = last_seq + 1
        WHERE chat_id = $1
        RETURNING last_seq
    )
    INSERT INTO message (
        chat_id, sender_id, client_message_id, seq, type,
        storage_key, duration_seconds, media_sha256
    )
    SELECT $1, $2, $3, last_seq, $4, $5, $6, $7
    FROM next_position
    RETURNING
        id, chat_id, client_message_id, seq, sender_id,
        type, body, storage_key, duration_seconds,
        created_at, updated_at, edited_at, deleted_at, version`

func (r *ChatRepository) CreateMediaMessage(ctx context.Context, userID int64, command model.ChatCreateMedia) (*model.StoredMessage, bool, error) {

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})

	if err != nil {
		return nil, false, fmt.Errorf("create media message: begin: %w", err)
	}
	defer tx.Rollback()

	var lastSeq int64
	err = tx.QueryRowContext(ctx, lockChatSequenceQuery, userID, command.ChatID).Scan(&lastSeq)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, model.ErrChatNotFound
	}

	if err != nil {
		return nil, false, fmt.Errorf("create media message: lock chat: %w", err)
	}

	existing, err := findClientMedia(ctx, tx, command.ChatID, userID, command.ClientMessageID)

	if err == nil {

		if existing.Message.Type != command.Type || existing.SHA256 != command.SHA256 {
			return nil, false, model.ErrIdempotencyConflict
		}

		if err := tx.Commit(); err != nil {
			return nil, false, fmt.Errorf("create media message: commit retry: %w", errors.Join(model.ErrMediaCommitUnknown, err))
		}

		return &existing.Message, false, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("create media message: find existing: %w", err)
	}

	var allowed bool

	if err := tx.QueryRowContext(ctx, canSendMessageQuery, userID, command.ChatID).Scan(&allowed); err != nil {
		return nil, false, fmt.Errorf("create media message: check access: %w", err)
	}

	if !allowed {
		return nil, false, model.ErrMessagingNotAllowed
	}

	var message model.StoredMessage

	err = tx.QueryRowContext(ctx, createMediaMessageQuery,
		command.ChatID, userID, command.ClientMessageID, string(command.Type),
		command.StorageKey, command.DurationSeconds, command.SHA256,
	).Scan(
		&message.ID, &message.ChatID, &message.ClientMessageID, &message.Seq,
		&message.SenderID, &message.Type, &message.Text, &message.StorageKey,
		&message.DurationSeconds, &message.CreatedAt, &message.UpdatedAt,
		&message.EditedAt, &message.DeletedAt, &message.Version,
	)

	if err != nil {
		return nil, false, fmt.Errorf("create media message: insert: %w", err)
	}

	if err := incrementMessageVersions(ctx, tx, command.ChatID, userID); err != nil {
		return nil, false, err
	}

	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("create media message: commit: %w", errors.Join(model.ErrMediaCommitUnknown, err))
	}

	return &message, true, nil
}
