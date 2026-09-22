CREATE TABLE "user" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "email" TEXT NOT NULL UNIQUE,
    "password_hash" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "user_id_positive" CHECK (id > 0),
    CONSTRAINT "user_name_valid" CHECK (char_length(name) BETWEEN 1 AND 100 AND name ~ '[^[:space:]]'),
    CONSTRAINT "user_email_valid" CHECK (char_length(email) BETWEEN 1 AND 254 AND email !~ '[[:space:]]'),
    CONSTRAINT "user_password_hash_nonblank" CHECK (password_hash ~ '[^[:space:]]'),
    CONSTRAINT "user_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "user_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "user_updated_not_before_created" CHECK (updated_at >= created_at)
);

CREATE TABLE "profile" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "user_id" BIGINT NOT NULL UNIQUE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "profile_id_positive" CHECK (id > 0),
    CONSTRAINT "profile_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "profile_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "profile_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "profile_owner" FOREIGN KEY ("user_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

CREATE TABLE "profile_version" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "profile_id" BIGINT NOT NULL,
    "revision" INTEGER NOT NULL,
    "recorded_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "birth_date" DATE NOT NULL,
    "sex" TEXT NOT NULL,
    "search_sex" TEXT NOT NULL,
    "search_age_from" INTEGER NOT NULL,
    "search_age_to" INTEGER NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("profile_id", "revision"),
    CONSTRAINT "profile_version_id_positive" CHECK (id > 0),
    CONSTRAINT "profile_version_revision_positive" CHECK (revision > 0),
    CONSTRAINT "profile_version_recorded_at_finite" CHECK (isfinite(recorded_at)),
    CONSTRAINT "profile_version_birth_date_finite" CHECK (isfinite(birth_date)),
    CONSTRAINT "profile_version_sex_valid" CHECK (sex IN ('male', 'female')),
    CONSTRAINT "profile_version_search_sex_valid" CHECK (search_sex IN ('male', 'female', 'all')),
    CONSTRAINT "profile_version_search_age_valid" CHECK (search_age_from >= 18 AND search_age_to >= search_age_from),
    CONSTRAINT "profile_version_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "profile_version_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "profile_version_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "version_owner" FOREIGN KEY ("profile_id")
        REFERENCES "profile" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

CREATE TABLE "test" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "test_id_positive" CHECK (id > 0),
    CONSTRAINT "test_name_valid" CHECK (char_length(name) BETWEEN 1 AND 200 AND name ~ '[^[:space:]]'),
    CONSTRAINT "test_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "test_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "test_updated_not_before_created" CHECK (updated_at >= created_at)
);

