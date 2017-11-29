package monitor

import (
	"sync"
	"time"

	"github.com/banzaicloud/hollowtrees/conf"
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
	log = conf.Logger()
	ticker := time.NewTicker(c.PollPeriod)
	reevaluatingTicker := time.NewTicker(c.ReevaluatingPollPeriod)

	go func() {
		for {
			select {
			case result := <-c.Results:
				log.Info("Received result:", *result.VmPoolTask.VmPoolName)
				c.InProgress.Lock()
				c.InProgress.r[*result.VmPoolTask.VmPoolName] = false
				c.InProgress.Unlock()

			case <-ticker.C:
				log.Info("ticker triggered:", time.Now())
				for _, vmPool := range c.VmPoolManager.MonitorVmPools() {
					c.InProgress.Lock()
					if !c.InProgress.r[*vmPool.VmPoolName] {
						c.InProgress.r[*vmPool.VmPoolName] = true
						c.Requests <- VmPoolRequest{VmPoolTask: vmPool}
						log.Info("Pushing VM pool to processor queue ", *vmPool)
					} else {
						log.Info("A processor is already working on this VM pool ", *vmPool)
					}
					c.InProgress.Unlock()
				}
				log.Info("ticker finished:", time.Now())
			}
		}
	}()

	go func() {
		for {
			select {
			case <-reevaluatingTicker.C:
				log.Info("ticker triggered:", time.Now())
				for _, vmPool := range c.VmPoolManager.ReevaluateVmPools() {
					c.InProgress.Lock()
					if !c.InProgress.r[*vmPool.VmPoolName] {
						c.InProgress.r[*vmPool.VmPoolName] = true
						c.Requests <- VmPoolRequest{VmPoolTask: vmPool}
						log.Info("Pushing VM pool to processor queue ", *vmPool)
					} else {
						log.Info("A processor is already working on this VM pool, won't reevaluate it now. ", *vmPool)
					}
					c.InProgress.Unlock()
				}
				log.Info("ticker finished:", time.Now())
			}
		}
	}()
}
