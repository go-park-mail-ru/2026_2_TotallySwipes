BEGIN;

ALTER TABLE "profile_version"
    ADD COLUMN "education" TEXT,
    ADD COLUMN "work" TEXT,
    ADD COLUMN "smoking" TEXT,
    ADD COLUMN "alcohol" TEXT,
    ADD COLUMN "height" SMALLINT,
    ADD CONSTRAINT "profile_version_education_valid" CHECK (education IN ('secondary', 'vocational', 'incomplete_higher', 'higher', 'degree')),
    ADD CONSTRAINT "profile_version_work_valid" CHECK (char_length(work) BETWEEN 1 AND 100 AND work ~ '[^[:space:]]'),
    ADD CONSTRAINT "profile_version_smoking_valid" CHECK (smoking IN ('negative', 'neutral', 'positive')),
    ADD CONSTRAINT "profile_version_alcohol_valid" CHECK (alcohol IN ('negative', 'neutral', 'positive')),
    ADD CONSTRAINT "profile_version_height_valid" CHECK (height BETWEEN 100 AND 250);

COMMIT;
