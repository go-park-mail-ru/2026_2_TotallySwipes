BEGIN;

ALTER TABLE "profile_version"
    DROP COLUMN "education",
    DROP COLUMN "work",
    DROP COLUMN "smoking",
    DROP COLUMN "alcohol",
    DROP COLUMN "height";

COMMIT;
