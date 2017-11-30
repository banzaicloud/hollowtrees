package aws

import (
	"time"

	"strconv"
	"strings"

	"sort"

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
	selectedInstanceTypes := selectInstanceTypes(instanceTypes, *originalDesiredCap)
	// request spot instances instead
	instanceIds, err := requestAndWaitSpotInstances(ec2Svc, originalDesiredCap, selectedInstanceTypes, launchConfig, group)
	if err != nil {
		//TODO: error handling
		log.Error("couldn't request spot instances" + err.Error())
	}
	log.Info("all instances are running")

	// attach new instances to ASG
	_, err = asgSvc.AttachInstances(&autoscaling.AttachInstancesInput{
		InstanceIds:          instanceIds,
		AutoScalingGroupName: group.AutoScalingGroupName,
	})
	if err != nil {
		log.Info("failed to attach instances: ", err.Error())
	}

	// change back ASG min size to original
	_, err = asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: group.AutoScalingGroupName,
		MinSize:              originalMinSize,
	})
	if err != nil {
		log.Info("couldn't update min size", err.Error())
	}
	//}
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

type ByCostScore []recommender.InstanceTypeInfo

func (a ByCostScore) Len() int      { return len(a) }
func (a ByCostScore) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByCostScore) Less(i, j int) bool {
	costScore1, _ := strconv.ParseFloat(strings.Split(a[i].CostScore, " ")[0], 32)
	costScore2, _ := strconv.ParseFloat(strings.Split(a[j].CostScore, " ")[0], 32)
	return costScore1 < costScore2
}

func selectInstanceTypes(instanceTypes []recommender.InstanceTypeInfo, nrOfInstances int64) []recommender.InstanceTypeInfo {
	sort.Sort(sort.Reverse(ByCostScore(instanceTypes)))
	if nrOfInstances < 2 || len(instanceTypes) < 2 {
		return instanceTypes[:1]
	} else if nrOfInstances < 9 || len(instanceTypes) < 3 {
		return instanceTypes[:2]
	} else if nrOfInstances < 20 || len(instanceTypes) < 4 {
		return instanceTypes[:3]
	} else {
		return instanceTypes[:4]
	}
}

func requestAndWaitSpotInstances(ec2Svc *ec2.EC2, count *int64, selectedInstanceTypes []recommender.InstanceTypeInfo, launchConfig autoscaling.LaunchConfiguration, group *autoscaling.Group) ([]*string, error) {
	var instanceIds []*string
	var spotRequestIds []*string
	countPerType := *count / int64(len(selectedInstanceTypes))
	remainderCount := *count % int64(len(selectedInstanceTypes))
	for i, instanceType := range selectedInstanceTypes {
		countForType := countPerType
		if i == 0 {
			countForType = countForType + remainderCount
		}
		requestSpotResult, err := ec2Svc.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
			InstanceCount: &countForType,
			SpotPrice:     &instanceType.OnDemandPrice,
			LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
				InstanceType: &instanceType.InstanceTypeName,
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
			log.Info("couldn't request spot instances", err.Error())
			return nil, err
		}
		for _, spotReq := range requestSpotResult.SpotInstanceRequests {
			spotRequestIds = append(spotRequestIds, spotReq.SpotInstanceRequestId)
		}
	}

	// collect instanceids of newly started spot instances
	for int64(len(instanceIds)) != *count {
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
	for *count != currentlyRunning {
		currentlyRunning = 0
		log.Info("describing instances:")
		describeInstResult, err := ec2Svc.DescribeInstanceStatus(&ec2.DescribeInstanceStatusInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			log.Info("failed to describe instances ", err.Error())
		}
		log.Info("nr of instances in describe result ", len(describeInstResult.InstanceStatuses))
		for _, instanceStatus := range describeInstResult.InstanceStatuses {
			log.Info(*instanceStatus.InstanceState.Name)
			if *instanceStatus.InstanceState.Name == "running" {
				currentlyRunning += 1
			}
		}
		log.Info("currently running ", currentlyRunning)
		time.Sleep(1 * time.Second)
	}
	return instanceIds, nil
}
