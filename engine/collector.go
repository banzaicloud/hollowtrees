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
	"fmt"

	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/satori/go.uuid"
)

type Collector struct {
	Requests chan action.AlertEvent
}

func NewCollector(requests chan action.AlertEvent) *Collector {
	return &Collector{
		Requests: requests,
	}
}

func (c *Collector) Collect(alerts []types.Alert) {
	for _, alert := range alerts {
		event := c.Convert(alert)
		log.WithField("eventId", event.EventId).Debugf("Pushing event to queue: %#v", event)
		c.Requests <- *event
	}
}

func (c *Collector) Convert(alert types.Alert) *action.AlertEvent {
	event := &action.AlertEvent{
		EventId:   uuid.NewV4().String(),
		EventType: fmt.Sprintf("prometheus.server.alert.%s", alert.Labels["alertname"]),
		Resource: &action.Resource{
			ResourceType: "prometheus.server",
			ResourceId:   alert.GeneratorURL,
		},
		Data: alert.Labels,
	}
	return event
}
