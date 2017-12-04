package monitor

import (
	"time"

	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/monitor/aws"
	"github.com/banzaicloud/hollowtrees/monitor/types"
	"github.com/sirupsen/logrus"
)

var log *logrus.Entry

func init() {
	log = conf.Logger().WithField("package", "monitor")
}

type VmPoolRequest struct {
	VmPoolTask *types.VmPoolTask
}

type VmPoolManager interface {
	MonitorVmPools() []*types.VmPoolTask
	ReevaluateVmPools() []*types.VmPoolTask
	UpdateVmPool(vmPoolTask *types.VmPoolTask)
}

func Start(region string) {
	// TODO: 100/100/10/3 should come from configuration
	vmPoolManager, err := aws.New(region)
	if err != nil {
		log.Fatal("Couldn't initialize VM Pool manager: ", err)
	}
	poolRequestChan := make(chan VmPoolRequest, 100)
	poolResponseChan := make(chan VmPoolRequest, 100)
	NewDispatcher(10, poolRequestChan, poolResponseChan, vmPoolManager).Start()
	NewCollector(3*time.Second, 60*time.Second, poolRequestChan, poolResponseChan, vmPoolManager).Start()
}
