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

const (
	CEIncomingTopic = "cloud.events.incoming"
)

type baseEventSubscriber interface {
	SubscribeAsync(topic string, fn interface{}, transactional bool) error
}

type eventSubscriber interface {
	SubscribeAsync(topic string, flow ActionFlow) error
}

type flowEventDispatcher interface {
	eventSubscriber
}

type eventDispatcher struct {
	eb baseEventSubscriber
}

// NewEventDispatcher returns an initialized eventDispatcher
func NewEventDispatcher(eb baseEventSubscriber) flowEventDispatcher {
	return &eventDispatcher{
		eb: eb,
	}
}

// SubscribeAsync implements interface func
func (b *eventDispatcher) SubscribeAsync(topic string, flow ActionFlow) error {
	return b.eb.SubscribeAsync(topic, flow.Handle, false)
}
