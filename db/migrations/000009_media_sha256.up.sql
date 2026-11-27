BEGIN;

ALTER TABLE message
    ADD COLUMN media_sha256 TEXT,
    ADD CONSTRAINT message_media_sha256_valid CHECK (
        media_sha256 IS NULL OR (
            type IN ('voice', 'video_note') AND media_sha256 ~ '^[0-9a-f]{64}$'
        )
    );

COMMIT;
