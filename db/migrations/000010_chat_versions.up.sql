BEGIN;

ALTER TABLE chat_member
    ADD COLUMN card_version BIGINT NOT NULL DEFAULT 1 CHECK (card_version > 0);

CREATE TABLE chat_user_state (
    user_id BIGINT PRIMARY KEY CHECK (user_id > 0),
    summary_version BIGINT NOT NULL DEFAULT 1 CHECK (summary_version > 0)
);

INSERT INTO chat_user_state (user_id)
SELECT DISTINCT user_id FROM chat_member;

COMMIT;
