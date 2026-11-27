package repository_test

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

const getReadPositionQuery = `
    SELECT member.last_read_seq, COALESCE(account.is_disabled, TRUE)
    FROM chat_member AS member
    LEFT JOIN "user" AS account ON account.id = member.user_id
    WHERE member.user_id = $1 AND member.chat_id = $2`
