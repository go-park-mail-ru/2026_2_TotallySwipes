BEGIN;

DROP TABLE chat_user_state;
ALTER TABLE chat_member DROP COLUMN card_version;

COMMIT;
