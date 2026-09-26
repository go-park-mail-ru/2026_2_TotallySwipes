package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"dating-app/internal/model"
)

// Ключи по схеме db/normalized/relations.md:
//
//	refresh:session:<id>        - hash: user_id, token_hash, expires_at, revoked
//	refresh:hash:<token_hash>   - string: <id>
//
// У обоих ключей одинаковое абсолютное время истечения
func sessionKey(id string) string       { return "refresh:session:" + id }
func sessionHashKey(hash string) string { return "refresh:hash:" + hash }

// revokeScript помечает сессию отозванной, только если она ещё существует
var revokeScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then
	return redis.call('HSET', KEYS[1], 'revoked', '1')
end
return 0`)

type SessionRepo struct {
	rdb *redis.Client
}

func NewSessionRepository(rdb *redis.Client) *SessionRepo {
	return &SessionRepo{rdb: rdb}
}

func (r *SessionRepo) Create(ctx context.Context, s *model.Session) error {
	if s == nil {
		return fmt.Errorf("create session: input is nil")
	}

	_, err := r.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, sessionKey(s.ID),
			"user_id", s.UserID,
			"token_hash", s.TokenHash,
			"expires_at", s.ExpiresAt.Unix(),
			"revoked", boolToFlag(s.Revoked),
		)
		p.ExpireAt(ctx, sessionKey(s.ID), s.ExpiresAt)
		p.Set(ctx, sessionHashKey(s.TokenHash), s.ID, 0)
		p.ExpireAt(ctx, sessionHashKey(s.TokenHash), s.ExpiresAt)
		return nil
	})
	if err != nil {
		return fmt.Errorf("create session id=%s: %w", s.ID, err)
	}
	return nil
}

// GetByTokenHash - model.ErrNotFound, если сессии нет или она истекла
func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error) {
	id, err := r.rdb.Get(ctx, sessionHashKey(tokenHash)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session by hash: %w", err)
	}

	fields, err := r.rdb.HGetAll(ctx, sessionKey(id)).Result()
	if err != nil {
		return nil, fmt.Errorf("get session id=%s: %w", id, err)
	}
	if len(fields) == 0 {
		return nil, ErrNotFound
	}

	userID, err := strconv.ParseInt(fields["user_id"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("get session id=%s: bad user_id: %w", id, err)
	}
	expiresAt, err := strconv.ParseInt(fields["expires_at"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("get session id=%s: bad expires_at: %w", id, err)
	}

	return &model.Session{
		ID:        id,
		UserID:    userID,
		TokenHash: fields["token_hash"],
		ExpiresAt: time.Unix(expiresAt, 0),
		Revoked:   fields["revoked"] == "1",
	}, nil
}

// Revoke не считает ошибкой отсутствие сессии
func (r *SessionRepo) Revoke(ctx context.Context, id string) error {
	if err := revokeScript.Run(ctx, r.rdb, []string{sessionKey(id)}).Err(); err != nil {
		return fmt.Errorf("revoke session id=%s: %w", id, err)
	}
	return nil
}

func boolToFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
