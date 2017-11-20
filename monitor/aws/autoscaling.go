package aws

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/banzaicloud/hollowtrees/recommender"
	"github.com/banzaicloud/hollowtrees/monitor/types"
	"time"
)

var log *logrus.Logger

type AutoScalingGroupManager struct {
	session *session.Session
}

type InstanceType struct {
	instType     string
	spotBidPrice string
}

type ASGInstanceTypes struct {
	Name          string
	InstanceTypes map[InstanceType]int
}

func New(region string) (*AutoScalingGroupManager, error) {
	log = conf.Logger()
	session, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		log.Info("Error creating session ", err)
		return nil, err
	}
	return &AutoScalingGroupManager{
		session: session,
	}, nil
}

func (asgm *AutoScalingGroupManager) CollectVmPools() []*types.VmPoolTask {
	var vmPoolTasks []*types.VmPoolTask
	log = conf.Logger()
	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())
	ec2Svc := ec2.New(asgm.session, aws.NewConfig())
	result, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		log.Error("something happened while polling ASGs" + err.Error())
	}
	log.Info("number of ASGs found:", len(result.AutoScalingGroups))
	for _, asg := range result.AutoScalingGroups {
		htManaged := false
		for _, tag := range asg.Tags {
			if *tag.Key == "Hollowtrees" && *tag.Value == "true" {
				htManaged = true;
				break
			}
		}
		if htManaged {
			nrOfPending := 0
			nrOfTerminating := 0
			instanceIds := []*string{}

			if len(asg.Instances) > 0 {
				for _, instance := range asg.Instances {
					instanceIds = append(instanceIds, instance.InstanceId)
					if *instance.LifecycleState == "Pending" {
						nrOfPending++
					} else if *instance.LifecycleState == "Terminating" {
						nrOfTerminating++
					}
				}

				log.WithFields(logrus.Fields{
					"asg": *asg.AutoScalingGroupName,
				}).Info("desired/current/pending:", *asg.DesiredCapacity, len(asg.Instances), nrOfPending)
			}

			// ASG is initializing if the desired cap is not zero but the nr of instances is 0 or all of them are pending
			if *asg.DesiredCapacity != 0 && (len(asg.Instances) == 0 || nrOfPending == len(asg.Instances)) {
				log.Info("ASG is initializing")
				vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
					VmPoolName:   asg.AutoScalingGroupName,
					VmPoolAction: aws.String("initializing"),
				})
				break
			}

			// ASG is upscaling if desired cap is not zero and nr of running+pending = desired cap
			if *asg.DesiredCapacity != 0 && nrOfPending != 0 && nrOfTerminating == 0 {
				log.Info("ASG is upscaling")
				vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
					VmPoolName:   asg.AutoScalingGroupName,
					VmPoolAction: aws.String("upscaling"),
				})
				break
			}

			// ASG is downscaling (or instances are terminated) if some instances are terminating
			if nrOfTerminating != 0 && nrOfPending == 0 {
				log.Info("ASG is downscaling")
				vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
					VmPoolName:   asg.AutoScalingGroupName,
					VmPoolAction: aws.String("downscaling"),
				})
				break
			}

			// ASG is running okay, but recommendation shows something else
			if nrOfPending == 0 && nrOfTerminating == 0 {
				log.Info("there are no operations in progress, checking current state")
				// describing current state
				// TODO: cache this state??
				instances, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
					InstanceIds: instanceIds,
				})
				if err != nil {
					// TODO
					log.Info("failed to describe instances: ", err.Error())
				}

				var state ASGInstanceTypes = ASGInstanceTypes{
					Name:          *asg.AutoScalingGroupName,
					InstanceTypes: make(map[InstanceType]int),
				}
				var spotRequests []*string
				for _, reservation := range instances.Reservations {
					for _, instance := range reservation.Instances {
						if instance.SpotInstanceRequestId != nil {
							spotRequests = append(spotRequests, instance.SpotInstanceRequestId)
						} else {
							it := InstanceType{
								instType:     *instance.InstanceType,
								spotBidPrice: "",
							}
							state.InstanceTypes[it] += 1
						}
					}
				}

				if len(spotRequests) > 0 {
					output, err := ec2Svc.DescribeSpotInstanceRequests(&ec2.DescribeSpotInstanceRequestsInput{
						SpotInstanceRequestIds: spotRequests,
					})
					if err != nil {
						//TODO error handling
						log.Info("failed to describe spot requests ")
					}

					for _, spotRequest := range output.SpotInstanceRequests {
						it := InstanceType{
							instType:     *spotRequest.LaunchSpecification.InstanceType,
							spotBidPrice: *spotRequest.SpotPrice,
						}
						state.InstanceTypes[it] += 1
					}
				}

				log.Info("current state of instanceTypes in ASG: ", state)

				recommendations, err := recommender.RecommendSpotInstanceTypes("eu-west-1", "eu-west-1a", "m4.xlarge")
				if err != nil {
					log.Info("couldn't get recommendations")
					//TODO error handling
				}

				// If there is at least one spot instance that's not recommended then create an action
				for stateInfo := range state.InstanceTypes {
					recommendationContains := false
					for _, recommendation := range recommendations["eu-west-1a"] {
						if stateInfo.spotBidPrice != "" && recommendation.InstanceTypeName == stateInfo.instType {
							recommendationContains = true
							break
						}
					}
					if !recommendationContains {
						log.Info("instanceType ", stateInfo, " is not among recommendations, sending rebalancing request")
						vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
							VmPoolName:   asg.AutoScalingGroupName,
							VmPoolAction: aws.String("rebalancing"),
						})
						break
					}
				}
			}
		}
	}
	return vmPoolTasks
}

