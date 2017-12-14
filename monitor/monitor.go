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
	CheckVmPools() ([]*types.VmPoolTask, error)
	ReevaluateVmPools() ([]*types.VmPoolTask, error)
	UpdateVmPool(vmPoolTask *types.VmPoolTask) error
}

func Start(region string, bufferSize int, pluginAddress string, monitorInterval time.Duration, reevaluateInterval time.Duration) {
	vmPoolManager, err := aws.New(region)
	if err != nil {
		log.Fatal("Couldn't initialize VM Pool manager: ", err)
	}
	poolRequestChan := make(chan VmPoolRequest, bufferSize)
	poolResponseChan := make(chan VmPoolRequest, bufferSize)
	NewDispatcher(pluginAddress, poolRequestChan, poolResponseChan, vmPoolManager).Start()
	NewCollector(monitorInterval, reevaluateInterval, poolRequestChan, poolResponseChan, vmPoolManager).Start()
}
