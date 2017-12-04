package aws

import (
	"time"

	"github.com/banzaicloud/hollowtrees/monitor/types"
	"github.com/sirupsen/logrus"
)

func upscaleASG(asgm *AutoScalingGroupManager, vmPoolTask *types.VmPoolTask) error {
	// if the launch config is properly set, we don't have too many things to do here
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("ASG is upscaling: ", *vmPoolTask.VmPoolName)
	for i := 0; i < 10; i++ {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info(i, "... updating ASG ", *vmPoolTask.VmPoolName)
		time.Sleep(1 * time.Second)
	}
	return nil
}
