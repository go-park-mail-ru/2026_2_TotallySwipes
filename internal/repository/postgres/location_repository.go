package postgres

import (
	"context"
	"database/sql"
	"dating-app/internal/model"
	"errors"
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

const getUserLocationQuery = `
 SELECT user_id, city, district,
        ST_X(coordinates::geometry) AS longitude,
        ST_Y(coordinates::geometry) AS latitude
 FROM user_location
 WHERE user_id = $1
`

type LocationRepository struct {
	db *sql.DB
}

func NewLocationRepository(db *sql.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

// Upsert создаёт или обновляет город, район и координаты пользователя
func (r *LocationRepository) Upsert(ctx context.Context, location *model.UserLocation) error {

	_, err := r.db.ExecContext(ctx, upsertUserLocationQuery, location.UserID, location.City, location.District, location.Coordinates.Longitude, location.Coordinates.Latitude)

	if err != nil {
		return fmt.Errorf("upsert location user_id=%d: %w", location.UserID, err)
	}

	return nil
}

// GetLocation возвращает сохранённую локацию пользователя; если её нет — nil без ошибки
func (r *LocationRepository) GetLocation(ctx context.Context, userID int64) (*model.UserLocation, error) {

	var location model.UserLocation
	err := r.db.QueryRowContext(ctx, getUserLocationQuery, userID).Scan(&location.UserID, &location.City, &location.District, &location.Coordinates.Longitude, &location.Coordinates.Latitude)

	if err != nil {

		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("get location user_id=%d: %w", userID, err)
	}

	return &location, nil
}
