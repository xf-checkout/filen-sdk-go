package filen

import (
	"context"
	"errors"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"github.com/google/uuid"
	"sync"
	"time"
)

type BackendLock struct {
	mu           types.CtxMutex
	count        int
	lockUUID     string
	cancelTicker chan struct{}

	muPoisoned sync.RWMutex
	poisoned   bool
}

const (
	resourceName       = "drive-write"
	maxLockAttempts    = 100
	retryLockSleepTime = 1000 * time.Millisecond
	refreshInterval    = 20 * time.Second
)

var (
	FailedToReleaseLock = errors.New("failed to release lock")
	LockPoisoned        = errors.New("lock refresh failed")
)

func NewBackendLock() BackendLock {
	return BackendLock{
		mu:           types.NewCtxMutex(),
		cancelTicker: make(chan struct{}, 1),
	}
}

func (api *Filen) acquireBackendLock(ctx context.Context) error {
	req := client.V3UserLockRequest{
		LockUUID: uuid.NewString(),
		Type:     "acquire",
		Resource: resourceName,
	}
	for i := 0; i < maxLockAttempts; i++ {
		resp, err := api.Client.PostV3UserLock(ctx, req)
		if err != nil {
			return err
		}
		if resp.Acquired {
			break
		}
		time.Sleep(retryLockSleepTime)
	}
	api.lock.lockUUID = req.LockUUID

	// new lock acquired, reset the poison flag
	api.lock.muPoisoned.Lock()
	api.lock.poisoned = false
	api.lock.muPoisoned.Unlock()

	ticker := time.NewTicker(refreshInterval)
	go api.refreshLockHandler(ticker)
	return nil
}

func (api *Filen) refreshLockHandler(ticker *time.Ticker) {
	for {
		select {
		case <-ticker.C:
			resp, err := api.Client.PostV3UserLock(context.Background(), client.V3UserLockRequest{
				LockUUID: api.lock.lockUUID,
				Type:     "refresh",
				Resource: resourceName,
			})
			if err == nil && resp.Refreshed {
				continue
			}
			api.lock.muPoisoned.Lock()
			api.lock.poisoned = true
			api.lock.muPoisoned.Unlock()
			return
		case <-api.lock.cancelTicker:
			return
		}
	}
}

func (api *Filen) releaseBackendLock() {
	api.lock.cancelTicker <- struct{}{}
	// we use context.Background here because this function should always be executed
	// even if the original context is cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := api.Client.PostV3UserLock(ctx, client.V3UserLockRequest{
		LockUUID: api.lock.lockUUID,
		Type:     "release",
		Resource: resourceName,
	})
	api.lock.lockUUID = ""
	if err != nil || !resp.Released {
		api.lock.muPoisoned.Lock()
		api.lock.poisoned = true
		api.lock.muPoisoned.Unlock()
	}
}

func (api *Filen) Lock(ctx context.Context) error {
	err := api.lock.mu.Lock(ctx)
	if err != nil {
		return err
	}
	defer api.lock.mu.Unlock()

	if api.lock.count == 0 {
		err = api.acquireBackendLock(ctx)
		if err != nil {
			return err
		}
	} else {
		api.lock.muPoisoned.RLock()
		if api.lock.poisoned {
			api.lock.muPoisoned.RUnlock()
			return LockPoisoned
		}
		api.lock.muPoisoned.RUnlock()
	}
	
	api.lock.count++
	return nil
}

func (api *Filen) Unlock() {
	// we use BlockUntilLock here because this function should always be executed
	api.lock.mu.BlockUntilLock()
	defer api.lock.mu.Unlock()
	api.lock.count--
	if api.lock.count == 0 {
		api.releaseBackendLock()
	}
}
