package aws

import "time"

func rebalanceASG(asgm *AutoScalingGroupManager, vmPoolName *string) {
	log.Info("ASG will be rebalanced: ", vmPoolName)
	for i := 0; i < 32; i++ {
		log.Info(i, "... updating ASG ", *vmPoolName)
		time.Sleep(1 * time.Second)
	}
}

