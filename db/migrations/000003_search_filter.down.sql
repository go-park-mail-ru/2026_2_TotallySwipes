BEGIN;

-- Версии анкеты неизменяемы, поэтому фильтры в них не возвращаются.
ALTER TABLE "profile_version"
    ADD COLUMN "search_sex" TEXT,
    ADD COLUMN "search_age_from" INTEGER,
    ADD COLUMN "search_age_to" INTEGER,
    ADD CONSTRAINT "profile_version_search_sex_valid" CHECK (search_sex IN ('male', 'female', 'all')),
    ADD CONSTRAINT "profile_version_search_age_valid" CHECK (search_age_from >= 18 AND search_age_to >= search_age_from),
    ADD CONSTRAINT "profile_version_search_age_pair" CHECK ((search_age_from IS NULL) = (search_age_to IS NULL));

DROP TABLE "search_filter";

COMMIT;
