BEGIN;

ALTER TABLE message
    DROP CONSTRAINT message_media_sha256_valid,
    DROP COLUMN media_sha256;

COMMIT;
