package repository

import (
	"context"
	"database/sql"
	"dating-app/internal/model"
	"errors"
)

type MatchRepository struct {
	db *sql.DB
}

func NewMatchRepository(db *sql.DB) *MatchRepository {
	return &MatchRepository{db: db}
}

const getParticipantsQuery = `	
							SELECT user1_id, user2_id 
							FROM match
							WHERE id = $1 AND (user1_id = $2 OR user2_id = $2);`

func (r *MatchRepository) GetParticipants(ctx context.Context, userID, matchID int64) ([2]int64, error) {

	var partisipants [2]int64
	err := r.db.QueryRowContext(ctx, getParticipantsQuery, matchID, userID).Scan(&partisipants[0], &partisipants[1])

	if errors.Is(err, sql.ErrNoRows) {
		return [2]int64{}, model.ErrNotFound
	}

	return partisipants, nil
}
