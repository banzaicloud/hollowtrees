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
		log.WithField("eventId", event.EventId).Infof("Pushing event to queue: %#v", event)
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
