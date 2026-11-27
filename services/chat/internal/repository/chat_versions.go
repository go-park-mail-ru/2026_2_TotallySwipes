package repository

import (
	"context"
	"database/sql"
	"fmt"
)

const incrementChatCardVersionsQuery = `
    UPDATE chat_member
    SET card_version = card_version + 1
    WHERE chat_id = $1`

const incrementUserSummaryVersionQuery = `
    INSERT INTO chat_user_state (user_id, summary_version)
    VALUES ($1, 2)
    ON CONFLICT (user_id)
    DO UPDATE SET summary_version = chat_user_state.summary_version + 1`

const incrementChatSummaryVersionsQuery = `
    INSERT INTO chat_user_state (user_id, summary_version)
    SELECT user_id, 2
    FROM chat_member
    WHERE chat_id = $1 AND ($2::bigint IS NULL OR user_id <> $2)
    ORDER BY user_id
    ON CONFLICT (user_id)
    DO UPDATE SET summary_version = chat_user_state.summary_version + 1`

// Вызывается только для нового сообщения, внутри транзакции его сохранения.
func incrementMessageVersions(ctx context.Context, tx *sql.Tx, chatID, senderID int64) error {

	if _, err := tx.ExecContext(ctx, incrementChatCardVersionsQuery, chatID); err != nil {
		return fmt.Errorf("increment chat card versions: %w", err)
	}

	if _, err := tx.ExecContext(ctx, incrementChatSummaryVersionsQuery, chatID, senderID); err != nil {
		return fmt.Errorf("increment recipient summary version: %w", err)
	}

	return nil
}
