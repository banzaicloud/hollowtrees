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

	recommendations, err := recommender.RecommendSpotInstanceTypes(*asgm.session.Config.Region, azList, *launchConfig.InstanceType)
	if err != nil {
		log.Error("couldn't recommend spot instance types" + err.Error())
		//TODO: error handling
	}
	log.Info("recommendations in selected AZs are", recommendations)

	i := 0
	selectedInstanceTypes := make(map[string][]recommender.InstanceTypeInfo)
	countsPerAz := make(map[string]int64)
	for az, recommendedTypes := range recommendations {
		countInAZ := *originalDesiredCap / int64(len(azList))
		remainderCount := *originalDesiredCap % int64(len(azList))
		if i == 0 {
			countInAZ = countInAZ + remainderCount
		}
		countsPerAz[az] = countInAZ
		selectedInstanceTypes[az] = selectInstanceTypesByCost(recommendedTypes, countInAZ)
		i++
	}

	instanceIds, err := requestAndWaitSpotInstances(ec2Svc, countsPerAz, subnetsPerAz, selectedInstanceTypes, launchConfig)
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

func selectInstanceTypesByCost(recommendations []recommender.InstanceTypeInfo, nrOfInstances int64) []recommender.InstanceTypeInfo {
	sort.Sort(sort.Reverse(ByCostScore(recommendations)))
	if nrOfInstances < 2 || len(recommendations) < 2 {
		return recommendations[:1]
	} else if nrOfInstances < 9 || len(recommendations) < 3 {
		return recommendations[:2]
	} else if nrOfInstances < 20 || len(recommendations) < 4 {
		return recommendations[:3]
	} else {
		return recommendations[:4]
	}
}

func requestAndWaitSpotInstances(ec2Svc *ec2.EC2, countsPerAZ map[string]int64, subnetsPerAZ map[string][]string, selectedInstanceTypes map[string][]recommender.InstanceTypeInfo, launchConfig autoscaling.LaunchConfiguration) ([]*string, error) {
	var instanceIds []*string
	var spotRequestIds []*string

	log.Info("counts per az: ", countsPerAZ)
	log.Info("subnets per az: ", subnetsPerAZ)
	log.Info("selected instance types: ", selectedInstanceTypes)

	for az, selectedInstanceTypesInAZ := range selectedInstanceTypes {
		log.Info("az process started: ", az)
		log.Info("selected instance types here are: ", selectedInstanceTypesInAZ)
		totalCountInAZ := countsPerAZ[az]
		countPerType := totalCountInAZ / int64(len(selectedInstanceTypesInAZ))
		remainderCount := totalCountInAZ % int64(len(selectedInstanceTypesInAZ))

		log.Info("total count in az: ", totalCountInAZ)
		log.Info("count per type: ", countPerType)
		log.Info("remainder: ", remainderCount)

		for i, instanceType := range selectedInstanceTypesInAZ {
			log.Info("selected instance type: ", i, instanceType)
			countForType := countPerType
			if i == 0 {
				countForType = countForType + remainderCount
			}

			log.Info("count for type: ", countForType)

			subnetsInAZ := subnetsPerAZ[az]

			log.Info("subnets in AZ: ", subnetsInAZ)

			itCountPerSubnet := countForType / int64(len(subnetsInAZ))
			remainderItCount := countForType % int64(len(subnetsInAZ))

			log.Info("instance type count per subnet: ", itCountPerSubnet)
			log.Info("remainder: ", remainderItCount)

			for i, subnet := range subnetsInAZ {
				log.Info("processing subnet: ", subnet)

				itCountForSubnet := itCountPerSubnet
				if i == 0 {
					itCountForSubnet = itCountForSubnet + remainderItCount
				}

				log.Info("it count for subnet: ", itCountForSubnet)

				if itCountForSubnet != 0 {
					requestSpotResult, err := ec2Svc.RequestSpotInstances(&ec2.RequestSpotInstancesInput{
						InstanceCount: &itCountForSubnet,
						SpotPrice:     &instanceType.OnDemandPrice,
						LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
							InstanceType: &instanceType.InstanceTypeName,
							ImageId:      launchConfig.ImageId,
							NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
								{
									DeviceIndex:              aws.Int64(0),
									SubnetId:                 &subnet,
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
						//return nil, err
					}
					for _, spotReq := range requestSpotResult.SpotInstanceRequests {
						spotRequestIds = append(spotRequestIds, spotReq.SpotInstanceRequestId)
					}
				}
			}
		}
	}

	var totalCount int64

	for _, countInAz := range countsPerAZ {
		totalCount += countInAz
	}

	// collect instanceids of newly started spot instances
	for int64(len(instanceIds)) != totalCount {
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
	for totalCount != currentlyRunning {
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
