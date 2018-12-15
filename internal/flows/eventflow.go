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

	"github.com/banzaicloud/hollowtrees/internal/ce"
)

const (
	EventFlowCompleted   EventFlowStatus = "completed"
	EventFlowFailed      EventFlowStatus = "failed"
	EventFlowInProgress  EventFlowStatus = "inprogress"
	EventFlowInitialized EventFlowStatus = "initialized"
	EventFlowCoolingDown EventFlowStatus = "coolingdown"
)

type EventFlowStatus string

// EventFlow is an actual sequential executing of defined plugins
// for a particular event and a defined action flow
type EventFlow struct {
	Status EventFlowStatus
	Error  error

	flow  *Flow
	event *ce.Event
}

// NewEventFlow returns an initialized EventFlow
func NewEventFlow(flow *Flow, event *ce.Event) *EventFlow {
	return &EventFlow{
		Status: EventFlowInitialized,

		flow:  flow,
		event: event,
	}
}

// Exec executes the defined plugins sequentially
func (ef *EventFlow) Exec() error {
	ef.Status = EventFlowInProgress

	plugins, err := ef.flow.manager.Plugins().GetByNames(ef.flow.plugins...)
	if err != nil {
		ef.Status = EventFlowFailed
		ef.Error = err
		return err
	}

	for _, plugin := range plugins {
		err := plugin.Handle(ef.event)
		if err != nil {
			ef.flow.manager.ErrorHandler().Handle(err)
		}
	}

	ef.Status = EventFlowCoolingDown

	time.Sleep(ef.flow.cooldown)

	ef.Status = EventFlowCompleted

	return nil
}
