package factory

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/redis"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/queue"

	goredis "github.com/redis/go-redis/v9"
)

// QueueDriverRedis is the default (and today the only) queue driver.
const QueueDriverRedis = "redis"

// QueueConfig selects and locates the broker behind the queue port.
type QueueConfig struct {
	// Driver picks the adapter. Empty means QueueDriverRedis.
	Driver string
	URL    string
}

func (c QueueConfig) driver() string {
	d := strings.ToLower(strings.TrimSpace(c.Driver))
	if d == "" {
		return QueueDriverRedis
	}
	return d
}

// QueueConn is an open queue: the port, plus what is needed to close it.
type QueueConn struct {
	queue.Queue

	close func() error
}

// Close releases the connection. Call it after every Consume has returned.
func (c *QueueConn) Close() error {
	if c.close != nil {
		return c.close()
	}
	return nil
}

// ConnectQueue connects to the broker cfg names and returns the queue port.
func ConnectQueue(ctx context.Context, cfg QueueConfig) (*QueueConn, error) {
	switch cfg.driver() {
	case QueueDriverRedis:
		if cfg.URL == "" {
			return nil, fmt.Errorf("queue: REDIS_URL is required")
		}
		opts, err := goredis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("queue: parse redis URL: %w", err)
		}
		client := goredis.NewClient(opts)
		if err := client.Ping(ctx).Err(); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("queue: ping redis: %w", err)
		}
		return &QueueConn{Queue: redis.NewQueue(client, redis.QueueOptions{}), close: client.Close}, nil
	default:
		return nil, fmt.Errorf("unsupported queue driver %q", cfg.Driver)
	}
}
