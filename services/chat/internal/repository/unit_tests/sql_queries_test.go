package repository_test

// Ожидаемые запросы фиксируют условия доступа и порядок параметров.
const getChatQuery = `
		SELECT id, match_id, created_at 
		FROM chat 
		WHERE id = $1`

const getMembersQuery = `
		SELECT chat_id, user_id, seen_at, last_read_seq
		FROM chat_member
		WHERE chat_id = $1
		ORDER BY user_id`

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

const getMessageQuery = `
    SELECT
        id, chat_id, client_message_id, seq, sender_id,
        type, body, storage_key, duration_seconds,
        created_at, updated_at, edited_at, deleted_at, version
    FROM message
    WHERE chat_id = $1 AND id = $2`

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

const markSeenQuery = `
    UPDATE chat_member
    SET seen_at = COALESCE(seen_at, CURRENT_TIMESTAMP)
    WHERE user_id = $1
      AND chat_id = $2`

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
