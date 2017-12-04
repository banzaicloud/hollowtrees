package aws

import (
	"time"

	"github.com/banzaicloud/hollowtrees/monitor/types"
	"github.com/sirupsen/logrus"
)

func downscaleASG(asgm *AutoScalingGroupManager, vmPoolTask *types.VmPoolTask) error {
	// we can check if the most expensive vm will be detached or not
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("ASG is downscaling: ", *vmPoolTask.VmPoolName)
	for i := 0; i < 32; i++ {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info(i, "... updating ASG ", *vmPoolTask.VmPoolName)
		time.Sleep(1 * time.Second)
	}
	return nil
}
