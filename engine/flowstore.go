package engine

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type FlowStore interface {
	getState(string) (*ActionFlowState, error)
	setState(string, *ActionFlowState, time.Duration) error
}

type InMemoryFlowStore struct {
	AlertCache *cache.Cache
}

func NewInMemFlowStore() *InMemoryFlowStore {
	return &InMemoryFlowStore{
		AlertCache: cache.New(5*time.Minute, 1*time.Minute),
	}
}

func (i *InMemoryFlowStore) getState(key string) (*ActionFlowState, error) {
	if a, ok := i.AlertCache.Get(key); ok {
		afs := a.(ActionFlowState)
		return &afs, nil
	} else {
		return nil, nil
	}
}

func (i *InMemoryFlowStore) setState(key string, afs *ActionFlowState, ttl time.Duration) error {
	i.AlertCache.Set(key, *afs, ttl)
	return nil
}
