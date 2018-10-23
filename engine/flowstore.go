// Copyright © 2018 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
