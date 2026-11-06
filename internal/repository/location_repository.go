package repository

import (
	"context"
	"database/sql"
	"dating-app/internal/model"
	"fmt"
)

const upsertUserLocationQuery = `
	INSERT INTO user_location (user_id, city, district, coordinates)
    VALUES (
        $1, $2, $3,
        ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography
    )
    ON CONFLICT (user_id) DO
	UPDATE SET
        city = EXCLUDED.city,
        district = EXCLUDED.district,
        coordinates = EXCLUDED.coordinates,
        updated_at = CURRENT_TIMESTAMP
		`

type LocationRepository struct {
	db *sql.DB
}

func NewLocationRepository(db *sql.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

func (r *LocationRepository) Upsert(ctx context.Context, location *model.UserLocation) error {

	_, err := r.db.ExecContext(ctx, upsertUserLocationQuery, location.UserID, location.City, location.District, location.Coordinates.Longitude, location.Coordinates.Latitude)

	if err != nil {
		return fmt.Errorf("upsert location user_id=%d: %w", location.UserID, err)
	}

	return nil
}
