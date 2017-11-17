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

type CurrentRequests struct {
	r map[string]bool
	sync.Mutex
}

type Collector struct {
	pollPeriod       time.Duration
	poolRequestQueue chan PoolRequest
	finishQueue      chan PoolRequest
	requests         CurrentRequests
}

func NewCollector(p time.Duration, requestQueue chan PoolRequest, responseQueue chan PoolRequest) *Collector {
	return &Collector{
		pollPeriod:       p,
		poolRequestQueue: requestQueue,
		finishQueue:      responseQueue,
		requests: CurrentRequests{
			r: make(map[string]bool),
		},
	}
}

func (c *Collector) Start() {
	log = conf.Logger()
	ticker := time.NewTicker(c.pollPeriod)

	session, err := session.NewSession()
	if err != nil {
		log.Info("we couldn't even create an AWS session")
	}
	asgSvc := autoscaling.New(session, aws.NewConfig().WithRegion("eu-west-1"))

	go func() {
		for {
			select {
			case work := <-c.finishQueue:
				log.Info("Received finished request:", *work.asg.AutoScalingGroupName)
				c.requests.Lock()
				c.requests.r[*work.asg.AutoScalingGroupName] = false
				c.requests.Unlock()

			case <-ticker.C:
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
						//originalMinSize := *asg.MinSize
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
							c.requests.Lock()
							if !c.requests.r[*asg.AutoScalingGroupName] {
								c.requests.r[*asg.AutoScalingGroupName] = true
								c.poolRequestQueue <- PoolRequest{asg: *asg}
								log.Info("Found ASG where we have some work, pushing to the queue...")
							} else {
								log.Info("It seems that we are already working on this ASG...")
							}
							c.requests.Unlock()
							log.Info("current time", time.Now(), "vs created time", *asg.CreatedTime)
						}
					}
				}
				log.Info("ticker finished:", time.Now())
			}
		}
	}()
}
