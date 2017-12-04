package aws

import (
	"errors"

	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/monitor/types"
	"github.com/banzaicloud/hollowtrees/recommender"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

type AutoScalingGroupManager struct {
	session *session.Session
}

type InstanceType struct {
	instType     string
	az           string
	spotBidPrice string
}

type InstanceTypes map[InstanceType][]*string

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

func (asgm *AutoScalingGroupManager) MonitorVmPools() []*types.VmPoolTask {
	var vmPoolTasks []*types.VmPoolTask
	log = conf.Logger()
	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())

	result, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		log.Error("something happened while polling ASGs" + err.Error())
		//TODO: error handling
	}
	log.Info("number of ASGs found:", len(result.AutoScalingGroups))

	for _, asg := range result.AutoScalingGroups {

		if !isHollowtreesManaged(asg) {
			continue
		}

		nrOfPending, nrOfTerminating := getPendingAndTerminating(asg)

		// ASG is initializing if the desired cap is not zero but the nr of instances is 0 or all of them are pending
		if *asg.DesiredCapacity != 0 && (len(asg.Instances) == 0 || nrOfPending == len(asg.Instances)) {
			log.Info("ASG is initializing")
			vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
				VmPoolName:   asg.AutoScalingGroupName,
				VmPoolAction: aws.String("initializing"),
			})
			continue
		}

		// ASG is upscaling if desired cap is not zero and nr of running+pending = desired cap
		if *asg.DesiredCapacity != 0 && nrOfPending != 0 && nrOfTerminating == 0 {
			log.Info("ASG is upscaling")
			vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
				VmPoolName:   asg.AutoScalingGroupName,
				VmPoolAction: aws.String("upscaling"),
			})
			continue
		}

		// ASG is downscaling (or instances are terminated) if some instances are terminating
		if nrOfTerminating != 0 && nrOfPending == 0 {
			log.Info("ASG is downscaling")
			vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
				VmPoolName:   asg.AutoScalingGroupName,
				VmPoolAction: aws.String("downscaling"),
			})
			continue
		}
	}

	return vmPoolTasks
}

func (asgm *AutoScalingGroupManager) ReevaluateVmPools() []*types.VmPoolTask {
	var vmPoolTasks []*types.VmPoolTask
	log = conf.Logger()

	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())
	ec2Svc := ec2.New(asgm.session, aws.NewConfig())

	result, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		log.Error("something happened while polling ASGs" + err.Error())
		//TODO: error handling
	}
	log.Info("number of ASGs found:", len(result.AutoScalingGroups))

	var managedASGNames []string

	for _, asg := range result.AutoScalingGroups {

		if !isHollowtreesManaged(asg) {
			continue
		}

		managedASGNames = append(managedASGNames, *asg.AutoScalingGroupName)

		nrOfPending, nrOfTerminating := getPendingAndTerminating(asg)

		// ASG is running okay, but recommendation shows something else
		if nrOfPending == 0 && nrOfTerminating == 0 {

			var instanceIds []*string
			if len(asg.Instances) > 0 {
				for _, instance := range asg.Instances {
					instanceIds = append(instanceIds, instance.InstanceId)
				}
			}

			log.Info("there are no operations in progress, checking current state")
			// TODO: cache this state
			state, err := getCurrentInstanceTypeState(ec2Svc, instanceIds)
			if err != nil {
				//TODO error handling
				log.Info(err.Error())
				return nil
			}

			baseInstanceType, err := findBaseInstanceType(asgSvc, *asg.AutoScalingGroupName, *asg.LaunchConfigurationName)
			if err != nil {
				log.Info("couldn't find base instance type")
				//TODO error handling
			}

			// if we have an instance that is not recommended in the AZ where it is placed then signal

			// TODO: cache the recommendation as well
			recommendations, err := recommender.RecommendSpotInstanceTypes(*asgm.session.Config.Region, nil, baseInstanceType)
			if err != nil {
				log.Info("couldn't get recommendations")
				//TODO error handling
			}

			// If there is at least one spot instance that's not recommended then create a rebalancing action
			for stateInfo := range state {
				recommendationContains := false
				for _, recommendation := range recommendations[stateInfo.az] {
					if stateInfo.spotBidPrice != "" && recommendation.InstanceTypeName == stateInfo.instType {
						log.Info("recommendation contains: ", stateInfo.instType, " for AZ: ", stateInfo.az)
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

			// TODO: If launch config is not the same as the recommended one then create a launch config renew action
		}
	}

	cleanupLCs(asgSvc, managedASGNames)

	return vmPoolTasks
}

func isHollowtreesManaged(asg *autoscaling.Group) bool {
	for _, tag := range asg.Tags {
		if *tag.Key == "Hollowtrees" && *tag.Value == "true" {
			return true
			log.Info("Found a Hollowtrees managed AutoScaling Group: ", asg.AutoScalingGroupName)
		}
	}
	return false
}

func getPendingAndTerminating(asg *autoscaling.Group) (int, int) {
	nrOfPending := 0
	nrOfTerminating := 0
	if len(asg.Instances) > 0 {
		for _, instance := range asg.Instances {
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
	return nrOfPending, nrOfTerminating
}

func getCurrentInstanceTypeState(ec2Svc *ec2.EC2, instanceIds []*string) (InstanceTypes, error) {
	if len(instanceIds) < 1 {
		return nil, errors.New("number of instance ids cannot be less than 1")
	}
	instances, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: instanceIds,
	})
	if err != nil {
		log.Error("failed to describe instances: ", err.Error())
		return nil, err
	}

	state := make(InstanceTypes)

	var spotRequests []*string
	for _, reservation := range instances.Reservations {
		for _, instance := range reservation.Instances {
			if instance.SpotInstanceRequestId != nil {
				spotRequests = append(spotRequests, instance.SpotInstanceRequestId)
			} else {
				it := InstanceType{
					instType:     *instance.InstanceType,
					az:           *instance.Placement.AvailabilityZone,
					spotBidPrice: "",
				}
				state[it] = append(state[it], instance.InstanceId)
			}
		}
	}
	if len(spotRequests) > 0 {
		output, err := ec2Svc.DescribeSpotInstanceRequests(&ec2.DescribeSpotInstanceRequestsInput{
			SpotInstanceRequestIds: spotRequests,
		})
		if err != nil {
			log.Error("failed to describe spot requests ")
			return nil, err
		}

		for _, spotRequest := range output.SpotInstanceRequests {
			it := InstanceType{
				instType:     *spotRequest.LaunchSpecification.InstanceType,
				az:           *spotRequest.LaunchedAvailabilityZone,
				spotBidPrice: *spotRequest.SpotPrice,
			}
			state[it] = append(state[it], spotRequest.InstanceId)
		}
	}
	log.Info("current state of instanceTypes in ASG: ", state)
	return state, err
}

func findBaseInstanceType(asgSvc *autoscaling.AutoScaling, asgName string, lcName string) (string, error) {
	originalLCName := asgName + "-ht-orig"
	originalLaunchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{&originalLCName},
	})
	if err != nil {
		log.Error("something happened during describing launch configs" + err.Error())
		return "", err
	}
	log.Info("Described original LaunchConfigs, length of result is: ", len(originalLaunchConfigs.LaunchConfigurations))

	if len(originalLaunchConfigs.LaunchConfigurations) > 0 {
		log.Info("Base instance type is: ", *originalLaunchConfigs.LaunchConfigurations[0].InstanceType)
		return *originalLaunchConfigs.LaunchConfigurations[0].InstanceType, nil
	} else {
		launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
			LaunchConfigurationNames: []*string{&lcName},
		})
		if err != nil {
			log.Error("something happened during describing launch configs" + err.Error())
			return "", err
		}
		log.Info("Base instance type is: ", *launchConfigs.LaunchConfigurations[0].InstanceType)
		return *launchConfigs.LaunchConfigurations[0].InstanceType, nil
	}
}