CREATE TABLE "question" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "test_id" BIGINT NOT NULL,
    "operation_id" INTEGER NOT NULL,
    "body" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "question_id_positive" CHECK (id > 0),
    CONSTRAINT "question_operation_positive" CHECK (operation_id > 0),
    CONSTRAINT "question_body_valid" CHECK (char_length(body) BETWEEN 1 AND 2000 AND body ~ '[^[:space:]]'),
    CONSTRAINT "question_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "question_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "question_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "question_test" FOREIGN KEY ("test_id")
        REFERENCES "test" ("id") ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE "profile_psycho" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "profile_id" BIGINT NOT NULL,
    "test_id" BIGINT NOT NULL,
    "revision" INTEGER NOT NULL,
    "recorded_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "weight_1" NUMERIC,
    "weight_2" NUMERIC,
    "weight_3" NUMERIC,
    "weight_4" NUMERIC,
    "weight_5" NUMERIC,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("profile_id", "revision"),
    CONSTRAINT "profile_psycho_id_positive" CHECK (id > 0),
    CONSTRAINT "profile_psycho_revision_positive" CHECK (revision > 0),
    CONSTRAINT "profile_psycho_recorded_at_finite" CHECK (isfinite(recorded_at)),
    CONSTRAINT "profile_psycho_weights_complete" CHECK (( weight_1 IS NULL AND weight_2 IS NULL AND weight_3 IS NULL AND weight_4 IS NULL AND weight_5 IS NULL ) OR ( weight_1 IS NOT NULL AND weight_2 IS NOT NULL AND weight_3 IS NOT NULL AND weight_4 IS NOT NULL AND weight_5 IS NOT NULL )),
    CONSTRAINT "profile_psycho_weight_1_finite" CHECK (weight_1 <> 'NaN'::numeric AND weight_1 <> 'Infinity'::numeric AND weight_1 <> '-Infinity'::numeric),
    CONSTRAINT "profile_psycho_weight_2_finite" CHECK (weight_2 <> 'NaN'::numeric AND weight_2 <> 'Infinity'::numeric AND weight_2 <> '-Infinity'::numeric),
    CONSTRAINT "profile_psycho_weight_3_finite" CHECK (weight_3 <> 'NaN'::numeric AND weight_3 <> 'Infinity'::numeric AND weight_3 <> '-Infinity'::numeric),
    CONSTRAINT "profile_psycho_weight_4_finite" CHECK (weight_4 <> 'NaN'::numeric AND weight_4 <> 'Infinity'::numeric AND weight_4 <> '-Infinity'::numeric),
    CONSTRAINT "profile_psycho_weight_5_finite" CHECK (weight_5 <> 'NaN'::numeric AND weight_5 <> 'Infinity'::numeric AND weight_5 <> '-Infinity'::numeric),
    CONSTRAINT "profile_psycho_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "profile_psycho_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "profile_psycho_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "psycho_owner" FOREIGN KEY ("profile_id")
        REFERENCES "profile" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "psycho_test" FOREIGN KEY ("test_id")
        REFERENCES "test" ("id") ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE "user_answer" (
    "profile_psycho_id" BIGINT NOT NULL,
    "question_id" BIGINT NOT NULL,
    "answer_value" SMALLINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("profile_psycho_id", "question_id"),
    CONSTRAINT "user_answer_value_valid" CHECK (answer_value BETWEEN 0 AND 7),
    CONSTRAINT "user_answer_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "user_answer_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "user_answer_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "answer_result" FOREIGN KEY ("profile_psycho_id")
        REFERENCES "profile_psycho" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "answer_question" FOREIGN KEY ("question_id")
        REFERENCES "question" ("id") ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE "plan" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL UNIQUE,
    "ads_enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "plan_id_positive" CHECK (id > 0),
    CONSTRAINT "plan_name_valid" CHECK (char_length(name) BETWEEN 1 AND 100 AND name ~ '[^[:space:]]'),
    CONSTRAINT "plan_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "plan_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "plan_updated_not_before_created" CHECK (updated_at >= created_at)
);

CREATE TABLE "subscription" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "user_id" BIGINT NOT NULL,
    "plan_id" BIGINT NOT NULL,
    "starts_at" TIMESTAMPTZ NOT NULL,
    "ends_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "subscription_id_positive" CHECK (id > 0),
    CONSTRAINT "subscription_period_finite" CHECK (isfinite(starts_at) AND isfinite(ends_at)),
    CONSTRAINT "subscription_period_valid" CHECK (ends_at > starts_at),
    CONSTRAINT "subscription_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "subscription_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "subscription_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "subscription_user" FOREIGN KEY ("user_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "subscription_plan" FOREIGN KEY ("plan_id")
        REFERENCES "plan" ("id") ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE "tag" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL UNIQUE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "tag_id_positive" CHECK (id > 0),
    CONSTRAINT "tag_name_valid" CHECK (char_length(name) BETWEEN 1 AND 100 AND name ~ '[^[:space:]]'),
    CONSTRAINT "tag_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "tag_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "tag_updated_not_before_created" CHECK (updated_at >= created_at)
);

CREATE TABLE "profile_tag" (
    "profile_id" BIGINT NOT NULL,
    "tag_id" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("profile_id", "tag_id"),
    CONSTRAINT "profile_tag_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "profile_tag_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "profile_tag_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "profile_tag_profile" FOREIGN KEY ("profile_id")
        REFERENCES "profile" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "profile_tag_tag" FOREIGN KEY ("tag_id")
        REFERENCES "tag" ("id") ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE "report_type" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL UNIQUE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "report_type_id_positive" CHECK (id > 0),
    CONSTRAINT "report_type_name_valid" CHECK (char_length(name) BETWEEN 1 AND 100 AND name ~ '[^[:space:]]'),
    CONSTRAINT "report_type_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "report_type_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "report_type_updated_not_before_created" CHECK (updated_at >= created_at)
);

