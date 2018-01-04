package engine

import (
	"github.com/banzaicloud/hollowtrees/engine/types"
)

type Collector struct {
	Requests chan types.AlertRequest
}

func NewCollector(requests chan types.AlertRequest) *Collector {
	return &Collector{
		Requests: requests,
	}
}

func (c *Collector) Collect(alerts []types.Alert) {
	// TODO: log fields
	log.Info("Pushing alerts to queue")
	c.Requests <- types.AlertRequest{Alerts: alerts}
}
