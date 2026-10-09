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

// sessionKey формирует ключ данных refresh-сессии в Redis.
// Принимает: ID сессии id.
// Возвращает: строку refresh:session:<id>.
func sessionKey(id string) string { return "refresh:session:" + id }

// sessionHashKey формирует ключ индекса refresh-токена в Redis.
// Принимает: хеш токена hash.
// Возвращает: строку refresh:hash:<hash>.
func sessionHashKey(hash string) string { return "refresh:hash:" + hash }

// revokeScript отзывает сессию, только если она ещё активна. Несуществующий
// (в том числе истёкший) ключ не создаётся: HGET вернёт nil, а не '0'
var revokeScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'revoked') == '0' then
	redis.call('HSET', KEYS[1], 'revoked', '1')
	return 1
end
return 0`)

type SessionRepo struct {
	rdb *redis.Client
}

// NewSessionRepository создаёт репозиторий refresh-сессий.
// Принимает: клиент Redis rdb.
// Возвращает: экземпляр SessionRepo.
func NewSessionRepository(rdb *redis.Client) *SessionRepo {
	return &SessionRepo{rdb: rdb}
}

// Create сохраняет сессию и индекс её токена в Redis с одинаковым сроком истечения.
// Принимает: контекст ctx и данные сессии s.
// Возвращает: nil при успехе или ошибку записи.
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

// GetByTokenHash находит refresh-сессию по хешу токена.
// Принимает: контекст ctx и хеш tokenHash.
// Возвращает: сессию или ошибку; если ключи отсутствуют, в том числе после истечения TTL, — model.ErrNotFound.
func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error) {
	id, err := r.rdb.Get(ctx, sessionHashKey(tokenHash)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session by hash: %w", err)
	}

	fields, err := r.rdb.HGetAll(ctx, sessionKey(id)).Result()
	if err != nil {
		return nil, fmt.Errorf("get session id=%s: %w", id, err)
	}
	if len(fields) == 0 {
		return nil, model.ErrNotFound
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

// Revoke атомарно отзывает сессию, если она ещё активна.
// Принимает: контекст ctx и ID сессии id.
// Возвращает: true, если именно этот вызов отозвал сессию; false, если она уже
// отозвана или не существует; при сбое Redis — ошибку.
func (r *SessionRepo) Revoke(ctx context.Context, id string) (bool, error) {
	n, err := revokeScript.Run(ctx, r.rdb, []string{sessionKey(id)}).Int()
	if err != nil {
		return false, fmt.Errorf("revoke session id=%s: %w", id, err)
	}
	return n == 1, nil
}

// boolToFlag преобразует логическое значение в строковый флаг Redis.
// Принимает: логическое значение b.
// Возвращает: строку «1» для true или «0» для false.
func boolToFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
