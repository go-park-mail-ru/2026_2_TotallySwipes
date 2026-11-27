package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"dating-app/services/matching/internal/model"
)

type FeedRepository struct{ db *sql.DB }

func NewFeedRepository(db *sql.DB) *FeedRepository {

	return &FeedRepository{db: db}
}

const selectSwipeStatesQuery = `
    SELECT candidates.user_id,
           EXISTS (SELECT 1 FROM swipe WHERE author_id = $1 AND target_profile_id = candidates.profile_id),
           EXISTS (SELECT 1 FROM swipe WHERE author_id = candidates.user_id AND target_profile_id = $4 AND action IN ('like', 'super_like'))
    FROM unnest($2::bigint[], $3::bigint[]) AS candidates(user_id, profile_id)`

func (r *FeedRepository) GetSwipeStates(ctx context.Context, userID, profileID int64, candidates []model.Profile) (map[int64]model.SwipeState, error) {

	result := make(map[int64]model.SwipeState, len(candidates))

	if len(candidates) == 0 {
		return result, nil
	}

	ids, profileIDs := make([]int64, 0, len(candidates)), make([]int64, 0, len(candidates))

	for _, candidate := range candidates {
		if candidate.ID <= 0 {
			return nil, fmt.Errorf("proto: missing candidate profile_id")
		}

		ids = append(ids, candidate.UserID)
		profileIDs = append(profileIDs, candidate.ID)
	}

	rows, err := r.db.QueryContext(ctx, selectSwipeStatesQuery, userID, ids, profileIDs, profileID)

	if err != nil {
		return nil, fmt.Errorf("get swipe states: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var id int64
		var state model.SwipeState

		if err := rows.Scan(&id, &state.Swiped, &state.LikedMe); err != nil {
			return nil, fmt.Errorf("scan swipe state: %w", err)
		}

		result[id] = state
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read swipe states: %w", err)
	}

	return result, nil
}
