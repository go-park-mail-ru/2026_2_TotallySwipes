package repository

import (
	"context"
	"dating-app/internal/model"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type EventRepository struct {
	rdb *redis.Client
	ttl time.Duration
}

func searchKey(searchID uuid.UUID) string {
	return "place-search:" + searchID.String()
}

func activeSearchKey(matchID int64) string {
	return fmt.Sprintf("place-search:active:%d", matchID)
}

const searchTransactionAttempts = 8

func NewEventRepository(rdb *redis.Client, ttl time.Duration) *EventRepository {
	return &EventRepository{rdb: rdb, ttl: ttl}
}

func (r *EventRepository) GetActive(ctx context.Context, matchID int64) (*model.PlaceSearch, error) {

	searchID, err := r.rdb.Get(ctx, activeSearchKey(matchID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	id, err := uuid.Parse(searchID)
	if err != nil {
		return nil, fmt.Errorf("parse active search ID: %w", err)
	}

	search, err := r.GetBySearchID(ctx, id)
	if errors.Is(err, model.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("get active search: %w", err)
	}

	return search, nil
}

func (r *EventRepository) GetBySearchID(ctx context.Context, searchID uuid.UUID) (*model.PlaceSearch, error) {

	data, err := r.rdb.Get(ctx, searchKey(searchID)).Bytes()

	if errors.Is(err, redis.Nil) {
		return nil, model.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var search model.PlaceSearch
	err = json.Unmarshal(data, &search)

	if err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	return &search, nil
}

func (r *EventRepository) CreateSearch(ctx context.Context, search *model.PlaceSearch) (*model.PlaceSearch, bool, error) {

	if search == nil || r.ttl <= 0 {
		return nil, false, fmt.Errorf("create search: search is required and ttl must be positive")
	}

	activeKey := activeSearchKey(search.MatchID)
	for range searchTransactionAttempts {

		var actual *model.PlaceSearch
		created := false

		err := r.rdb.Watch(ctx, func(tx *redis.Tx) error {

			activeID, err := tx.Get(ctx, activeKey).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}

			if err == nil {

				id, err := uuid.Parse(activeID)
				if err != nil {
					return fmt.Errorf("parse active search: %w", err)
				}

				key := searchKey(id)
				if err := tx.Watch(ctx, key).Err(); err != nil {
					return err
				}

				data, err := tx.Get(ctx, key).Bytes()
				if err != nil && !errors.Is(err, redis.Nil) {
					return err
				}

				if err == nil {

					var existing model.PlaceSearch
					if err := json.Unmarshal(data, &existing); err != nil {
						return err
					}

					if existing.MatchID != search.MatchID {
						return fmt.Errorf("active search match mismatch")
					}

					if existing.Status == model.SearchStatusVoting && existing.ExpiresAt.After(time.Now()) {

						actual = &existing
						_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
							pipe.Get(ctx, activeKey)
							return nil
						})

						return err
					}
				}
			}

			candidate := *search
			candidate.ExpiresAt = time.Now().Add(r.ttl).Truncate(time.Millisecond)
			data, err := json.Marshal(candidate)

			if err != nil {
				return err
			}

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				args := redis.SetArgs{ExpireAt: candidate.ExpiresAt}
				pipe.SetArgs(ctx, searchKey(candidate.SearchID), data, args)
				pipe.SetArgs(ctx, activeKey, candidate.SearchID.String(), args)
				return nil
			})

			if err == nil {
				actual = &candidate
				created = true
			}

			return err

		}, activeKey)

		if errors.Is(err, redis.TxFailedErr) {
			continue
		}

		if err != nil {
			return nil, false, fmt.Errorf("create search: %w", err)
		}

		return actual, created, nil
	}

	return nil, false, fmt.Errorf("create search retries exhausted: %w", redis.TxFailedErr)
}

func (r *EventRepository) SubmitResults(ctx context.Context, userID, matchID int64, searchID uuid.UUID, likedEvents []int64) (*model.PlaceSearch, error) {

	key, activeKey := searchKey(searchID), activeSearchKey(matchID)
	for range searchTransactionAttempts {

		var actual *model.PlaceSearch
		err := r.rdb.Watch(ctx, func(tx *redis.Tx) error {

			data, err := tx.Get(ctx, key).Bytes()
			if errors.Is(err, redis.Nil) {
				return model.ErrNotFound
			}

			if err != nil {
				return err
			}

			var search model.PlaceSearch
			if err := json.Unmarshal(data, &search); err != nil {
				return err
			}

			if search.MatchID != matchID || search.SearchID != searchID || !search.ExpiresAt.After(time.Now()) {
				return model.ErrNotFound
			}

			index := -1
			for i, participant := range search.Participants {
				if participant.UserID == userID {
					index = i
					break
				}
			}

			if index == -1 {
				return model.ErrNotFound
			}

			available := make(map[int64]bool, len(search.Events))
			for _, event := range search.Events {
				available[event.ID] = true
			}

			likes := make(map[int64]bool, len(likedEvents))
			for _, id := range likedEvents {
				if !available[id] || likes[id] {
					return model.ErrInvalidEventResults
				}

				likes[id] = true
			}

			participant := &search.Participants[index]
			if participant.Submitted {
				return model.ErrResultAlreadySubmitted
			}

			if search.Status != model.SearchStatusVoting {
				return model.ErrSearchStateConflict
			}

			participant.Submitted = true
			participant.LikedEventsIDs = append([]int64{}, likedEvents...)
			completed := search.Participants[0].Submitted && search.Participants[1].Submitted

			if completed {
				search.Status = model.SearchStatusCompleted
			}

			activeID, err := tx.Get(ctx, activeKey).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}

			updated, err := json.Marshal(search)
			if err != nil {
				return err
			}

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.SetArgs(ctx, key, updated, redis.SetArgs{KeepTTL: true})
				if completed && activeID == searchID.String() {
					pipe.Del(ctx, activeKey)
				}
				return nil
			})

			if err == nil {
				actual = &search

			}

			return err
		}, key, activeKey)

		if errors.Is(err, redis.TxFailedErr) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("submit results: %w", err)
		}

		return actual, nil
	}

	return nil, fmt.Errorf("submit results retries exhausted: %w", redis.TxFailedErr)
}