CREATE TABLE "report" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "author_id" BIGINT NOT NULL,
    "reported_user_id" BIGINT NOT NULL,
    "report_type_id" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("author_id", "reported_user_id", "report_type_id"),
    CONSTRAINT "report_id_positive" CHECK (id > 0),
    CONSTRAINT "report_not_self" CHECK (author_id <> reported_user_id),
    CONSTRAINT "report_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "report_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "report_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "report_author" FOREIGN KEY ("author_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "report_target" FOREIGN KEY ("reported_user_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "report_type_ref" FOREIGN KEY ("report_type_id")
        REFERENCES "report_type" ("id") ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE TABLE "photo" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "profile_id" BIGINT NOT NULL,
    "storage_key" TEXT NOT NULL UNIQUE,
    "position" SMALLINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("profile_id", "position"),
    CONSTRAINT "photo_id_positive" CHECK (id > 0),
    CONSTRAINT "photo_storage_key_valid" CHECK (storage_key ~ '[^[:space:]]' AND char_length(storage_key) <= 1024),
    CONSTRAINT "photo_position_positive" CHECK (position > 0),
    CONSTRAINT "photo_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "photo_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "photo_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "photo_profile" FOREIGN KEY ("profile_id")
        REFERENCES "profile" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

CREATE TABLE "photo_comment" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "author_id" BIGINT NOT NULL,
    "photo_id" BIGINT NOT NULL,
    "body" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "photo_comment_id_positive" CHECK (id > 0),
    CONSTRAINT "photo_comment_body_valid" CHECK (char_length(body) BETWEEN 1 AND 2000 AND body ~ '[^[:space:]]'),
    CONSTRAINT "photo_comment_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "photo_comment_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "photo_comment_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "photo_comment_author" FOREIGN KEY ("author_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "photo_comment_photo" FOREIGN KEY ("photo_id")
        REFERENCES "photo" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

CREATE TABLE "profile_like" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "author_id" BIGINT NOT NULL,
    "profile_id" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("author_id", "profile_id"),
    CONSTRAINT "profile_like_id_positive" CHECK (id > 0),
    CONSTRAINT "profile_like_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "profile_like_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "profile_like_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "profile_like_author" FOREIGN KEY ("author_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "profile_like_profile" FOREIGN KEY ("profile_id")
        REFERENCES "profile" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

CREATE TABLE "match" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "first_user_id" BIGINT NOT NULL,
    "second_user_id" BIGINT NOT NULL,
    "access_to_message" BOOLEAN NOT NULL DEFAULT TRUE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("first_user_id", "second_user_id"),
    CONSTRAINT "match_id_positive" CHECK (id > 0),
    CONSTRAINT "match_users_canonical_order" CHECK (first_user_id < second_user_id),
    CONSTRAINT "match_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "match_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "match_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "match_first_user" FOREIGN KEY ("first_user_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "match_second_user" FOREIGN KEY ("second_user_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);

CREATE TABLE "message" (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY NOT NULL PRIMARY KEY,
    "match_id" BIGINT NOT NULL,
    "sender_id" BIGINT NOT NULL,
    "body" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "message_id_positive" CHECK (id > 0),
    CONSTRAINT "message_body_valid" CHECK (char_length(body) BETWEEN 1 AND 10000 AND body ~ '[^[:space:]]'),
    CONSTRAINT "message_created_at_finite" CHECK (isfinite(created_at)),
    CONSTRAINT "message_updated_at_finite" CHECK (isfinite(updated_at)),
    CONSTRAINT "message_updated_not_before_created" CHECK (updated_at >= created_at),
    CONSTRAINT "message_match" FOREIGN KEY ("match_id")
        REFERENCES "match" ("id") ON DELETE CASCADE ON UPDATE RESTRICT,
    CONSTRAINT "message_sender" FOREIGN KEY ("sender_id")
        REFERENCES "user" ("id") ON DELETE CASCADE ON UPDATE RESTRICT
);
