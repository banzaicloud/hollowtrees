package aws

import (
	"sort"

	"strconv"

	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/banzaicloud/hollowtrees/monitor/types"
	"github.com/banzaicloud/hollowtrees/recommender"
	"github.com/sirupsen/logrus"
)

func updateLaunchConfig(asgm *AutoScalingGroupManager, vmPoolTask *types.VmPoolTask) error {
	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Updating Launch Configuration of the Auto Scaling Group")

	describeAsgResult, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{vmPoolTask.VmPoolName},
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("something happened while polling ASGs" + err.Error())
		return err
	}
	if len(describeAsgResult.AutoScalingGroups) < 1 {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("Autoscaling group is probably deleted.")
		return errors.New("described ASG had empty result, it was probably deleted")
	}
	group := describeAsgResult.AutoScalingGroups[0]
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Described ASG, name is: ", *group.AutoScalingGroupName)

	launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{group.LaunchConfigurationName},
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("something happened during describing launch configs" + err.Error())
		return err
	}
	currentLaunchConfig := *launchConfigs.LaunchConfigurations[0]
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Described current LaunchConfig, name is: ", *currentLaunchConfig.LaunchConfigurationName)

	var baseInstanceType string

	originalLCName := *group.AutoScalingGroupName + "-ht-orig"
	originalLaunchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{&originalLCName},
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("something happened during describing launch configs" + err.Error())
		return err
	}
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Described original LaunchConfigs, length of result is: ", len(originalLaunchConfigs.LaunchConfigurations))

	if len(originalLaunchConfigs.LaunchConfigurations) < 1 {
		baseInstanceType = *currentLaunchConfig.InstanceType
		_, err = asgSvc.CreateLaunchConfiguration(&autoscaling.CreateLaunchConfigurationInput{
			LaunchConfigurationName:      &originalLCName,
			InstanceType:                 currentLaunchConfig.InstanceType,
			SpotPrice:                    currentLaunchConfig.SpotPrice,
			AssociatePublicIpAddress:     currentLaunchConfig.AssociatePublicIpAddress,
			BlockDeviceMappings:          currentLaunchConfig.BlockDeviceMappings,
			ClassicLinkVPCId:             currentLaunchConfig.ClassicLinkVPCId,
			ClassicLinkVPCSecurityGroups: currentLaunchConfig.ClassicLinkVPCSecurityGroups,
			EbsOptimized:                 currentLaunchConfig.EbsOptimized,
			IamInstanceProfile:           currentLaunchConfig.IamInstanceProfile,
			ImageId:                      currentLaunchConfig.ImageId,
			InstanceMonitoring:           currentLaunchConfig.InstanceMonitoring,
			//KernelId:                     currentLaunchConfig.KernelId,
			KeyName:          currentLaunchConfig.KeyName,
			PlacementTenancy: currentLaunchConfig.PlacementTenancy,
			//RamdiskId:                    currentLaunchConfig.RamdiskId,
			SecurityGroups: currentLaunchConfig.SecurityGroups,
			UserData:       currentLaunchConfig.UserData,
		})
		if err != nil {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Error("Couldn't copy original launch config to backup launch config" + err.Error())
			return err
		}
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info("Created copy of original LaunchConfig: ", originalLCName)
	} else {
		baseInstanceType = *originalLaunchConfigs.LaunchConfigurations[0].InstanceType
	}

	recommendations, err := recommender.RecommendSpotInstanceTypes(*asgm.session.Config.Region, aws.StringValueSlice(group.AvailabilityZones), baseInstanceType)
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("couldn't recommend spot instance types: " + err.Error())
		return err
	}
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("recommendations in selected AZs are ", recommendations)

	// TODO: if recommendation matches the lc then return
	var commonRecommendations []recommender.InstanceTypeInfo
	az, baseAzRecommendations := getRandomAzRecommendation(recommendations)

	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Selected a recommendation list from a random AZ: ", az)
	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Finding intersect of recommendations in every AZ")

	for _, baseRecommendation := range baseAzRecommendations {
		foundInAll := true
		for _, azRecommendations := range recommendations {
			foundInCurrent := false
			for _, rec := range azRecommendations {
				onDemandPrice, _ := strconv.ParseFloat(rec.OnDemandPrice, 32)
				currentPrice, _ := strconv.ParseFloat(rec.CurrentPrice, 32)
				if rec.InstanceTypeName == baseRecommendation.InstanceTypeName && currentPrice < onDemandPrice {
					foundInCurrent = true
				}
			}
			if foundInCurrent == false {
				foundInAll = false
			}
		}
		if foundInAll {
			// TODO: should append with the max spot price of all AZs
			commonRecommendations = append(commonRecommendations, baseRecommendation)
		}
	}

	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Intersect of recommendations: ", commonRecommendations)

	var recommendation recommender.InstanceTypeInfo
	if len(commonRecommendations) > 0 {
		sort.Sort(sort.Reverse(ByCostScore(commonRecommendations)))
		recommendation = commonRecommendations[0]
	} else {
		sort.Sort(sort.Reverse(ByCostScore(baseAzRecommendations)))
		recommendation = baseAzRecommendations[0]
	}

	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Recommendation selected for new launch config: ", recommendation)

	var newLCName string
	if *currentLaunchConfig.LaunchConfigurationName == *group.AutoScalingGroupName+"-ht-1" {
		newLCName = *group.AutoScalingGroupName + "-ht-2"
	} else {
		newLCName = *group.AutoScalingGroupName + "-ht-1"
	}

	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Name of new launch config: ", newLCName)

	_, err = asgSvc.CreateLaunchConfiguration(&autoscaling.CreateLaunchConfigurationInput{
		LaunchConfigurationName:      &newLCName,
		InstanceType:                 &recommendation.InstanceTypeName,
		SpotPrice:                    &recommendation.SuggestedBidPrice,
		AssociatePublicIpAddress:     currentLaunchConfig.AssociatePublicIpAddress,
		BlockDeviceMappings:          currentLaunchConfig.BlockDeviceMappings,
		ClassicLinkVPCId:             currentLaunchConfig.ClassicLinkVPCId,
		ClassicLinkVPCSecurityGroups: currentLaunchConfig.ClassicLinkVPCSecurityGroups,
		EbsOptimized:                 currentLaunchConfig.EbsOptimized,
		IamInstanceProfile:           currentLaunchConfig.IamInstanceProfile,
		ImageId:                      currentLaunchConfig.ImageId,
		InstanceMonitoring:           currentLaunchConfig.InstanceMonitoring,
		//KernelId:                     currentLaunchConfig.KernelId,
		KeyName:          currentLaunchConfig.KeyName,
		PlacementTenancy: currentLaunchConfig.PlacementTenancy,
		//RamdiskId:                    currentLaunchConfig.RamdiskId,
		SecurityGroups: currentLaunchConfig.SecurityGroups,
		UserData:       currentLaunchConfig.UserData,
	})
	if err != nil {
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Error("couldn't create a new launch configuration: " + err.Error())
		return err
	}

	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Created new launch config: ", newLCName)

	_, err = asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName:    group.AutoScalingGroupName,
		LaunchConfigurationName: &newLCName,
	})
	if err != nil {
		log.Error("couldn't update the autoscaling group with the new launch configuration: " + err.Error())
		return err
	}

	log.WithFields(logrus.Fields{
		"autoScalingGroup": *vmPoolTask.VmPoolName,
		"taskID":           vmPoolTask.TaskID,
	}).Info("Updated auto scaling group with new launch config: ", *group.AutoScalingGroupName, newLCName)

	if *currentLaunchConfig.LaunchConfigurationName == *group.AutoScalingGroupName+"-ht-1" ||
		*currentLaunchConfig.LaunchConfigurationName == *group.AutoScalingGroupName+"-ht-2" {
		_, err = asgSvc.DeleteLaunchConfiguration(&autoscaling.DeleteLaunchConfigurationInput{
			LaunchConfigurationName: currentLaunchConfig.LaunchConfigurationName,
		})
		if err != nil {
			log.WithFields(logrus.Fields{
				"autoScalingGroup": *vmPoolTask.VmPoolName,
				"taskID":           vmPoolTask.TaskID,
			}).Error("couldn't delete the deprecated launch configuration: " + err.Error())
			return err
		}
		log.WithFields(logrus.Fields{
			"autoScalingGroup": *vmPoolTask.VmPoolName,
			"taskID":           vmPoolTask.TaskID,
		}).Info("Deleted old launch config: ", *currentLaunchConfig.LaunchConfigurationName)
	}
	return nil
}

func getRandomAzRecommendation(azRecommendations map[string][]recommender.InstanceTypeInfo) (string, []recommender.InstanceTypeInfo) {
	for az, azRec := range azRecommendations {
		return az, azRec
	}
	return "", nil
}
