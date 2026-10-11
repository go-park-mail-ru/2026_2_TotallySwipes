BEGIN;

-- Откат восстанавливает структуру, но не удалённые данные.
DROP TABLE "swipe";

CREATE TABLE "profile_like" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "author_id" BIGINT NOT NULL,
    "profile_id" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("author_id", "profile_id"),
    CONSTRAINT "profile_like_id_positive" CHECK (id > 0),
    CONSTRAINT "profile_like_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "profile_like_author" FOREIGN KEY ("author_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "profile_like_profile" FOREIGN KEY ("profile_id")
        REFERENCES "profile" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

CREATE TRIGGER profile_like_no_update
    BEFORE UPDATE ON profile_like
    FOR EACH ROW EXECUTE FUNCTION reject_history_update();

CREATE TRIGGER profile_like_no_delete
    BEFORE DELETE ON profile_like
    FOR EACH ROW EXECUTE FUNCTION protect_history_delete();

COMMIT;
