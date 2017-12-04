package aws

import (
	"time"

	"strconv"
	"strings"

	"sort"

	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/banzaicloud/hollowtrees/monitor/types"
	"github.com/banzaicloud/hollowtrees/recommender"
	"github.com/sirupsen/logrus"
)

func initializeASG(asgm *AutoScalingGroupManager, vmPoolTask *types.VmPoolTask) error {
	ec2Svc := ec2.New(asgm.session, aws.NewConfig())
	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())
	describeResult, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{vmPoolTask.VmPoolName},
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("something happened while polling ASGs" + err.Error())
		return err
	}
	group := describeResult.AutoScalingGroups[0]
	originalDesiredCap := group.DesiredCapacity
	originalMinSize := group.MinSize
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("updating ASG's desired capacity to 0")
	// setting desired capacity to 0
	_, err = asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: group.AutoScalingGroupName,
		DesiredCapacity:      aws.Int64(0),
		MinSize:              aws.Int64(0),
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("error happened during updating to 0 desired", err.Error())
		return err
	}
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("updated ASG's desired capacity to 0")

	launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{group.LaunchConfigurationName},
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("Couldn't describe launch configs" + err.Error())
		return err
	}
	launchConfig := *launchConfigs.LaunchConfigurations[0]
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("launch config instance type is", *launchConfig.InstanceType)

	subnetIds := strings.Split(*group.VPCZoneIdentifier, ",")
	subnets, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		SubnetIds: aws.StringSlice(subnetIds),
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("couldn't describe subnets" + err.Error())
		return err
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
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("couldn't recommend spot instance types" + err.Error())
		return err
	}
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("recommendations in selected AZs are", recommendations)

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

	instanceIds, err := requestAndWaitSpotInstances(ec2Svc, vmPoolTask, countsPerAz, subnetsPerAz, selectedInstanceTypes, launchConfig)
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("couldn't request spot instances" + err.Error())
		return err
	}
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("all instances are running")

	// attach new instances to ASG
	_, err = asgSvc.AttachInstances(&autoscaling.AttachInstancesInput{
		InstanceIds:          instanceIds,
		AutoScalingGroupName: group.AutoScalingGroupName,
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("failed to attach instances: ", err.Error())
		return err
	}

	// change back ASG min size to original
	_, err = asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: group.AutoScalingGroupName,
		MinSize:              originalMinSize,
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("couldn't update min size", err.Error())
		return err
	}

	// wait until there are no pending instances in ASG
	nrOfPending := *originalDesiredCap
	for nrOfPending != 0 {
		nrOfPending = 0
		r, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: []*string{group.AutoScalingGroupName},
		})
		if err != nil {
			// TODO: timeout
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Error("couldn't describe ASG while waiting for pending instances")
		}
		for _, instance := range r.AutoScalingGroups[0].Instances {
			if *instance.LifecycleState == "Pending" {
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *vmPoolTask.VmPoolName,
					"taskID":           vmPoolTask.TaskID,
				}).Info("found a pending instance: ", *instance.InstanceId)
				nrOfPending++
			}
		}
		time.Sleep(1 * time.Second)
	}
	return nil
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

func requestAndWaitSpotInstances(ec2Svc *ec2.EC2, vmPoolTask *types.VmPoolTask, countsPerAZ map[string]int64, subnetsPerAZ map[string][]string, selectedInstanceTypes map[string][]recommender.InstanceTypeInfo, launchConfig autoscaling.LaunchConfiguration) ([]*string, error) {
	var instanceIds []*string
	var spotRequestIds []*string

	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("counts per az: ", countsPerAZ)
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("subnets per az: ", subnetsPerAZ)
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("selected instance types: ", selectedInstanceTypes)

	for az, selectedInstanceTypesInAZ := range selectedInstanceTypes {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info("az process started: ", az)
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info("selected instance types here are: ", selectedInstanceTypesInAZ)
		totalCountInAZ := countsPerAZ[az]
		countPerType := totalCountInAZ / int64(len(selectedInstanceTypesInAZ))
		remainderCount := totalCountInAZ % int64(len(selectedInstanceTypesInAZ))

		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info("total count in az: ", totalCountInAZ)
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info("count per type: ", countPerType)
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info("remainder: ", remainderCount)

		for i, instanceType := range selectedInstanceTypesInAZ {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Info("selected instance type: ", i, instanceType)
			countForType := countPerType
			if i == 0 {
				countForType = countForType + remainderCount
			}

			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Info("count for type: ", countForType)

			subnetsInAZ := subnetsPerAZ[az]

			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Info("subnets in AZ: ", subnetsInAZ)

			itCountPerSubnet := countForType / int64(len(subnetsInAZ))
			remainderItCount := countForType % int64(len(subnetsInAZ))

			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Info("instance type count per subnet: ", itCountPerSubnet)
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Info("remainder: ", remainderItCount)

			for i, subnet := range subnetsInAZ {
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *vmPoolTask.VmPoolName,
					"taskID":           vmPoolTask.TaskID,
				}).Info("processing subnet: ", subnet)

				itCountForSubnet := itCountPerSubnet
				if i == 0 {
					itCountForSubnet = itCountForSubnet + remainderItCount
				}

				log.WithFields(logrus.Fields{
					"autoScalingGroup": *vmPoolTask.VmPoolName,
					"taskID":           vmPoolTask.TaskID,
				}).Info("it count for subnet: ", itCountForSubnet)

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
						log.WithFields(logrus.Fields{
							"autoScalingGroup": *vmPoolTask.VmPoolName,
							"taskID":           vmPoolTask.TaskID,
						}).Info("couldn't request spot instances", err.Error())
						return nil, err
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
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Error("failed to describe spot requests")
			return nil, err
		}
		for _, spotReq := range spotRequests.SpotInstanceRequests {
			if spotReq.InstanceId != nil {
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *vmPoolTask.VmPoolName,
					"taskID":           vmPoolTask.TaskID,
				}).Info("instanceId of spot request is:", *spotReq.InstanceId)
				if *spotReq.InstanceId != "" {
					instanceIds = append(instanceIds, spotReq.InstanceId)
				}
			} else {
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *vmPoolTask.VmPoolName,
					"taskID":           vmPoolTask.TaskID,
				}).Info("instance id in request is null")
			}
		}
		time.Sleep(1 * time.Second)
	}

	// wait until new instances are running
	var currentlyRunning int64 = 0
	for totalCount != currentlyRunning {
		currentlyRunning = 0
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info("Describing instances")
		describeInstResult, err := ec2Svc.DescribeInstanceStatus(&ec2.DescribeInstanceStatusInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Error("failed to describe instances ", err.Error())
			return nil, err
		}
		log.Info("nr of instances in describe result ", len(describeInstResult.InstanceStatuses))
		for _, instanceStatus := range describeInstResult.InstanceStatuses {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Info(*instanceStatus.InstanceState.Name)
			if *instanceStatus.InstanceState.Name == "running" {
				currentlyRunning += 1
			}
		}
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info(fmt.Sprintf("currently running %v/%v", currentlyRunning, totalCount))
		time.Sleep(1 * time.Second)
	}
	return instanceIds, nil
}
