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
	"path"
	"time"

	"github.com/goph/emperror"
	"github.com/goph/logur"
	"github.com/pkg/errors"

	"github.com/banzaicloud/hollowtrees/internal/ce"
)

// ActionFlow defines an action flow
type ActionFlow interface {
	Handle(event interface{})
}

// Flow describes an action flow
type Flow struct {
	id            string
	name          string
	description   string
	allowedEvents []string
	cooldown      time.Duration
	groupBy       []string
	plugins       []string
	filters       map[string]string

	cache   FlowStore
	manager FlowManager
}

// NewFlow returns an initialized action flow
func NewFlow(manager FlowManager, cache FlowStore, id string, name string, opts ...Option) *Flow {
	f := &Flow{
		id:   id,
		name: name,

		manager: manager,
		cache:   cache,
	}

	for _, o := range opts {
		o.apply(f)
	}

	return f
}

// Handle handles the event by starting and event flow which executes the defined plugins
func (f *Flow) Handle(event interface{}) {
	e, ok := event.(*ce.Event)
	if !ok {
		f.manager.ErrorHandler().Handle(emperror.With(errors.Errorf("invalid event value: %#v", event)))
		return
	}

	err := f.handleEvent(e)
	if err != nil {
		f.manager.ErrorHandler().Handle(emperror.WrapWith(err, "could not handle event", "type", e.Type, "id", e.ID, "flow", f.name))
	}
}

func (f *Flow) handleEvent(event *ce.Event) error {
	key := f.getEventKey(event, f.groupBy)
	cid, _ := event.GetString("correlationid")
	log := f.manager.Logger().WithFields(logur.Fields{
		"correlation-id": cid,
		"event-id":       event.ID,
		"flow-id":        f.id,
		"type":           event.Type,
		"group-key":      key,
	})

	if !f.isEventTypeAllowed(event.Type) {
		log.Debug("skip flow - disallowed event type")
		return nil
	}

	if !f.isEventMatched(event) {
		log.Debug("skip flow - filter does not match")
		return nil
	}

	ef, created, err := f.createOrGetEventFlow(event, key)
	if err != nil {
		return err
	}

	if created {
		log.Debugf("executing event flow - %s", ef.Status)
		err = ef.Exec()
		if err != nil {
			return err
		}
		f.cache.Delete(key)
	}

	return nil
}

func (f *Flow) createOrGetEventFlow(event *ce.Event, key string) (*EventFlow, bool, error) { // f.mux.Lock()
	created := false

	ef, err := f.cache.Get(key)
	if err != nil {
		return nil, created, err
	}

	if ef != nil && ef.Status == EventFlowCompleted {
		ef = nil
	}

	if ef == nil {
		ef = NewEventFlow(f, event)
		err := f.cache.Set(key, ef, f.cooldown+time.Duration(5)*time.Minute)
		if err != nil {
			return nil, created, err
		}
		created = true
	}

	return ef, created, nil
}

func (f *Flow) getEventKey(event *ce.Event, groupBy []string) string {
	key := event.Type

	grouped := false
	for _, g := range groupBy {
		if s, ok := event.GetString(g); ok {
			grouped = true
			key = path.Join(key, s)
		}
	}

	if !grouped {
		return path.Join(key, event.ID)
	}

	return key
}

func (f *Flow) isEventMatched(event *ce.Event) bool {
	if len(f.filters) == 0 {
		return true
	}

	for key, value := range f.filters {
		if v, ok := event.GetString(key); !ok || v != value {
			return false
		}
	}

	return true
}

func (f *Flow) isEventTypeAllowed(eventType string) bool {
	if len(f.allowedEvents) == 0 {
		return true
	}

	for _, t := range f.allowedEvents {
		if t == eventType {
			return true
		}
	}

	return false
}
