package monitor

import (
	"fmt"
	"time"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/sirupsen/logrus"
	"github.com/banzaicloud/hollowtrees/conf"
)

var log *logrus.Logger

func Start() {
	ticker := time.NewTicker(3 * time.Second)
	for range ticker.C {
		log = conf.Logger()
		fmt.Println("ticker triggered:", time.Now())
		session, err := session.NewSession()
		asgSvc := autoscaling.New(session, aws.NewConfig().WithRegion("eu-west-1"))
		result, err := asgSvc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{

		})
		if err != nil {
			log.Error("something happened while polling ASGs" + err.Error())
		}
		log.Info("number of ASGs found:", len(result.AutoScalingGroups))
		for _, asg := range result.AutoScalingGroups {
			log.Info("Name:", asg.AutoScalingGroupName)
			for _, tag := range asg.Tags {
				log.Info(tag.Key)
				log.Info(tag.Value)
				if *tag.Key == "Hollowtrees" {
					log.Info("desired", asg.DesiredCapacity)
					log.Info("nr of instances", len(asg.Instances))
				}
			}
		}
		fmt.Println("ticker finished:", time.Now())
	}

}
