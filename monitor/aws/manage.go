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
	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

var log *logrus.Entry

func init() {
	log = conf.Logger().WithField("package", "monitor/aws")
}

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
	session, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		log.Info("Error creating session: ", err)
		return nil, err
	}
	return &AutoScalingGroupManager{
		session: session,
	}, nil
}

func (asgm *AutoScalingGroupManager) CheckVmPools() ([]*types.VmPoolTask, error) {
	var vmPoolTasks []*types.VmPoolTask
	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())

	result, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		log.Error("An error happened while describing AutoScaling Groups: " + err.Error())
		return nil, err
	}
	log.Info("Number of AutoScaling Groups found:", len(result.AutoScalingGroups))

	for _, asg := range result.AutoScalingGroups {

		if !isHollowtreesManaged(asg) {
			continue
		}

		nrOfPending, nrOfTerminating := getPendingAndTerminating(asg)

		// ASG is initializing if the desired cap is not zero but the nr of instances is 0 or all of them are pending
		if *asg.DesiredCapacity != 0 && (len(asg.Instances) == 0 || nrOfPending == len(asg.Instances)) {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *asg.AutoScalingGroupName,
			}).Info("AutoScaling Group is initializing, creating task")
			vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
				TaskID:       uuid.NewV4().String(),
				VmPoolName:   asg.AutoScalingGroupName,
				VmPoolAction: aws.String("initializing"),
			})
			continue
		}

		// ASG is upscaling if desired cap is not zero and nr of running+pending = desired cap
		if *asg.DesiredCapacity != 0 && nrOfPending != 0 && nrOfTerminating == 0 {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *asg.AutoScalingGroupName,
			}).Info("AutoScaling Group is upscaling, creating task")
			vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
				TaskID:       uuid.NewV4().String(),
				VmPoolName:   asg.AutoScalingGroupName,
				VmPoolAction: aws.String("upscaling"),
			})
			continue
		}

		// ASG is downscaling (or instances are terminated) if some instances are terminating
		if nrOfTerminating != 0 && nrOfPending == 0 {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *asg.AutoScalingGroupName,
			}).Info("AutoScaling Group is downscaling, creating task")
			vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
				TaskID:       uuid.NewV4().String(),
				VmPoolName:   asg.AutoScalingGroupName,
				VmPoolAction: aws.String("downscaling"),
			})
			continue
		}
	}

	for _, vmPoolTask := range vmPoolTasks {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
			"action":           *vmPoolTask.VmPoolAction,
		}).Info("Created task")
	}
	return vmPoolTasks, nil
}

func (asgm *AutoScalingGroupManager) ReevaluateVmPools() ([]*types.VmPoolTask, error) {
	var vmPoolTasks []*types.VmPoolTask

	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())
	ec2Svc := ec2.New(asgm.session, aws.NewConfig())

	result, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		log.Error("An error happened while describing AutoScaling Groups: " + err.Error())
		return nil, err
	}
	log.Info("number of AutoScaling Groups found:", len(result.AutoScalingGroups))

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

			// TODO: cache this state
			state, err := getCurrentInstanceTypeState(ec2Svc, *asg.AutoScalingGroupName, instanceIds)
			if err != nil {
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *asg.AutoScalingGroupName,
				}).Error("Couldn't get the current state of the AutoScaling group", err.Error())
				return nil, err
			}

			baseInstanceType, err := findBaseInstanceType(asgSvc, *asg.AutoScalingGroupName, *asg.LaunchConfigurationName)
			if err != nil {
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *asg.AutoScalingGroupName,
				}).Error("Couldn't find base instance type for the AutoScaling Group")
				return nil, err
			}

			// if we have an instance that is not recommended in the AZ where it is placed then signal

			// TODO: cache the recommendation as well
			recommendations, err := recommender.RecommendSpotInstanceTypes(*asgm.session.Config.Region, nil, baseInstanceType)
			if err != nil {
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *asg.AutoScalingGroupName,
				}).Error("Couldn't get instance type recommendations for AutoScaling Group")
				return nil, err
			}

			// If there is at least one spot instance that's not recommended then create a rebalancing action
			for stateInfo := range state {
				recommendationContains := false
				for _, recommendation := range recommendations[stateInfo.az] {
					if stateInfo.spotBidPrice != "" && recommendation.InstanceTypeName == stateInfo.instType {
						log.WithFields(logrus.Fields{
							"autoScalingGroup": *asg.AutoScalingGroupName,
						}).Info("recommendation contains: ", stateInfo.instType, " for AZ: ", stateInfo.az)
						recommendationContains = true
						break
					}
				}
				if !recommendationContains {
					log.WithFields(logrus.Fields{
						"autoScalingGroup": *asg.AutoScalingGroupName,
					}).Info("instanceType ", stateInfo, " is not among recommendations, creating rebalancing task")
					vmPoolTasks = append(vmPoolTasks, &types.VmPoolTask{
						TaskID:       uuid.NewV4().String(),
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

	for _, vmPoolTask := range vmPoolTasks {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
			"action":           *vmPoolTask.VmPoolAction,
		}).Info("Created task")
	}

	return vmPoolTasks, nil
}

func isHollowtreesManaged(asg *autoscaling.Group) bool {
	for _, tag := range asg.Tags {
		if *tag.Key == "Hollowtrees" && *tag.Value == "true" {
			return true
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *asg.AutoScalingGroupName,
			}).Info("Found a Hollowtrees managed AutoScaling Group: ", asg.AutoScalingGroupName)
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
			"autoScalingGroup": *asg.AutoScalingGroupName,
		}).Info("desired/current/pending:", *asg.DesiredCapacity, len(asg.Instances), nrOfPending)
	}
	return nrOfPending, nrOfTerminating
}

