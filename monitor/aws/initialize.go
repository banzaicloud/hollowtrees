package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/banzaicloud/hollowtrees/recommender"
)

func initializeASG(asgm *AutoScalingGroupManager, vmPoolName *string) {
	ec2Svc := ec2.New(asgm.session, aws.NewConfig())
	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())
	describeResult, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{vmPoolName},
	})
	if err != nil {
		log.Error("something happened while polling ASGs" + err.Error())
		//TODO: error handling
	}
	group := describeResult.AutoScalingGroups[0]
	originalDesiredCap := group.DesiredCapacity
	originalMinSize := group.MinSize
	log.Info("updating to 0 desired")
	// setting desired capacity to 0
	result, err := asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: group.AutoScalingGroupName,
		DesiredCapacity:      aws.Int64(0),
		MinSize:              aws.Int64(0),
	})
	if err != nil {
		log.Info("error happened during updating to 0 desired", err.Error())
		//TODO: error handling
	}
	log.Info(result)
	log.Info("updated to 0 desired")
	// get launchconfig instancetype
	launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{group.LaunchConfigurationName},
	})
	if err != nil {
		log.Error("something happened during describing launch configs" + err.Error())
		//TODO: error handling
	}
	launchConfig := *launchConfigs.LaunchConfigurations[0]
	log.Info("launch config instance type is", *launchConfig.InstanceType)
	recommendations, err := recommender.RecommendSpotInstanceTypes("eu-west-1", "1a", *launchConfig.InstanceType)
	if err != nil {
		log.Error("couldn't recommend spot instance types" + err.Error())
		//TODO: error handling
	}
	instanceTypes := recommendations["eu-west-1a"]
	log.Info("recommendations in eu-west-1a are", instanceTypes)
	// TODO: use other instance types as well
	instType := instanceTypes[0]
	log.Info("instance type: ", instType.InstanceTypeName)
	// request spot instances instead
	requestSpotResult, err := ec2Svc.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
		InstanceCount: originalDesiredCap,
		SpotPrice:     &instType.OnDemandPrice,
		LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
			InstanceType: &instType.InstanceTypeName,
			ImageId:      launchConfig.ImageId,
			NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
				{
					DeviceIndex:              aws.Int64(0),
					SubnetId:                 group.VPCZoneIdentifier,
					AssociatePublicIpAddress: launchConfig.AssociatePublicIpAddress,
					Groups: launchConfig.SecurityGroups,
				},
			},
			EbsOptimized: launchConfig.EbsOptimized,
			IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
				Name: launchConfig.IamInstanceProfile,
			},
			KeyName:  launchConfig.KeyName,
			UserData: launchConfig.UserData,
		},
	})
	if err != nil {
		log.Info("couldn't start instances", err.Error())
	}
	var spotRequestIds []*string
	for _, spotReq := range requestSpotResult.SpotInstanceRequests {
		spotRequestIds = append(spotRequestIds, spotReq.SpotInstanceRequestId)
	}
	// collect instanceids of newly started spot instances
	var instanceIds []*string
	for int64(len(instanceIds)) != *originalDesiredCap {
		instanceIds = []*string{}
		spotRequests, err := ec2Svc.DescribeSpotInstanceRequests(&ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: spotRequestIds,
		})
		if err != nil {
			log.Info("failed to describe spot requests")
		}
		for _, spotReq := range spotRequests.SpotInstanceRequests {
			if spotReq.InstanceId != nil {
				log.Info("instanceId of spot request is:", *spotReq.InstanceId)
				if *spotReq.InstanceId != "" {
					instanceIds = append(instanceIds, spotReq.InstanceId)
				}
			} else {
				log.Info("instance id in request is null")
			}
		}
		time.Sleep(1 * time.Second)
	}
	// wait until new instances are running
	var currentlyRunning int64 = 0
	for *originalDesiredCap != currentlyRunning {
		currentlyRunning = 0
		log.Info("describing instances:")
		desribeInstResult, err := ec2Svc.DescribeInstanceStatus(&ec2.DescribeInstanceStatusInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			log.Info("failed to describe instances ", err.Error())
		}
		log.Info("nr of instances in describe result ", len(desribeInstResult.InstanceStatuses))
		for _, instanceStatus := range desribeInstResult.InstanceStatuses {
			log.Info(*instanceStatus.InstanceState.Name)
			if *instanceStatus.InstanceState.Name == "running" {
				currentlyRunning += 1
			}
		}
		log.Info("currently running ", currentlyRunning)
		time.Sleep(1 * time.Second)
	}
	if currentlyRunning == *originalDesiredCap {
		log.Info("all instances are running")

		// attach new instances to ASG
		_, err := asgSvc.AttachInstances(&autoscaling.AttachInstancesInput{
			InstanceIds:          instanceIds,
			AutoScalingGroupName: group.AutoScalingGroupName,
		})
		if err != nil {
			log.Info("failed to attach instances: ", err.Error())
		}
		log.Info("")

		// change back ASG min size to original
		_, err2 := asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
			AutoScalingGroupName: group.AutoScalingGroupName,
			MinSize:              originalMinSize,
		})
		if err2 != nil {
			log.Info("couldn't update min size", err2.Error())
		}
	}
	// wait until there are no pending instances in ASG
	nrOfPending := *originalDesiredCap
	for nrOfPending != 0 {
		nrOfPending = 0
		r, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: []*string{group.AutoScalingGroupName},
		})
		if err != nil {
			log.Info("couldn't describe ASG while checking it at the end")
		}
		for _, instance := range r.AutoScalingGroups[0].Instances {
			if *instance.LifecycleState == "Pending" {
				log.Info("found a pending instance: ", *instance.InstanceId)
				nrOfPending++
			}
		}
		time.Sleep(1 * time.Second)
	}
}
