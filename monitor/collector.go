package monitor

import (
	"github.com/banzaicloud/hollowtrees/conf"
	"time"
	"sync"
)

type InProgressRequests struct {
	r map[string]bool
	sync.Mutex
}

type Collector struct {
	PollPeriod    time.Duration
	Requests      chan VmPoolRequest
	Results       chan VmPoolRequest
	InProgress    InProgressRequests
	VmPoolManager VmPoolManager
}

func NewCollector(p time.Duration, requests chan VmPoolRequest, results chan VmPoolRequest, manager VmPoolManager) *Collector {
	return &Collector{
		PollPeriod: p,
		Requests:   requests,
		Results:    results,
		InProgress: InProgressRequests{
			r: make(map[string]bool),
		},
		VmPoolManager: manager,
	}
}

func (c *Collector) Start() {
	log = conf.Logger()
	ticker := time.NewTicker(c.PollPeriod)

	go func() {
		for {
			select {
			case result := <-c.Results:
				log.Info("Received result:", *result.VmPoolName)
				c.InProgress.Lock()
				c.InProgress.r[*result.VmPoolName] = false
				c.InProgress.Unlock()

			case <-ticker.C:
				// go func??? - if we kick off a go func here, it is possible that a "lot" of go funcs will wait here blocked
				// and it will cause a memory leak
				// but if not, it is possible that this routine will be blocked and ticker won't tick properly
				log.Info("ticker triggered:", time.Now())
				for _, vmPool := range c.VmPoolManager.CollectVmPools() {
					c.InProgress.Lock()
					if !c.InProgress.r[*vmPool] {
						c.InProgress.r[*vmPool] = true
						c.Requests <- VmPoolRequest{VmPoolName: vmPool}
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
}
