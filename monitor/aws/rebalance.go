package aws

import (
	"strings"

	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/banzaicloud/hollowtrees/recommender"
)

func rebalanceASG(asgm *AutoScalingGroupManager, vmPoolName *string) {
	log.Info("ASG will be rebalanced: ", vmPoolName)
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

	var instanceIds []*string
	if len(group.Instances) > 0 {
		for _, instance := range group.Instances {
			instanceIds = append(instanceIds, instance.InstanceId)
		}
	}

	state, err := getCurrentInstanceTypeState(ec2Svc, instanceIds)
	if err != nil {
		//TODO error handling
		log.Info(err.Error())
		return
	}

	subnetIds := strings.Split(*group.VPCZoneIdentifier, ",")
	subnets, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		SubnetIds: aws.StringSlice(subnetIds),
	})
	if err != nil {
		log.Error("couldn't describe subnets" + err.Error())
		//TODO: error handling
	}

	subnetsPerAz := make(map[string][]string)
	for _, subnet := range subnets.Subnets {
		subnetsPerAz[*subnet.AvailabilityZone] = append(subnetsPerAz[*subnet.AvailabilityZone], *subnet.SubnetId)
	}

	azList := make([]string, 0, len(subnetsPerAz))
	for k := range subnetsPerAz {
		azList = append(azList, k)
	}

	baseInstanceType, err := findBaseInstanceType(asgSvc, *group.AutoScalingGroupName, *group.LaunchConfigurationName)
	if err != nil {
		log.Info("couldn't find base instance type")
		//TODO error handling
	}
	recommendations, err := recommender.RecommendSpotInstanceTypes(*asgm.session.Config.Region, azList, baseInstanceType)
	if err != nil {
		log.Info("couldn't get recommendations")
		//TODO error handling
	}

	for stateInfo, instanceIdsOfType := range state {
		recommendationContains := false
		for _, recommendation := range recommendations[stateInfo.az] {
			if stateInfo.spotBidPrice != "" && recommendation.InstanceTypeName == stateInfo.instType {
				recommendationContains = true
				break
			}
		}
		if !recommendationContains {
			log.Info("this instance type will be changed to a different one because it is not among the recommended options:", stateInfo)

			launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
				LaunchConfigurationNames: []*string{group.LaunchConfigurationName},
			})
			if err != nil {
				log.Error("something happened during describing launch configs" + err.Error())
				//TODO: error handling
			}

			// TODO: we should check the current diversification of the ASG and set the selected instance types accordingly
			// TODO: this way it's possible that if there's 100 instances in AZ-1a, 25 of them is of type A and no longer recommended
			// TODO: selectInstanceTypesByCost will select 4 different types because it's 25

			selectedInstanceTypes := make(map[string][]recommender.InstanceTypeInfo)
			selectedInstanceTypes[stateInfo.az] = selectInstanceTypesByCost(recommendations[stateInfo.az], int64(len(instanceIdsOfType)))
			countsPerAz := make(map[string]int64)
			countsPerAz[stateInfo.az] = int64(len(instanceIdsOfType))

			// start new, detach, wait, attach
			instanceIdsToAttach, err := requestAndWaitSpotInstances(ec2Svc, countsPerAz, subnetsPerAz, selectedInstanceTypes, *launchConfigs.LaunchConfigurations[0])

			// change ASG min size so we can detach instances
			_, err = asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
				AutoScalingGroupName: group.AutoScalingGroupName,
				MinSize:              aws.Int64(int64(len(instanceIds) - len(instanceIdsOfType))),
			})
			if err != nil {
				log.Info("failed to update ASG: ", err.Error())
			}

			// TODO: we shouldn't detach all instances at once, we can stick to the minsize of the group and only detach
			// TODO: as many instances as we can until the minsize, then start it again
			_, err = asgSvc.DetachInstances(&autoscaling.DetachInstancesInput{
				AutoScalingGroupName:           group.AutoScalingGroupName,
				ShouldDecrementDesiredCapacity: aws.Bool(true),
				InstanceIds:                    instanceIdsOfType,
			})
			if err != nil {
				log.Info("failed to detach instances: ", err.Error())
			}

			_, err = ec2Svc.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: instanceIdsOfType,
			})
			if err != nil {
				log.Info("failed to terminate instances: ", err.Error())
			}

			_, err = asgSvc.AttachInstances(&autoscaling.AttachInstancesInput{
				InstanceIds:          instanceIdsToAttach,
				AutoScalingGroupName: group.AutoScalingGroupName,
			})
			if err != nil {
				log.Info("failed to attach instances: ", err.Error())
			}

			// change back ASG min size to original
			_, err = asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
				AutoScalingGroupName: group.AutoScalingGroupName,
				MinSize:              group.MinSize,
			})
			if err != nil {
				log.Info("couldn't update min size", err.Error())
			}

			// wait until there are no pending instances in ASG
			nrOfPending := len(instanceIdsOfType)
			for nrOfPending != 0 {
				nrOfPending = 0
				r, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
					AutoScalingGroupNames: []*string{group.AutoScalingGroupName},
				})
				if err != nil {
					log.Info("couldn't describe ASG while checking it at the end")
					//TODO: error handling
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
	}
}
