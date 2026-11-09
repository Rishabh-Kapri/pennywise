package factory

import (
	"context"
	"sync"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sharedPool struct {
	pool *pgxpool.Pool
	refs int
}

var (
	poolsMu sync.Mutex
	pools   = map[string]*sharedPool{}
)

func acquirePool(ctx context.Context, url string) (*pgxpool.Pool, func(), error) {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	sp, ok := pools[url]
	if !ok {
		p, err := postgres.ConnectWithURL(ctx, url)
		if err != nil {
			return nil, nil, err
		}
		sp = &sharedPool{pool: p}
		pools[url] = sp
	}
	sp.refs++
	var once sync.Once
	release := func() {
		once.Do(func() {
			poolsMu.Lock()
			defer poolsMu.Unlock()
			if sp.refs--; sp.refs == 0 {
				sp.pool.Close()
				delete(pools, url)
			}
		})
	}
	return sp.pool, release, nil
}
