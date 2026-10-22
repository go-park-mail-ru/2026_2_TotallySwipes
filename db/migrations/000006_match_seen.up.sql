BEGIN;

-- Время, когда участник впервые открыл мэтч; NULL - мэтч для него новый.
ALTER TABLE "match"
    ADD COLUMN "first_user_seen_at" TIMESTAMPTZ,
    ADD COLUMN "second_user_seen_at" TIMESTAMPTZ,
    ADD CONSTRAINT "match_first_user_seen_at_finite" CHECK (isfinite(first_user_seen_at)),
    ADD CONSTRAINT "match_second_user_seen_at_finite" CHECK (isfinite(second_user_seen_at));

COMMIT;
