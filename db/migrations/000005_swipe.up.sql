BEGIN;

-- Замена таблицы без переноса существующих лайков.
DROP TABLE "profile_like";

CREATE TABLE "swipe" (
    "target_profile_id" BIGINT NOT NULL,
    "author_id" BIGINT NOT NULL,
    "action" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("author_id", "target_profile_id"),
    CONSTRAINT "swipe_action_valid" CHECK (action IN ('like', 'dislike', 'super_like')),
    CONSTRAINT "swipe_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "swipe_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "swipe_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "swipe_author" FOREIGN KEY ("author_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "swipe_profile" FOREIGN KEY ("target_profile_id")
        REFERENCES "profile" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

COMMIT;
