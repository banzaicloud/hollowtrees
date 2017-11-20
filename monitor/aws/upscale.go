package aws

import "time"

func upscaleASG(asgm *AutoScalingGroupManager, vmPoolName *string) {
	log.Info("ASG is upscaling: ", vmPoolName)
	for i := 0; i < 32; i++ {
		log.Info(i, "... updating ASG ", *vmPoolName)
		time.Sleep(1 * time.Second)
	}
}