func cleanupLCs(asgSvc *autoscaling.AutoScaling, managedASGNames []string) {
	lcResult, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{})
	if err != nil {
		log.Error("something happened while polling LCs" + err.Error())
		//TODO: error handling
	}
	log.Info("number of LCs found:", len(lcResult.LaunchConfigurations))
	for _, lc := range lcResult.LaunchConfigurations {
		var asgName = ""
		if strings.HasSuffix(*lc.LaunchConfigurationName, "-ht-orig") {
			asgName = strings.TrimSuffix(*lc.LaunchConfigurationName, "-ht-orig")
		} else if strings.HasSuffix(*lc.LaunchConfigurationName, "-ht-1") {
			asgName = strings.TrimSuffix(*lc.LaunchConfigurationName, "-ht-1")
		} else if strings.HasSuffix(*lc.LaunchConfigurationName, "-ht-2") {
			asgName = strings.TrimSuffix(*lc.LaunchConfigurationName, "-ht-2")
		}

		if asgName != "" {
			foundASG := false
			for _, managedASGName := range managedASGNames {
				if asgName == managedASGName {
					foundASG = true
				}
			}
			if !foundASG {
				_, err = asgSvc.DeleteLaunchConfiguration(&autoscaling.DeleteLaunchConfigurationInput{
					LaunchConfigurationName: lc.LaunchConfigurationName,
				})
				if err != nil {
					log.Error("couldn't clean up LC: " + err.Error())
					//TODO: error handling
				}
			}
		}
	}
}

func (asgm *AutoScalingGroupManager) UpdateVmPool(vmPoolTask *types.VmPoolTask) {
	switch *vmPoolTask.VmPoolAction {
	case "initializing":
		initializeASG(asgm, vmPoolTask.VmPoolName)
	case "upscaling":
		upscaleASG(asgm, vmPoolTask.VmPoolName)
	case "downscaling":
		downscaleASG(asgm, vmPoolTask.VmPoolName)
	case "rebalancing":
		rebalanceASG(asgm, vmPoolTask.VmPoolName)
	}
	updateLaunchConfig(asgm, vmPoolTask.VmPoolName)
}
