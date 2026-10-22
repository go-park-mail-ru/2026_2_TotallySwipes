BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE "user_location" (
    user_id BIGINT PRIMARY KEY REFERENCES "user"(id) ON DELETE CASCADE,
    сity text NOT NULL CHECK (btrim(city) <> ''),
    district text CHECK (district IS NULL OR btrim(district) <> ''),
    coordinates geography(Point, 4326),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_location_dates CHECK (created_at <= updated_at)
);

CREATE INDEX user_location_coordinates_gist ON user_location USING GIST (coordinates);

COMMIT;