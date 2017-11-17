package monitor

import (
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/sirupsen/logrus"
	"github.com/banzaicloud/hollowtrees/conf"
	"time"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"sync"
)

type InProgressRequests struct {
	r map[string]bool
	sync.Mutex
}

type Collector struct {
	PollPeriod time.Duration
	Requests   chan VmPoolRequest
	Results    chan VmPoolRequest
	InProgress InProgressRequests
}

func NewCollector(p time.Duration, requests chan VmPoolRequest, results chan VmPoolRequest) *Collector {
	return &Collector{
		PollPeriod: p,
		Requests:   requests,
		Results:    results,
		InProgress: InProgressRequests{
			r: make(map[string]bool),
		},
	}
}

func (c *Collector) Start() {
	log = conf.Logger()
	ticker := time.NewTicker(c.PollPeriod)

	session, err := session.NewSession()
	if err != nil {
		log.Info("we couldn't even create an AWS session")
	}
	asgSvc := autoscaling.New(session, aws.NewConfig().WithRegion("eu-west-1"))

	go func() {
		for {
			select {
			case work := <-c.Results:
				log.Info("Received finished request:", *work.VmPoolName)
				c.InProgress.Lock()
				c.InProgress.r[*work.VmPoolName] = false
				c.InProgress.Unlock()

			case <-ticker.C:
				// go func??? - if we kick off a go func here, it is possible that a "lot" of go funcs will wait here blocked
				// and it will cause a memory leak
				log.Info("ticker triggered:", time.Now())
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
							log.Info("Found ASG where it seems that there is work, but check it first...")
							c.InProgress.Lock()
							if !c.InProgress.r[*asg.AutoScalingGroupName] {
								c.InProgress.r[*asg.AutoScalingGroupName] = true
								c.Requests <- VmPoolRequest{VmPoolName: asg.AutoScalingGroupName}
								log.Info("Found ASG where we have some work, pushing to the queue...")
							} else {
								log.Info("It seems that we are already working on this ASG...")
							}
							c.InProgress.Unlock()
							log.Info("current time", time.Now(), "vs created time", *asg.CreatedTime)
						}
					}
				}
				log.Info("ticker finished:", time.Now())
			}
		}
	}()
}
