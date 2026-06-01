package service

import (
	"context"
	"fmt"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/redis/go-redis/v9"
)

type RateLimitService interface {
	Check(ctx context.Context, keyHash string, limit int64) (*RateLimitResult, error)
}

type rateLimitService struct {
	client *redis.Client
	window time.Duration
}

// Implements a sliding window rate limiting using Redis
func NewRateLimitService(redisClient *redis.Client) RateLimitService {
	return &rateLimitService{
		client: redisClient,
		window: time.Minute, // 1 minute sliding window
	}
}

// rate limit check result
type RateLimitResult struct {
	Allowed    bool
	Remaining  int64
	ResetAt    time.Time
	RetryAfter time.Duration
}

func (s *rateLimitService) Check(ctx context.Context, keyHash string, limit int64) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-s.window)
	key := fmt.Sprintf("rateLimit:%s", keyHash)

	// lua script
	script := redis.NewScript(`
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local window_ms = tonumber(ARGV[4])


		-- Remove old entries outside the window
		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

		-- Count current requests in window
		local count = redis.call('ZCARD', key)

		if count < limit then
		  -- Add new request with current timestamp
		  redis.call('ZADD', key, now, now .. '-' .. math.random())
		  -- Set expiry on the key
		  redis.call('PEXPIRE', key, window_ms)
		  return {1, limit - count - 1, 0}
		else
		  -- Get oldest entry to calculate retry time
		  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
		  local retry_after = 0
		  if #oldest > 0 then
		    retry_after = oldest[2] + window_ms - now
		  end
		  return {0, 0, retry_after}
		end
		`)
	result, err := script.Run(ctx, s.client, []string{key},
		now.UnixMilli(),
		windowStart.UnixMilli(),
		limit,
		s.window.Milliseconds(),
	).Slice()
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "rate limit check failed", err)
	}

	allowed := result[0].(int64) == 1
	remaining := result[1].(int64)
	retryAfterMs := result[2].(int64)

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		ResetAt:    now.Add(s.window),
		RetryAfter: time.Duration(retryAfterMs) * time.Millisecond,
	}, nil
}
