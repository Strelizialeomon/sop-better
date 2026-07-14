package task

import (
	"context"
	"sync"
	"time"
)

type leaseHeartbeat struct {
	service LeaseService
	mu      sync.Mutex
	claim   StoredClaim
	err     error
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

func startLeaseHeartbeat(
	ctx context.Context,
	cancel context.CancelFunc,
	service LeaseService,
	claim StoredClaim,
	interval time.Duration,
) *leaseHeartbeat {
	heartbeat := &leaseHeartbeat{
		service: service, claim: claim, stop: make(chan struct{}), done: make(chan struct{}),
	}
	go func() {
		defer close(heartbeat.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeat.stop:
				return
			case <-ticker.C:
				if _, err := heartbeat.Renew(ctx); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	return heartbeat
}

func (heartbeat *leaseHeartbeat) Renew(ctx context.Context) (StoredClaim, error) {
	heartbeat.mu.Lock()
	defer heartbeat.mu.Unlock()
	if heartbeat.err != nil {
		return heartbeat.claim, heartbeat.err
	}
	renewed, err := heartbeat.service.Renew(ctx, heartbeat.claim)
	if err != nil {
		heartbeat.err = err
		return heartbeat.claim, err
	}
	heartbeat.claim = renewed
	return renewed, nil
}

func (heartbeat *leaseHeartbeat) Stop() (StoredClaim, error) {
	heartbeat.once.Do(func() { close(heartbeat.stop) })
	<-heartbeat.done
	heartbeat.mu.Lock()
	defer heartbeat.mu.Unlock()
	return heartbeat.claim, heartbeat.err
}
