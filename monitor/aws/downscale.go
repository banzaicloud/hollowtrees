package aws

import "time"

func downscaleASG(asgm *AutoScalingGroupManager, vmPoolName *string) {
	// we can check if the most expensive vm will be detached or not
	log.Info("ASG is downscaling: ", vmPoolName)
	for i := 0; i < 32; i++ {
		log.Info(i, "... updating ASG ", *vmPoolName)
		time.Sleep(1 * time.Second)
	}
}
