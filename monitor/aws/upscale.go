package aws

import "time"

func upscaleASG(asgm *AutoScalingGroupManager, vmPoolName *string) {
	// if the launch config is properly set, we don't have too many things to do here
	log.Info("ASG is upscaling: ", vmPoolName)
	for i := 0; i < 10; i++ {
		log.Info(i, "... updating ASG ", *vmPoolName)
		time.Sleep(1 * time.Second)
	}
}
