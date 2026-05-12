package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// PermissionFetcher reads the effective permission codes for a user from
// the database. Implemented by the sqlc-generated repository.
type PermissionFetcher interface {
	GetEffectivePermissions(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// CachedLoader implements PermissionLoader, reading through Redis with a
// fall-through to the database. The cache key is namespaced by token_version,
// so a bumped version is an automatic invalidation — no explicit DEL required.
//
// Negative caching is intentional: an unknown user yields an empty Set, also
// cached, to absorb scan attacks against random UUIDs.
type CachedLoader struct {
	rdb     *redis.Client
	fetcher PermissionFetcher
	ttl     time.Duration
}

func NewCachedLoader(rdb *redis.Client, fetcher PermissionFetcher, ttl time.Duration) *CachedLoader {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CachedLoader{rdb: rdb, fetcher: fetcher, ttl: ttl}
}

func (l *CachedLoader) cacheKey(userID uuid.UUID, tv int) string {
	return fmt.Sprintf("rbac:perms:%s:v%d", userID.String(), tv)
}

func (l *CachedLoader) LoadEffective(ctx context.Context, userID uuid.UUID, tv int) (Set, error) {
	key := l.cacheKey(userID, tv)

	// Cache hit
	if l.rdb != nil {
		raw, err := l.rdb.Get(ctx, key).Bytes()
		if err == nil {
			var codes []string
			if jerr := json.Unmarshal(raw, &codes); jerr == nil {
				return NewSet(codes), nil
			}
			// Fall through on bad JSON — treat as miss.
		} else if err != redis.Nil {
			// Redis error: log via caller, but don't fail the request.
			// Fail-open on cache; the DB lookup below is authoritative.
		}
	}

	// Miss → DB
	codes, err := l.fetcher.GetEffectivePermissions(ctx, userID)
	if err != nil {
		return Set{}, err
	}

	// Store back. Best-effort: a Redis failure must not block the request.
	if l.rdb != nil {
		if buf, jerr := json.Marshal(codes); jerr == nil {
			_ = l.rdb.Set(ctx, key, buf, l.ttl).Err()
		}
	}

	return NewSet(codes), nil
}

// Invalidate removes the cached entry for a specific (user, token_version).
// Most callers should bump token_version instead, which lets the cache key
// roll forward naturally; use this only when you want to force a re-read
// without invalidating outstanding access tokens.
func (l *CachedLoader) Invalidate(ctx context.Context, userID uuid.UUID, tv int) error {
	if l.rdb == nil {
		return nil
	}
	return l.rdb.Del(ctx, l.cacheKey(userID, tv)).Err()
}
