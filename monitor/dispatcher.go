package monitor

import (
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

type PoolRequest struct {
	asg autoscaling.Group
}

type Dispatcher struct {
	processors       int
	processorQueue   chan chan PoolRequest
	poolRequestQueue chan PoolRequest
	finishQueue      chan PoolRequest
}

func NewDispatcher(p int, queue chan PoolRequest, responseQueue chan PoolRequest) *Dispatcher {
	return &Dispatcher{
		processors:     p,
		processorQueue: make(chan chan PoolRequest, p),
		finishQueue:      responseQueue,
		poolRequestQueue: queue,
	}
}

func (d *Dispatcher) Start() {
	log = conf.Logger()

	for i := 0; i < d.processors; i++ {
		log.Info("Starting processor", i+1)
		processor := NewPoolProcessor(i+1, d.processorQueue, d.finishQueue)
		processor.Start()
	}

	go func() {
		for {
			select {
			case request := <-d.poolRequestQueue:
				log.Info("Received work request")
				go func() {
					worker := <-d.processorQueue
					log.Info("Dispatching work request to next available worker")
					worker <- request
				}()
			}
		}
	}()
}
