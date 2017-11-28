package aws

import (
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
	spotBidPrice string
}

type InstanceTypes map[InstanceType]int

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
		//TODO: error handling
	}
	log.Info("number of ASGs found:", len(result.AutoScalingGroups))

	for _, asg := range result.AutoScalingGroups {

		if !isHollowtreesManaged(asg) {
			continue
		}

		// collect instanceIds, and number of pending and terminating instances
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

		// ASG is running okay, but recommendation shows something else
		if nrOfPending == 0 && nrOfTerminating == 0 {
			log.Info("there are no operations in progress, checking current state")
			// TODO: cache this state
			state, err := getCurrentInstanceTypeState(ec2Svc, instanceIds, asg)
			if err != nil {
				//TODO error handling
			}
			// TODO: cache the recommendation as well
			recommendations, err := recommender.RecommendSpotInstanceTypes("eu-west-1", "eu-west-1a", "m4.xlarge")
			if err != nil {
				log.Info("couldn't get recommendations")
				//TODO error handling
			}

			// If there is at least one spot instance that's not recommended then create a rebalancing action
			for stateInfo := range state {
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

func getCurrentInstanceTypeState(ec2Svc *ec2.EC2, instanceIds []*string, asg *autoscaling.Group) (InstanceTypes, error) {
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
					spotBidPrice: "",
				}
				state[it] += 1
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
				spotBidPrice: *spotRequest.SpotPrice,
			}
			state[it] += 1
		}
	}
	log.Info("current state of instanceTypes in ASG: ", state)
	return state, err
}

func (asgm *AutoScalingGroupManager) UpdateVmPool(vmPoolTask *types.VmPoolTask) {
	// TODO: handle errors
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
	//TODO: updateLaunchConfig here
}
