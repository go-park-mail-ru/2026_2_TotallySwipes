package repository_test

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
