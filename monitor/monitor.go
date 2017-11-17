package monitor

import (
	"time"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

type VmPoolRequest struct {
	VmPoolName *string
}

func Start() {
	// TODO: 100/100/10/3 should come from configuration
	poolRequestChan := make(chan VmPoolRequest, 100)
	poolResponseChan := make(chan VmPoolRequest, 100)
	NewDispatcher(10, poolRequestChan, poolResponseChan).Start()
	NewCollector(3*time.Second, poolRequestChan, poolResponseChan).Start()
}