func (asgm *AutoScalingGroupManager) UpdateVmPool(vmPoolTask *types.VmPoolTask) {
	log.Info("updating ASG: ", *vmPoolTask.VmPoolName, " ", *vmPoolTask.VmPoolAction)
	for i := 0; i < 32; i++ {
		log.Info(i, "... updating ASG ", *vmPoolTask.VmPoolName)
		time.Sleep(1 * time.Second)
	}
	//ec2Svc := ec2.New(asgm.session, aws.NewConfig())
	//asgSvc := autoscaling.New(asgm.session, aws.NewConfig())
	//
	//describeResult, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
	//	AutoScalingGroupNames: []*string{vmPoolTask.VmPoolName},
	//})
	//if err != nil {
	//	log.Error("something happened while polling ASGs" + err.Error())
	//}
	//group := describeResult.AutoScalingGroups[0]
	//originalDesiredCap := group.DesiredCapacity
	//originalMinSize := group.MinSize
	//
	//log.Info("updating to 0 desired")
	//
	//// setting desired capacity to 0
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
	//
	//// get launchconfig instancetype
	//launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
	//	LaunchConfigurationNames: []*string{group.LaunchConfigurationName},
	//})
	//if err != nil {
	//	log.Error("something happened during describing launch configs" + err.Error())
	//}
	//launchConfig := *launchConfigs.LaunchConfigurations[0]
	//log.Info("launch config instance type is", *launchConfig.InstanceType)
	//recommendations, err := recommender.RecommendSpotInstanceTypes("eu-west-1", "1a", *launchConfig.InstanceType)
	//if err != nil {
	//	log.Error("couldn't recommend spot instance types" + err.Error())
	//}
	//instanceTypes := recommendations["eu-west-1a"]
	//log.Info("recommendations in eu-west-1a are", instanceTypes)
	//instType := instanceTypes[0]
	//log.Info("instance type: ", instType.InstanceTypeName)
	//
	//// request spot instances instead
	//requestSpotResult, err := ec2Svc.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
	//	InstanceCount: originalDesiredCap,
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
	//// collect instanceids of newly started spot instances
	//var instanceIds []*string
	//for int64(len(instanceIds)) != *originalDesiredCap {
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
	//// wait until new instances are running
	//var currentlyRunning int64 = 0
	//for *originalDesiredCap != currentlyRunning {
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
	//if currentlyRunning == *originalDesiredCap {
	//	log.Info("all instances are running")
	//
	//	// attach new instances to ASG
	//	_, err := asgSvc.AttachInstances(&autoscaling.AttachInstancesInput{
	//		InstanceIds:          instanceIds,
	//		AutoScalingGroupName: group.AutoScalingGroupName,
	//	})
	//	if err != nil {
	//		log.Info("failed to attach instances: ", err.Error())
	//	}
	//	log.Info("")
	//
	//	// change back ASG min size to original
	//	_, err2 := asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
	//		AutoScalingGroupName: group.AutoScalingGroupName,
	//		MinSize:              originalMinSize,
	//	})
	//	if err2 != nil {
	//		log.Info("couldn't update min size", err2.Error())
	//	}
	//}
	//
	//// wait until there are no pending instances in ASG
	//nrOfPending := *originalDesiredCap
	//for nrOfPending != 0 {
	//	nrOfPending = 0
	//	r, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
	//		AutoScalingGroupNames: []*string{group.AutoScalingGroupName},
	//	})
	//	if err != nil {
	//		log.Info("couldn't describe ASG while checking it at the end")
	//	}
	//	for _, instance := range r.AutoScalingGroups[0].Instances {
	//		if *instance.LifecycleState == "Pending" {
	//			log.Info("found a pending instance: ", *instance.InstanceId)
	//			nrOfPending++
	//		}
	//	}
	//	time.Sleep(1 * time.Second)
	//}
}
