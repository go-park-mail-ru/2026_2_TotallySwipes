package repository

import (
	"context"
	"database/sql"
	"dating-app/services/chat/internal/model"
	"errors"
	"fmt"
	"slices"
)

type ChatRepository struct {
	db *sql.DB
}

func NewChatRepository(db *sql.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

const getChatQuery = `
		SELECT id, match_id, created_at 
		FROM chat 
		WHERE id = $1`

func (r *ChatRepository) GetChat(ctx context.Context, chatID int64) (*model.Chat, error) {

	var chat model.Chat

	err := r.db.QueryRowContext(ctx, getChatQuery, chatID).Scan(&chat.ID, &chat.MatchID, &chat.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrChatNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get chat: %w", err)
	}

	return &chat, nil
}

const getMembersQuery = `
		SELECT chat_id, user_id, seen_at, last_read_seq
		FROM chat_member
		WHERE chat_id = $1
		ORDER BY user_id`

func (r *ChatRepository) GeyMembers(ctx context.Context, chatID int64) ([]model.ChatMember, error) {
	rows, err := r.db.QueryContext(ctx, getMembersQuery, chatID)

	if err != nil {
		return nil, fmt.Errorf("get chat members: %w", err)
	}
	defer rows.Close()

	members := make([]model.ChatMember, 0, 2)
	for rows.Next() {
		var member model.ChatMember

		if err := rows.Scan(&member.ChatID, &member.UserID, &member.SeenAt, &member.LastReadSeq); err != nil {
			return nil, fmt.Errorf("scan chat member: %w", err)
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat members: %w", err)
	}

	return members, nil
}

const getSummaryQuery = `
	WITH user_chats AS (
		SELECT
			member.chat_id,
			member.seen_at,
			COUNT(message.id) AS unread_count
		FROM chat_member AS member
		LEFT JOIN message
			ON message.chat_id = member.chat_id
			AND message.seq > member.last_read_seq
			AND message.sender_id <> member.user_id
			AND message.deleted_at IS NULL
		WHERE member.user_id = $1
		GROUP BY member.chat_id, member.seen_at
	)
	SELECT
		COUNT(*) FILTER (WHERE seen_at IS NULL),
		COUNT(*) FILTER (WHERE unread_count > 0),
		COALESCE(SUM(unread_count), 0)::bigint
	FROM user_chats`

func (r *ChatRepository) GetSummary(ctx context.Context, userID int64) (*model.ChatSummary, error) {
	var summary model.ChatSummary

	err := r.db.QueryRowContext(ctx, getSummaryQuery, userID).Scan(
		&summary.NewChatsCount,
		&summary.UnreadChatsCount,
		&summary.UnreadMessagesCount,
	)

	if err != nil {
		return nil, fmt.Errorf("get chat summary: %w", err)
	}

	return &summary, nil
}

const getChatQueryState = `
    SELECT
        chat.id,
        chat.match_id,
        chat.created_at,
        viewer.user_id,
        viewer.seen_at,
        viewer.last_read_seq,
        peer.user_id,
        peer.seen_at,
        peer.last_read_seq,
        COALESCE(viewer_account.is_disabled, TRUE),
        COALESCE(peer_account.is_disabled, TRUE),

        matching.id IS NOT NULL,
        COALESCE(matching.access_to_message, FALSE),

        EXISTS (
            SELECT 1
            FROM user_block
            WHERE blocker_id = viewer.user_id
              AND blocked_user_id = peer.user_id
        ),
        EXISTS (
            SELECT 1
            FROM user_block
            WHERE blocker_id = peer.user_id
              AND blocked_user_id = viewer.user_id
        ),

        COALESCE(sequence.last_seq, 0),
        unread.message_count,
        COALESCE(last_message.created_at, chat.created_at),

        last_message.id,
        last_message.seq,
        last_message.sender_id,
        last_message.type,
        CASE
            WHEN last_message.deleted_at IS NULL
            THEN LEFT(last_message.body, 100)
        END,
        CASE
            WHEN last_message.deleted_at IS NULL
            THEN last_message.duration_seconds
        END,
        last_message.created_at,
        CASE
            WHEN last_message.sender_id = viewer.user_id
            THEN last_message.seq <= peer.last_read_seq
        END

    FROM chat

    JOIN chat_member AS viewer
        ON viewer.chat_id = chat.id
       AND viewer.user_id = $1

    JOIN chat_member AS peer
        ON peer.chat_id = chat.id
       AND peer.user_id <> viewer.user_id

    LEFT JOIN "user" AS viewer_account
        ON viewer_account.id = viewer.user_id

    LEFT JOIN "user" AS peer_account
        ON peer_account.id = peer.user_id

    LEFT JOIN "match" AS matching
        ON matching.id = chat.match_id
       AND matching.first_user_id = LEAST(viewer.user_id, peer.user_id)
       AND matching.second_user_id = GREATEST(viewer.user_id, peer.user_id)

    LEFT JOIN chat_sequence AS sequence
        ON sequence.chat_id = chat.id

    CROSS JOIN LATERAL (
        SELECT COUNT(*) AS message_count
        FROM message
        WHERE message.chat_id = chat.id
          AND message.seq > viewer.last_read_seq
          AND message.sender_id <> viewer.user_id
          AND message.deleted_at IS NULL
    ) AS unread

    LEFT JOIN LATERAL (
        SELECT id, seq, sender_id, type, body,
               duration_seconds, created_at, deleted_at
        FROM message
        WHERE message.chat_id = chat.id
        ORDER BY seq DESC
        LIMIT 1
    ) AS last_message ON TRUE

    WHERE chat.id = $2
`

func (r *ChatRepository) GetChatState(ctx context.Context, userID, chatID int64) (*model.ChatState, error) {

	var state model.ChatState
	var lastMessage model.ChatLastMessage

	var lastMessageID sql.NullInt64
	var lastMessageSeq sql.NullInt64
	var lastMessageType sql.NullString
	var lastMessageCreatedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, getChatQueryState, userID, chatID).Scan(
		&state.Chat.ID,
		&state.Chat.MatchID,
		&state.Chat.CreatedAt,

		&state.Viewer.UserID,
		&state.Viewer.SeenAt,
		&state.Viewer.LastReadSeq,

		&state.Peer.UserID,
		&state.Peer.SeenAt,
		&state.Peer.LastReadSeq,

		&state.ViewerDisabled,
		&state.PeerDisabled,

		&state.MatchExists,
		&state.MatchAllowsMessaging,

		&state.ViewerBlockedPeer,
		&state.PeerBlockedViewer,

		&state.LastSeq,
		&state.UnreadCount,
		&state.LastActivityAt,

		&lastMessageID,
		&lastMessageSeq,
		&lastMessage.SenderID,
		&lastMessageType,
		&lastMessage.TextPreview,
		&lastMessage.DurationSeconds,
		&lastMessageCreatedAt,
		&lastMessage.IsReadByPeer,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrChatNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get chat state: %w", err)
	}

	state.Viewer.ChatID = state.Chat.ID
	state.Peer.ChatID = state.Chat.ID

	if lastMessageID.Valid {

		lastMessage.MessageID = lastMessageID.Int64
		lastMessage.Seq = lastMessageSeq.Int64
		lastMessage.Type = model.ChatMessageType(lastMessageType.String)
		lastMessage.CreatedAt = lastMessageCreatedAt.Time

		state.LastMessage = &lastMessage
	}

	return &state, nil

}

const lockChatSequenceQuery = `
    SELECT sequence.last_seq
    FROM chat_sequence AS sequence
    JOIN chat_member AS member
        ON member.chat_id = sequence.chat_id
       AND member.user_id = $1
    WHERE sequence.chat_id = $2
    FOR UPDATE OF sequence`

const findClientMessageQuery = `
    SELECT
        id, chat_id, client_message_id, seq, sender_id,
        type, body, storage_key, duration_seconds,
        created_at, updated_at, edited_at, deleted_at, version
    FROM message
    WHERE chat_id = $1
      AND sender_id = $2
      AND client_message_id = $3`

const canSendMessageQuery = `
    SELECT EXISTS (
        SELECT 1
        FROM chat
        JOIN chat_member AS viewer
            ON viewer.chat_id = chat.id
           AND viewer.user_id = $1
        JOIN chat_member AS peer
            ON peer.chat_id = chat.id
           AND peer.user_id <> viewer.user_id
        JOIN "user" AS viewer_account
            ON viewer_account.id = viewer.user_id
        JOIN "user" AS peer_account
            ON peer_account.id = peer.user_id
        JOIN "match" AS matching
            ON matching.id = chat.match_id
           AND matching.first_user_id = LEAST(viewer.user_id, peer.user_id)
           AND matching.second_user_id = GREATEST(viewer.user_id, peer.user_id)
        WHERE chat.id = $2
          AND matching.access_to_message
          AND NOT viewer_account.is_disabled
          AND NOT peer_account.is_disabled
          AND NOT EXISTS (
              SELECT 1
              FROM user_block
              WHERE (
                  blocker_id = viewer.user_id
                  AND blocked_user_id = peer.user_id
              ) OR (
                  blocker_id = peer.user_id
                  AND blocked_user_id = viewer.user_id
              )
          )
    )`

const createMessageQuery = `
    WITH next_position AS (
        UPDATE chat_sequence
        SET last_seq = last_seq + 1
        WHERE chat_id = $1
        RETURNING last_seq
    )
    INSERT INTO message (
        chat_id, sender_id, client_message_id, seq, type, body
    )
    SELECT $1, $2, $3, last_seq, 'text', $4
    FROM next_position
    RETURNING
        id, chat_id, client_message_id, seq, sender_id,
        type, body, storage_key, duration_seconds,
        created_at, updated_at, edited_at, deleted_at, version`

func (r *ChatRepository) CreateMessage(ctx context.Context, userID int64, command model.ChatSendMessage) (*model.StoredMessage, bool, error) {

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})

	if err != nil {
		return nil, false, fmt.Errorf("create message: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var lastSeq int64

	err = tx.QueryRowContext(ctx, lockChatSequenceQuery, userID, command.ChatID).Scan(&lastSeq)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, model.ErrChatNotFound
	}

	if err != nil {
		return nil, false, fmt.Errorf("create message: lock chat: %w", err)
	}

	existing, err := findClientMessage(ctx, tx, command.ChatID, userID, command.ClientMessageID)

	if err == nil {

		if existing.Type != model.ChatMessageTypeText ||
			existing.Text == nil ||
			*existing.Text != command.Text {
			return nil, false, model.ErrIdempotencyConflict
		}

		if err := tx.Commit(); err != nil {
			return nil, false, fmt.Errorf("create message: commit retry: %w", err)
		}

		return existing, false, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("create message: find existing: %w", err)
	}

	var canSend bool

	err = tx.QueryRowContext(
		ctx,
		canSendMessageQuery,
		userID,
		command.ChatID,
	).Scan(&canSend)

	if err != nil {
		return nil, false, fmt.Errorf("create message: check access: %w", err)
	}

	if !canSend {
		return nil, false, model.ErrMessagingNotAllowed
	}

	message, err := insertMessage(ctx, tx, userID, command)

	if err != nil {
		return nil, false, fmt.Errorf("create message: insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("create message: commit: %w", err)
	}

	return message, true, nil
}

const getMessageQuery = `
    SELECT
        id, chat_id, client_message_id, seq, sender_id,
        type, body, storage_key, duration_seconds,
        created_at, updated_at, edited_at, deleted_at, version
    FROM message
    WHERE chat_id = $1 AND id = $2`

func (r *ChatRepository) GetMessage(ctx context.Context, chatID, messageID int64) (*model.StoredMessage, error) {

	var message model.StoredMessage

	err := r.db.QueryRowContext(ctx, getMessageQuery, chatID, messageID).Scan(
		&message.ID,
		&message.ChatID,
		&message.ClientMessageID,
		&message.Seq,
		&message.SenderID,
		&message.Type,
		&message.Text,
		&message.StorageKey,
		&message.DurationSeconds,
		&message.CreatedAt,
		&message.UpdatedAt,
		&message.EditedAt,
		&message.DeletedAt,
		&message.Version,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrMessageNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}

	return &message, nil
}

func findClientMessage(ctx context.Context, tx *sql.Tx, chatID, userID int64, clientMessageID string) (*model.StoredMessage, error) {

	var message model.StoredMessage

	err := tx.QueryRowContext(ctx, findClientMessageQuery, chatID, userID, clientMessageID).Scan(
		&message.ID,
		&message.ChatID,
		&message.ClientMessageID,
		&message.Seq,
		&message.SenderID,
		&message.Type,
		&message.Text,
		&message.StorageKey,
		&message.DurationSeconds,
		&message.CreatedAt,
		&message.UpdatedAt,
		&message.EditedAt,
		&message.DeletedAt,
		&message.Version,
	)

	if err != nil {
		return nil, err
	}

	return &message, nil
}

func insertMessage(ctx context.Context, tx *sql.Tx, userID int64, command model.ChatSendMessage) (*model.StoredMessage, error) {

	var message model.StoredMessage

	err := tx.QueryRowContext(ctx, createMessageQuery, command.ChatID, userID, command.ClientMessageID, command.Text).Scan(
		&message.ID,
		&message.ChatID,
		&message.ClientMessageID,
		&message.Seq,
		&message.SenderID,
		&message.Type,
		&message.Text,
		&message.StorageKey,
		&message.DurationSeconds,
		&message.CreatedAt,
		&message.UpdatedAt,
		&message.EditedAt,
		&message.DeletedAt,
		&message.Version,
	)

	if err != nil {
		return nil, err
	}

	return &message, nil
}

const getMessagesQuery = `
    SELECT
        message.id,
        message.chat_id,
        message.client_message_id,
        message.seq,
        message.sender_id,
        message.type,
        message.body,
        message.storage_key,
        message.duration_seconds,
        message.created_at,
        message.updated_at,
        message.edited_at,
        message.deleted_at,
        message.version
    FROM message
    JOIN chat_member AS member
        ON member.chat_id = message.chat_id
       AND member.user_id = $1
    WHERE message.chat_id = $2
      AND ($3::bigint IS NULL OR message.seq < $3)
      AND ($4::bigint IS NULL OR message.seq > $4)`

const getOlderMessagesQuery = getMessagesQuery + `
    ORDER BY message.seq DESC
    LIMIT $5`

const getNewerMessagesQuery = getMessagesQuery + `
    ORDER BY message.seq ASC
    LIMIT $5`

const defaultLimit = 50

func (r *ChatRepository) GetMessages(ctx context.Context, userID, chatID int64, query model.ChatMessagesQuery) (*model.StoredMessagesPage, error) {
	limit := defaultLimit

	if query.Limit != nil {
		limit = *query.Limit
	}

	sqlQuery := getOlderMessagesQuery
	direction := model.ChatPageDirectionOlder

	if query.AfterSeq != nil {

		sqlQuery = getNewerMessagesQuery
		direction = model.ChatPageDirectionNewer
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, userID, chatID, query.BeforeSeq, query.AfterSeq, limit+1)

	if err != nil {
		return nil, fmt.Errorf("get chat messages: %w", err)
	}

	defer rows.Close()

	page := &model.StoredMessagesPage{
		Items:     make([]model.StoredMessage, 0, limit+1),
		Direction: direction,
	}

	for rows.Next() {

		var message model.StoredMessage

		if err := rows.Scan(
			&message.ID,
			&message.ChatID,
			&message.ClientMessageID,
			&message.Seq,
			&message.SenderID,
			&message.Type,
			&message.Text,
			&message.StorageKey,
			&message.DurationSeconds,
			&message.CreatedAt,
			&message.UpdatedAt,
			&message.EditedAt,
			&message.DeletedAt,
			&message.Version,
		); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}

		page.Items = append(page.Items, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat messages: %w", err)
	}

	page.HasMore = len(page.Items) > limit

	if page.HasMore {
		page.Items = page.Items[:limit]
	}

	if direction == model.ChatPageDirectionOlder {
		slices.Reverse(page.Items)
	}

	if page.HasMore {

		var cursor int64

		if direction == model.ChatPageDirectionOlder {
			cursor = page.Items[0].Seq
		} else {
			cursor = page.Items[len(page.Items)-1].Seq
		}

		page.NextCursor = &cursor
	}

	return page, nil
}

const markSeenQuery = `
    UPDATE chat_member
    SET seen_at = COALESCE(seen_at, CURRENT_TIMESTAMP)
    WHERE user_id = $1
      AND chat_id = $2`

func (r *ChatRepository) MarkSeen(ctx context.Context, userID, chatID int64) error {

	result, err := r.db.ExecContext(ctx, markSeenQuery, userID, chatID)

	if err != nil {
		return fmt.Errorf("mark chat seen: %w", err)
	}

	affected, err := result.RowsAffected()

	if err != nil {
		return fmt.Errorf("mark chat seen: rows affected: %w", err)
	}

	if affected == 0 {
		return model.ErrChatNotFound
	}

	return nil
}

const readChatQuery = `
    UPDATE chat_member AS member
    SET last_read_seq = GREATEST(member.last_read_seq, $3)
    FROM chat_sequence AS sequence, "user" AS account
    WHERE member.chat_id = $2
      AND member.user_id = $1
      AND sequence.chat_id = member.chat_id
      AND account.id = member.user_id
      AND NOT account.is_disabled
      AND $3 >= 0
      AND $3 <= sequence.last_seq
    RETURNING member.last_read_seq`

func (r *ChatRepository) ReadChat(ctx context.Context, userID int64, command model.ChatRead) (*model.ChatReadUpdated, error) {
	state, err := r.GetChatState(ctx, userID, command.ChatID)

	if err != nil {
		return nil, err
	}

	if state.ViewerDisabled {
		return nil, model.ErrChatMemberBlocked
	}

	if command.LastReadSeq < 0 || command.LastReadSeq > state.LastSeq {
		return nil, model.ErrInvalidReadSeq
	}

	var lastReadSeq int64

	err = r.db.QueryRowContext(ctx, readChatQuery, userID, command.ChatID, command.LastReadSeq).Scan(&lastReadSeq)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrChatNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("read chat: update read position: %w", err)
	}

	return &model.ChatReadUpdated{
		ChatID:      command.ChatID,
		UserID:      userID,
		LastReadSeq: lastReadSeq,
	}, nil
}
