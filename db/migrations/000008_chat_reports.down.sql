BEGIN;

-- Старый контракт не умеет медиа, системные/удалённые сообщения и историю
-- удалённых мэтчей. Вместо потери данных откат явно отклоняется.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "user" WHERE is_disabled)
       OR EXISTS (SELECT 1 FROM user_block)
       OR EXISTS (SELECT 1 FROM report_message)
       OR EXISTS (SELECT 1 FROM report WHERE body IS NOT NULL OR status <> 'pending')
       OR EXISTS (SELECT 1 FROM chat_member WHERE seen_at IS NOT NULL OR last_read_seq <> 0)
       OR EXISTS (SELECT 1 FROM chat c LEFT JOIN "match" m ON m.id = c.match_id WHERE m.id IS NULL)
       OR EXISTS (SELECT 1 FROM message WHERE type <> 'text' OR deleted_at IS NOT NULL
            OR edited_at IS NOT NULL OR client_message_id IS NOT NULL OR version <> 1)
       OR EXISTS (SELECT 1 FROM report r LEFT JOIN "user" a ON a.id = r.author_id
            LEFT JOIN "user" t ON t.id = r.reported_user_id WHERE a.id IS NULL OR t.id IS NULL)
       OR EXISTS (SELECT 1 FROM report GROUP BY author_id, reported_user_id, report_type_id HAVING count(*) > 1)
    THEN
        RAISE EXCEPTION 'chat/report data cannot be represented by migration 000007; export or transform it before rollback';
    END IF;
END;
$$;

DROP TABLE report_message;
DROP INDEX report_queue_idx;
DROP INDEX report_target_idx;
ALTER TABLE report
    DROP CONSTRAINT report_body_required,
    DROP CONSTRAINT report_body_valid,
    DROP CONSTRAINT report_status_valid,
    DROP CONSTRAINT report_review_state,
    DROP CONSTRAINT report_user_ids_positive,
    DROP COLUMN body,
    DROP COLUMN status,
    DROP COLUMN reviewed_by,
    DROP COLUMN reviewed_at,
    ADD CONSTRAINT report_author_id_reported_user_id_report_type_id_key
        UNIQUE (author_id, reported_user_id, report_type_id),
    ADD CONSTRAINT report_author FOREIGN KEY (author_id) REFERENCES "user"(id) ON DELETE CASCADE ON UPDATE RESTRICT,
    ADD CONSTRAINT report_target FOREIGN KEY (reported_user_id) REFERENCES "user"(id) ON DELETE CASCADE ON UPDATE RESTRICT;

DROP TABLE user_block;

ALTER TABLE message ADD COLUMN match_id BIGINT;
UPDATE message msg SET match_id = c.match_id FROM chat c WHERE c.id = msg.chat_id;
DROP INDEX message_unread_idx;
ALTER TABLE message
    DROP CONSTRAINT message_chat,
    DROP CONSTRAINT message_chat_sender,
    DROP CONSTRAINT message_body_valid,
    DROP COLUMN chat_id,
    DROP COLUMN seq,
    DROP COLUMN client_message_id,
    DROP COLUMN type,
    DROP COLUMN storage_key,
    DROP COLUMN duration_seconds,
    DROP COLUMN edited_at,
    DROP COLUMN deleted_at,
    DROP COLUMN version,
    ALTER COLUMN match_id SET NOT NULL,
    ALTER COLUMN sender_id SET NOT NULL,
    ALTER COLUMN body SET NOT NULL,
    ADD CONSTRAINT message_match FOREIGN KEY (match_id) REFERENCES "match"(id) ON DELETE CASCADE ON UPDATE RESTRICT,
    ADD CONSTRAINT message_sender FOREIGN KEY (sender_id) REFERENCES "user"(id) ON DELETE CASCADE ON UPDATE RESTRICT,
    ADD CONSTRAINT message_body_valid CHECK (char_length(body) BETWEEN 1 AND 10000 AND body ~ '[^[:space:]]');

DROP TABLE chat_sequence;
DROP TABLE chat_member;
DROP TABLE chat;
ALTER TABLE "user" DROP COLUMN is_disabled;

COMMIT;
