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

package flows

import (
	"time"

	cache "github.com/patrickmn/go-cache"
)

type FlowStore interface {
	Get(string) (*EventFlow, error)
	Set(string, *EventFlow, time.Duration) error
	Delete(string)
}

type InMemoryFlowStore struct {
	EventFlowCache *cache.Cache
}

func NewInMemFlowStore() *InMemoryFlowStore {
	return &InMemoryFlowStore{
		EventFlowCache: cache.New(time.Duration(10)*time.Minute, time.Duration(1)*time.Minute),
	}
}

func (i *InMemoryFlowStore) Get(key string) (*EventFlow, error) {
	a, ok := i.EventFlowCache.Get(key)
	if a != nil && ok {
		ef := a.(*EventFlow)
		return ef, nil
	} else {
		return nil, nil
	}
}

func (i *InMemoryFlowStore) Set(key string, ef *EventFlow, ttl time.Duration) error {
	i.EventFlowCache.Set(key, ef, ttl)
	return nil
}

func (i *InMemoryFlowStore) Delete(key string) {
	i.EventFlowCache.Delete(key)
}
