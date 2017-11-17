package monitor

import (
	"time"
	"github.com/sirupsen/logrus"
	"github.com/banzaicloud/hollowtrees/monitor/aws"
)

var log *logrus.Logger

type VmPoolRequest struct {
	VmPoolName *string
}

type VmPoolManager interface {
	CollectVmPools() []*string
	UpdateVmPool(vmPoolName *string)
}

func Start() {
	// TODO: 100/100/10/3/eu-west-1 should come from configuration
	vmPoolManager, err := aws.New("eu-west-1")
	if err != nil {
		log.Fatal("Couldn't initialize VM Pool manager: ", err)
	}
	poolRequestChan := make(chan VmPoolRequest, 100)
	poolResponseChan := make(chan VmPoolRequest, 100)
	NewDispatcher(10, poolRequestChan, poolResponseChan, vmPoolManager).Start()
	NewCollector(3*time.Second, poolRequestChan, poolResponseChan, vmPoolManager).Start()
}
