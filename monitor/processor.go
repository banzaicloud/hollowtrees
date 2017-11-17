package monitor

import (
	"github.com/sirupsen/logrus"
	"github.com/banzaicloud/hollowtrees/conf"
	"time"
)

type PoolProcessor struct {
	ID          int
	Work        chan VmPoolRequest
	WorkerQueue chan chan VmPoolRequest
	Results     chan VmPoolRequest
	QuitChan    chan bool
}

func NewPoolProcessor(id int, workerQueue chan chan VmPoolRequest, results chan VmPoolRequest) PoolProcessor {
	return PoolProcessor{
		ID:          id,
		Work:        make(chan VmPoolRequest),
		WorkerQueue: workerQueue,
		Results:     results,
		QuitChan:    make(chan bool)}
}

func (w *PoolProcessor) Start() {
	//ec2Svc := ec2.New(session, aws.NewConfig().WithRegion("eu-west-1"))
	log = conf.Logger()
	go func() {
		for {
			// Add ourselves into the worker queue.
			w.WorkerQueue <- w.Work

			select {
			case work := <-w.Work:
				// Receive a work request.
				log.WithFields(logrus.Fields{
					"worker": w.ID,
					"asg":    *work.VmPoolName,
				}).Info("Received work request")

				w.UpdateASG(work.VmPoolName)
				log.WithFields(logrus.Fields{
					"worker": w.ID,
					"asg":    *work.VmPoolName,
				}).Info("Updated ASG done")
				w.Results <- work

			case <-w.QuitChan:
				log.WithFields(logrus.Fields{
					"worker": w.ID,
				}).Info("PoolProcessor stopping")
				return
			}
		}
	}()
}

func (w *PoolProcessor) Stop() {
	go func() {
		w.QuitChan <- true
	}()
}

func (w *PoolProcessor) UpdateASG(vmPoolName *string) {
	for i := 0; i < 32; i++ {
		time.Sleep(1 * time.Second)
		log.Info("sleeping .. ", *vmPoolName, i)
	}
	//result, err := asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
	//	AutoScalingGroupName: group.AutoScalingGroupName,
	//	DesiredCapacity:      aws.Int64(0),
	//	MinSize:              aws.Int64(0),
	//})
	//if err != nil {
	//	log.Info("error happened during updating to 0 desired", err.Error())
	//}
	//log.Info(result)
	//log.Info("updated to 0 desired")
	//launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
	//	LaunchConfigurationNames: []*string{group.LaunchConfigurationName},
	//})
	//if err != nil {
	//	//...
	//}
	//launchConfig := *launchConfigs.LaunchConfigurations[0]
	//log.Info("launch config instance type is", *launchConfig.InstanceType)
	//recommendations, err := recommender.RecommendSpotInstanceTypes("eu-west-1", "1a", *launchConfig.InstanceType)
	//if err != nil {
	//	//...
	//}
	//instanceTypes := recommendations["eu-west-1a"]
	//log.Info("recommendations in eu-west-1a are", instanceTypes)
	//instType := instanceTypes[0]
	//log.Info("instance type: ", instType.InstanceTypeName)
	//requestSpotResult, err := ec2Svc.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
	//	InstanceCount: &originalDesiredCap,
	//	SpotPrice:     &instType.OnDemandPrice,
	//	LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
	//		InstanceType: &instType.InstanceTypeName,
	//		ImageId:      launchConfig.ImageId,
	//		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
	//			{
	//				DeviceIndex:              aws.Int64(0),
	//				SubnetId:                 group.VPCZoneIdentifier,
	//				AssociatePublicIpAddress: launchConfig.AssociatePublicIpAddress,
	//				Groups:                   launchConfig.SecurityGroups,
	//			},
	//		},
	//		EbsOptimized: launchConfig.EbsOptimized,
	//		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
	//			Name: launchConfig.IamInstanceProfile,
	//		},
	//		KeyName:  launchConfig.KeyName,
	//		UserData: launchConfig.UserData,
	//	},
	//})
	//if err != nil {
	//	log.Info("couldn't start instances", err.Error())
	//}
	//var spotRequestIds []*string
	//for _, spotReq := range requestSpotResult.SpotInstanceRequests {
	//	spotRequestIds = append(spotRequestIds, spotReq.SpotInstanceRequestId)
	//}
	//
	//var instanceIds []*string
	//for int64(len(instanceIds)) != originalDesiredCap {
	//	instanceIds = []*string{}
	//	spotRequests, err := ec2Svc.DescribeSpotInstanceRequests(&ec2.DescribeSpotInstanceRequestsInput{
	//		SpotInstanceRequestIds: spotRequestIds,
	//	})
	//	if err != nil {
	//		log.Info("failed to describe spot requests")
	//	}
	//	for _, spotReq := range spotRequests.SpotInstanceRequests {
	//		if spotReq.InstanceId != nil {
	//			log.Info("instanceId of spot request is:", *spotReq.InstanceId)
	//			if *spotReq.InstanceId != "" {
	//				instanceIds = append(instanceIds, spotReq.InstanceId)
	//			}
	//		} else {
	//			log.Info("instance id in request is null")
	//		}
	//	}
	//	time.Sleep(1 * time.Second)
	//}
	//
	//var currentlyRunning int64 = 0
	//for originalDesiredCap != currentlyRunning {
	//	currentlyRunning = 0
	//	log.Info("describing instances:")
	//	desribeInstResult, err := ec2Svc.DescribeInstanceStatus(&ec2.DescribeInstanceStatusInput{
	//		InstanceIds: instanceIds,
	//	})
	//	if err != nil {
	//		log.Info("failed to describe instances ", err.Error())
	//	}
	//	log.Info("nr of instances in describe result ", len(desribeInstResult.InstanceStatuses))
	//	for _, instanceStatus := range desribeInstResult.InstanceStatuses {
	//		log.Info(*instanceStatus.InstanceState.Name)
	//		if *instanceStatus.InstanceState.Name == "running" {
	//			currentlyRunning += 1
	//		}
	//	}
	//	log.Info("currently running ", currentlyRunning)
	//	time.Sleep(1 * time.Second)
	//}
	//if currentlyRunning == originalDesiredCap {
	//	log.Info("all instances are running")
	//	_, err := asgSvc.AttachInstances(&autoscaling.AttachInstancesInput{
	//		InstanceIds:          instanceIds,
	//		AutoScalingGroupName: group.AutoScalingGroupName,
	//	})
	//	if err != nil {
	//		log.Info("failed to attach instances: ", err.Error())
	//	}
	//	log.Info("")
	//	_, err2 := asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
	//		AutoScalingGroupName: group.AutoScalingGroupName,
	//		MinSize:              &originalMinSize,
	//	})
	//	if err2 != nil {
	//		log.Info("couldn't update min size", err2.Error())
	//	}
	//}
	//
	//nrOfPending2 := originalDesiredCap
	//for nrOfPending2 != 0 {
	//	nrOfPending2 = 0
	//	r, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
	//		AutoScalingGroupNames: []*string{group.AutoScalingGroupName},
	//	})
	//	if err != nil {
	//		log.Info("couldn't describe ASG while checking it at the end")
	//	}
	//	for _, instance := range r.AutoScalingGroups[0].Instances {
	//		if *instance.LifecycleState == "Pending" {
	//			log.Info("found a pending instance: ", *instance.InstanceId)
	//			nrOfPending2++
	//		}
	//	}
	//	time.Sleep(1 * time.Second)
	//}
	////processing = false
	////}(*asg)
}
