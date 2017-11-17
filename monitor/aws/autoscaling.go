package aws

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/banzaicloud/hollowtrees/conf"
	"time"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/banzaicloud/hollowtrees/recommender"
)

var log *logrus.Logger

type AutoScalingGroupManager struct {
	session *session.Session
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

func (asgm *AutoScalingGroupManager) CollectVmPools() []*string {
	var vmPools []*string
	log = conf.Logger()
	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())
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
			log.Info("current time", time.Now(), "vs created time", *asg.CreatedTime)
			desiredCapacity := *asg.DesiredCapacity
			nrOfPending := 0
			for _, instance := range asg.Instances {
				if *instance.LifecycleState == "Pending" {
					nrOfPending++
				}
			}
			log.WithFields(logrus.Fields{
				"asg": *asg.AutoScalingGroupName,
			}).Info("desired/current/pending:", *asg.DesiredCapacity, len(asg.Instances), nrOfPending)
			if desiredCapacity != 0 && (len(asg.Instances) == 0 || nrOfPending == len(asg.Instances)) {
				log.Info("Found ASG where it seems that there is work")
				vmPools = append(vmPools, asg.AutoScalingGroupName)
			}
		}
	}
	return vmPools
}

func (asgm *AutoScalingGroupManager) UpdateVmPool(vmPoolName *string) {
	//for i := 0; i < 32; i++ {
	//	time.Sleep(1 * time.Second)
	//	log.Info("sleeping .. ", *vmPoolName, i)
	//}
	ec2Svc := ec2.New(asgm.session, aws.NewConfig())
	asgSvc := autoscaling.New(asgm.session, aws.NewConfig())

	describeResult, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{vmPoolName},
	})
	if err != nil {
		log.Error("something happened while polling ASGs" + err.Error())
	}
	group := describeResult.AutoScalingGroups[0]
	originalDesiredCap := group.DesiredCapacity
	originalMinSize := group.MinSize

	log.Info("updating to 0 desired")
	result, err := asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: group.AutoScalingGroupName,
		DesiredCapacity:      aws.Int64(0),
		MinSize:              aws.Int64(0),
	})
	if err != nil {
		log.Info("error happened during updating to 0 desired", err.Error())
	}
	log.Info(result)
	log.Info("updated to 0 desired")
	launchConfigs, err := asgSvc.DescribeLaunchConfigurations(&autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{group.LaunchConfigurationName},
	})
	if err != nil {
		log.Error("something happened during describing launch configs" + err.Error())
	}
	launchConfig := *launchConfigs.LaunchConfigurations[0]
	log.Info("launch config instance type is", *launchConfig.InstanceType)
	recommendations, err := recommender.RecommendSpotInstanceTypes("eu-west-1", "1a", *launchConfig.InstanceType)
	if err != nil {
		log.Error("couldn't recommend spot instance types" + err.Error())
	}
	instanceTypes := recommendations["eu-west-1a"]
	log.Info("recommendations in eu-west-1a are", instanceTypes)
	instType := instanceTypes[0]
	log.Info("instance type: ", instType.InstanceTypeName)
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
					Groups:                   launchConfig.SecurityGroups,
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
		_, err := asgSvc.AttachInstances(&autoscaling.AttachInstancesInput{
			InstanceIds:          instanceIds,
			AutoScalingGroupName: group.AutoScalingGroupName,
		})
		if err != nil {
			log.Info("failed to attach instances: ", err.Error())
		}
		log.Info("")
		_, err2 := asgSvc.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
			AutoScalingGroupName: group.AutoScalingGroupName,
			MinSize:              originalMinSize,
		})
		if err2 != nil {
			log.Info("couldn't update min size", err2.Error())
		}
	}

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
