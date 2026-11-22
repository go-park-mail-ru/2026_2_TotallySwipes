BEGIN;

ALTER TABLE "user"
    ADD COLUMN is_disabled BOOLEAN NOT NULL DEFAULT FALSE;

-- match_id — внешний идентификатор Matching. Намеренно без FK:
-- удаление обычного мэтча не должно удалять историю Chat.
-- Новый match_id создаёт новый чат; старый остаётся историей без права отправки.
CREATE TABLE chat (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    match_id BIGINT NOT NULL UNIQUE CHECK (match_id > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP CHECK (isfinite(created_at))
);

CREATE TABLE chat_member (
    chat_id BIGINT NOT NULL REFERENCES chat(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL CHECK (user_id > 0),
    seen_at TIMESTAMPTZ CHECK (isfinite(seen_at)),
    last_read_seq BIGINT NOT NULL DEFAULT 0 CHECK (last_read_seq >= 0),
    PRIMARY KEY (chat_id, user_id)
);

-- ID пользователей — внешние идентификаторы Auth. Состав личного чата
-- (ровно два участника мэтча) проверяет Chat при создании в одной транзакции.
CREATE INDEX chat_member_user_idx ON chat_member(user_id, chat_id);

-- Технический счётчик, обновляемый в одной транзакции с INSERT message.
-- UPDATE ... RETURNING last_seq сериализует отправки внутри одного чата.
CREATE TABLE chat_sequence (
    chat_id BIGINT PRIMARY KEY REFERENCES chat(id) ON DELETE CASCADE,
    last_seq BIGINT NOT NULL DEFAULT 0 CHECK (last_seq >= 0)
);

-- Создаём также пустые чаты для уже существующих мэтчей.
INSERT INTO chat (match_id, created_at)
SELECT id, created_at FROM "match" ORDER BY id;

INSERT INTO chat_member (chat_id, user_id)
SELECT c.id, m.first_user_id FROM chat c JOIN "match" m ON m.id = c.match_id
UNION ALL
SELECT c.id, m.second_user_id FROM chat c JOIN "match" m ON m.id = c.match_id;

ALTER TABLE message
    ADD COLUMN chat_id BIGINT,
    ADD COLUMN seq BIGINT,
    ADD COLUMN client_message_id TEXT,
    ADD COLUMN type TEXT NOT NULL DEFAULT 'text',
    ADD COLUMN storage_key TEXT DEFAULT NULL,
    ADD COLUMN duration_seconds DOUBLE PRECISION,
    ADD COLUMN edited_at TIMESTAMPTZ,
    ADD COLUMN deleted_at TIMESTAMPTZ,
    ADD COLUMN version BIGINT NOT NULL DEFAULT 1;

-- Сохраняем порядок существующей истории по времени и ID при равном времени.
WITH history AS (
    SELECT msg.id, c.id AS chat_id,
           row_number() OVER (PARTITION BY msg.match_id ORDER BY msg.created_at, msg.id) AS seq
    FROM message msg JOIN chat c ON c.match_id = msg.match_id
)
UPDATE message msg SET chat_id = h.chat_id, seq = h.seq
FROM history h WHERE msg.id = h.id;

INSERT INTO chat_sequence (chat_id, last_seq)
SELECT c.id, COALESCE(MAX(msg.seq), 0)
FROM chat c LEFT JOIN message msg ON msg.chat_id = c.id GROUP BY c.id;

ALTER TABLE message
    DROP CONSTRAINT message_match,
    DROP CONSTRAINT message_sender,
    DROP CONSTRAINT message_body_valid,
    DROP COLUMN match_id,
    ALTER COLUMN chat_id SET NOT NULL,
    ALTER COLUMN seq SET NOT NULL,
    ALTER COLUMN sender_id DROP NOT NULL,
    ALTER COLUMN body DROP NOT NULL,
    ADD CONSTRAINT message_chat FOREIGN KEY (chat_id) REFERENCES chat(id) ON DELETE RESTRICT,
    ADD CONSTRAINT message_chat_sender FOREIGN KEY (chat_id, sender_id)
        REFERENCES chat_member(chat_id, user_id) ON DELETE RESTRICT,
    ADD CONSTRAINT message_chat_seq_unique UNIQUE (chat_id, seq),
    ADD CONSTRAINT message_client_id_unique UNIQUE (chat_id, sender_id, client_message_id),
    ADD CONSTRAINT message_seq_positive CHECK (seq > 0),
    ADD CONSTRAINT message_version_positive CHECK (version > 0),
    ADD CONSTRAINT message_client_id_valid CHECK (
        client_message_id IS NULL OR
        (char_length(client_message_id) BETWEEN 1 AND 128 AND client_message_id ~ '[^[:space:]]')
    ),
    ADD CONSTRAINT message_type_valid CHECK (type IN ('text', 'voice', 'video_note', 'system')),
    ADD CONSTRAINT message_sender_type CHECK ((type = 'system') = (sender_id IS NULL)),
    ADD CONSTRAINT message_system_client_id CHECK (type <> 'system' OR client_message_id IS NULL),
    ADD CONSTRAINT message_body_valid CHECK (
        body IS NULL OR (char_length(body) BETWEEN 1 AND 10000 AND body ~ '[^[:space:]]')
    ),
    ADD CONSTRAINT message_storage_key_valid CHECK (
        storage_key IS NULL OR (char_length(storage_key) BETWEEN 1 AND 1024 AND storage_key ~ '[^[:space:]]')
    ),
    ADD CONSTRAINT message_content_type CHECK (
        (type IN ('text', 'system') AND body IS NOT NULL AND storage_key IS NULL AND duration_seconds IS NULL)
        OR
        (type IN ('voice', 'video_note') AND body IS NULL AND storage_key IS NOT NULL
            AND duration_seconds IS NOT NULL)
    ),
    ADD CONSTRAINT message_duration_valid CHECK (
        duration_seconds IS NULL OR (duration_seconds > 0 AND duration_seconds < 'Infinity'::float8)
    ),
    ADD CONSTRAINT message_edited_at_valid CHECK (
        edited_at IS NULL OR (isfinite(edited_at) AND edited_at >= created_at AND edited_at <= updated_at)
    ),
    ADD CONSTRAINT message_deleted_at_valid CHECK (
        deleted_at IS NULL OR (isfinite(deleted_at) AND deleted_at >= created_at AND deleted_at <= updated_at)
    );

-- Текст/ключ удалённого сообщения остаются в БД. API скрывает содержимое,
-- а выдача медиа обязана проверять deleted_at и участие пользователя в чате.
-- client_message_id у исторических сообщений отсутствует; новые клиентские
-- отправки обязаны передавать его через контракт сервиса.
CREATE INDEX message_unread_idx ON message(chat_id, seq) WHERE deleted_at IS NULL;

-- Блокировка пары принадлежит Matching, отличается от бана аккаунта в Auth.
CREATE TABLE user_block (
    blocker_id BIGINT NOT NULL CHECK (blocker_id > 0),
    blocked_user_id BIGINT NOT NULL CHECK (blocked_user_id > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP CHECK (isfinite(created_at)),
    PRIMARY KEY (blocker_id, blocked_user_id),
    CONSTRAINT user_block_not_self CHECK (blocker_id <> blocked_user_id)
);
CREATE INDEX user_block_target_idx ON user_block(blocked_user_id, blocker_id);

-- Одна категория может встречаться в нескольких жалобах на разные случаи.
ALTER TABLE report
    DROP CONSTRAINT report_author_id_reported_user_id_report_type_id_key,
    DROP CONSTRAINT report_author,
    DROP CONSTRAINT report_target,
    ADD COLUMN body TEXT,
    ADD COLUMN status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN reviewed_by BIGINT,
    ADD COLUMN reviewed_at TIMESTAMPTZ,
    ADD CONSTRAINT report_user_ids_positive CHECK (author_id > 0 AND reported_user_id > 0),
    ADD CONSTRAINT report_body_valid CHECK (
        body IS NULL OR (char_length(body) BETWEEN 1 AND 10000 AND body ~ '[^[:space:]]')
    ),
    ADD CONSTRAINT report_status_valid CHECK (status IN ('pending', 'accepted', 'rejected')),
    ADD CONSTRAINT report_review_state CHECK (
        (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL)
        OR
        (status IN ('accepted', 'rejected') AND reviewed_by IS NOT NULL AND reviewed_by > 0
            AND reviewed_at IS NOT NULL AND isfinite(reviewed_at)
            AND reviewed_at >= created_at AND reviewed_at <= updated_at)
    );

-- Исторические репорты не имели текста: не выдумываем его при миграции.
-- NOT VALID допускает старые NULL, но требует body у новых/обновляемых строк.
ALTER TABLE report ADD CONSTRAINT report_body_required CHECK (body IS NOT NULL) NOT VALID;

CREATE INDEX report_queue_idx ON report(status, created_at, id);
CREATE INDEX report_target_idx ON report(reported_user_id, created_at);

CREATE TABLE report_message (
    report_id BIGINT NOT NULL REFERENCES report(id) ON DELETE CASCADE,
    message_id BIGINT NOT NULL REFERENCES message(id) ON DELETE RESTRICT,
    PRIMARY KEY (report_id, message_id)
);
CREATE INDEX report_message_message_idx ON report_message(message_id);

COMMIT;
