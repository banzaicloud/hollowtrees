package aws

import (
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
			log.Info("this instance will be changed to a different one because it is not among the recommended options:", stateInfo)
		}
	}
}