func getCurrentInstanceTypeState(ec2Svc *ec2.EC2, asgName string, instanceIds []*string) (InstanceTypes, error) {
	if len(instanceIds) < 1 {
		return nil, errors.New("number of instance ids cannot be less than 1")
	}
	instances, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: instanceIds,
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": asgName,
		}).Error("Failed to describe instances in AutoScaling Group: ", err.Error())
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
			log.WithFields(logrus.Fields{
				"autoScalingGroup": asgName,
			}).Error("Failed to describe spot requests: ", err.Error())
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
	log.WithFields(logrus.Fields{
		"autoScalingGroup": asgName,
	}).Info("current state of instanceTypes in ASG: ", state)
	return state, err
}

func findBaseInstanceType(asgSvc *autoscaling.AutoScaling, asgName string, lcName string) (string, error) {
	originalLCName := asgName + "-ht-orig"
	originalLaunchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{&originalLCName},
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": asgName,
		}).Error("Failed to describe launch configurations: ", err.Error())
		return "", err
	}
	log.WithFields(logrus.Fields{
		"autoScalingGroup": asgName,
	}).Info("Described original LaunchConfigs, length of result is: ", len(originalLaunchConfigs.LaunchConfigurations))

	if len(originalLaunchConfigs.LaunchConfigurations) > 0 {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": asgName,
		}).Info("Base instance type is: ", *originalLaunchConfigs.LaunchConfigurations[0].InstanceType)
		return *originalLaunchConfigs.LaunchConfigurations[0].InstanceType, nil
	} else {
		launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
			LaunchConfigurationNames: []*string{&lcName},
		})
		if err != nil {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": asgName,
			}).Error("something happened during describing launch configs" + err.Error())
			return "", err
		}
		log.WithFields(logrus.Fields{
			"autoScalingGroup": asgName,
		}).Info("Base instance type is: ", *launchConfigs.LaunchConfigurations[0].InstanceType)
		return *launchConfigs.LaunchConfigurations[0].InstanceType, nil
	}
}

func cleanupLCs(asgSvc *autoscaling.AutoScaling, managedASGNames []string) {
	lcResult, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{})
	if err != nil {
		log.Error("Couldn't describe Launch Configurations, cleanup won't continue. " + err.Error())
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
					log.WithFields(logrus.Fields{
						"launchConfiguration": *lc.LaunchConfigurationName,
					}).Error("couldn't clean up Launch Configuration: " + err.Error())
				}
			}
		}
	}
}

func (asgm *AutoScalingGroupManager) UpdateVmPool(vmPoolTask *types.VmPoolTask) error {
	switch *vmPoolTask.VmPoolAction {
	case "initializing":
		if err := initializeASG(asgm, vmPoolTask); err != nil {
			return err
		}
	case "upscaling":
		if err := upscaleASG(asgm, vmPoolTask); err != nil {
			return err
		}
	case "downscaling":
		if err := downscaleASG(asgm, vmPoolTask); err != nil {
			return err
		}
	case "rebalancing":
		if err := rebalanceASG(asgm, vmPoolTask); err != nil {
			return err
		}
	}
	if err := updateLaunchConfig(asgm, vmPoolTask); err != nil {
		return err
	}
	return nil
}
