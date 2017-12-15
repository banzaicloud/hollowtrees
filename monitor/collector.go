package monitor

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type InProgressRequests struct {
	r map[string]bool
	sync.Mutex
}

type Collector struct {
	PollPeriod             time.Duration
	ReevaluatingPollPeriod time.Duration
	Requests               chan VmPoolRequest
	Results                chan VmPoolRequest
	InProgress             InProgressRequests
	VmPoolManager          VmPoolManager
}

func NewCollector(p time.Duration, rp time.Duration, requests chan VmPoolRequest, results chan VmPoolRequest, manager VmPoolManager) *Collector {
	return &Collector{
		PollPeriod:             p,
		ReevaluatingPollPeriod: rp,
		Requests:               requests,
		Results:                results,
		InProgress: InProgressRequests{
			r: make(map[string]bool),
		},
		VmPoolManager: manager,
	}
}

func (c *Collector) Start() {
	ticker := time.NewTicker(c.PollPeriod)
	reevaluatingTicker := time.NewTicker(c.ReevaluatingPollPeriod)

	go func() {
		for {
			select {
			case result := <-c.Results:
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *result.VmPoolTask.VmPoolName,
					"taskId":           result.VmPoolTask.TaskID,
					"action":           *result.VmPoolTask.VmPoolAction,
				}).Info("Received task result")
				c.InProgress.Lock()
				c.InProgress.r[*result.VmPoolTask.VmPoolName] = false
				c.InProgress.Unlock()

			case <-ticker.C:
				log.Info("ticker triggered:", time.Now())
				vmPoolTasks, err := c.VmPoolManager.CheckVmPools()
				if err != nil {
					log.Error("Failed to check VM pools in this tick: ", err)
				}
				for _, vmPool := range vmPoolTasks {
					c.InProgress.Lock()
					if !c.InProgress.r[*vmPool.VmPoolName] {
						c.InProgress.r[*vmPool.VmPoolName] = true
						c.Requests <- VmPoolRequest{VmPoolTask: vmPool}
						log.WithFields(logrus.Fields{
							"autoScalingGroup": *vmPool.VmPoolName,
							"taskId":           vmPool.TaskID,
							"action":           *vmPool.VmPoolAction,
						}).Info("Pushing VM pool task to processor queue")
					} else {
						log.WithFields(logrus.Fields{
							"autoScalingGroup": *vmPool.VmPoolName,
							"taskId":           vmPool.TaskID,
							"action":           *vmPool.VmPoolAction,
						}).Info("A processor is already working on this VM pool")
					}
					c.InProgress.Unlock()
				}
				log.Info("ticker finished:", time.Now())
			}
		}
	}()

	for {
		select {
		case <-reevaluatingTicker.C:
			log.Info("ticker triggered:", time.Now())
			vmPoolTasks, err := c.VmPoolManager.ReevaluateVmPools()
			if err != nil {
				log.Error("Failed to reevaluate VM pools in this tick: ", err)
			}
			if vmPoolTasks != nil {
				for _, vmPool := range vmPoolTasks {
					c.InProgress.Lock()
					if !c.InProgress.r[*vmPool.VmPoolName] {
						c.InProgress.r[*vmPool.VmPoolName] = true
						c.Requests <- VmPoolRequest{VmPoolTask: vmPool}
						log.WithFields(logrus.Fields{
							"autoScalingGroup": *vmPool.VmPoolName,
							"taskId":           vmPool.TaskID,
							"action":           *vmPool.VmPoolAction,
						}).Info("Pushing VM pool task to processor queue")
					} else {
						log.WithFields(logrus.Fields{
							"autoScalingGroup": *vmPool.VmPoolName,
							"taskId":           vmPool.TaskID,
							"action":           *vmPool.VmPoolAction,
						}).Info("A processor is already working on this VM pool")
					}
					c.InProgress.Unlock()
				}
			}
			log.Info("ticker finished:", time.Now())
		}
	}
}
