package engine

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/sirupsen/logrus"
	"github.com/banzaicloud/hollowtrees/conf"
)

type HollowGroupRequest struct {
	AutoScalingGroupName string `json:"autoScalingGroupName" binding:"required"`
}

var log *logrus.Logger

func CreateHollowGroup(group HollowGroupRequest) (string, error) {
	session, err := session.NewSession()
	if err != nil {
		fmt.Println("Error creating session ", err)
		return "", err
	}
	asgSvc := autoscaling.New(session, aws.NewConfig().WithRegion("eu-west-1"))
	result, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{&group.AutoScalingGroupName},
	})
	if err != nil {
		return "", err
	}
	log = conf.Logger()
	log.Info(result)

	return group.AutoScalingGroupName, nil
}
