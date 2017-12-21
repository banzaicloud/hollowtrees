package engine

import (
	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/sirupsen/logrus"
)

type Collector struct {
	Requests chan types.AlertRequest
}

func NewCollector(requests chan types.AlertRequest) *Collector {
	return &Collector{
		Requests: requests,
	}
}

func (c *Collector) Collect(alert *types.AlertInfo) {
	// TODO: log fields
	log.WithFields(logrus.Fields{
		"alertGroupKey": alert.GroupKey,
	}).Info("Pushing VM pool task to processor queue")
	c.Requests <- types.AlertRequest{AlertInfo: alert}
}
