BEGIN;

ALTER TABLE "match"
    DROP COLUMN "first_user_seen_at",
    DROP COLUMN "second_user_seen_at";

COMMIT;
