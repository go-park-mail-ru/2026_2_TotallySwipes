BEGIN;

CREATE TABLE "search_filter" (
    "user_id" BIGINT NOT NULL PRIMARY KEY,
    "sex" TEXT NOT NULL,
    "age_from" SMALLINT NOT NULL,
    "age_to" SMALLINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "search_filter_sex_valid" CHECK (sex IN ('male', 'female', 'all')),
    CONSTRAINT "search_filter_age_valid" CHECK (age_from >= 18 AND age_to <= 100 AND age_to >= age_from),
    CONSTRAINT "search_filter_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "search_filter_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "search_filter_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "search_filter_owner" FOREIGN KEY ("user_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

-- Переносит фильтр из последней версии анкеты; значения вне 18..100 не переносятся.
INSERT INTO "search_filter" (user_id, sex, age_from, age_to)
SELECT p.user_id, v.search_sex, v.search_age_from, v.search_age_to
FROM profile AS p
JOIN LATERAL (
    SELECT * FROM profile_version WHERE profile_id = p.id ORDER BY revision DESC LIMIT 1
) AS v ON true
WHERE v.search_sex IS NOT NULL AND v.search_age_to <= 100;

ALTER TABLE "profile_version"
    DROP COLUMN "search_sex",
    DROP COLUMN "search_age_from",
    DROP COLUMN "search_age_to";

COMMIT;
