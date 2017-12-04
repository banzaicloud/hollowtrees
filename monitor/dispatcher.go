package monitor

import "github.com/sirupsen/logrus"

type Dispatcher struct {
	NrProcessors   int
	ProcessorQueue chan chan VmPoolRequest
	Requests       chan VmPoolRequest
	Results        chan VmPoolRequest
	VmPoolManager  VmPoolManager
}

func NewDispatcher(p int, requests chan VmPoolRequest, results chan VmPoolRequest, manager VmPoolManager) *Dispatcher {
	return &Dispatcher{
		NrProcessors:   p,
		ProcessorQueue: make(chan chan VmPoolRequest, p),
		Results:        results,
		Requests:       requests,
		VmPoolManager:  manager,
	}
}

func (d *Dispatcher) Start() {
	for i := 0; i < d.NrProcessors; i++ {
		log.Info("Starting processor ", i+1)
		processor := NewPoolProcessor(i+1, d.ProcessorQueue, d.Results, d.VmPoolManager)
		processor.Start()
	}

	go func() {
		for {
			select {
			case request := <-d.Requests:
				log.WithFields(logrus.Fields{
					"autoScalingGroup": *request.VmPoolTask.VmPoolName,
					"taskID":           request.VmPoolTask.TaskID,
					"action":           *request.VmPoolTask.VmPoolAction,
				}).Info("Received work request")
				go func() {
					worker := <-d.ProcessorQueue
					log.WithFields(logrus.Fields{
						"autoScalingGroup": *request.VmPoolTask.VmPoolName,
						"taskID":           request.VmPoolTask.TaskID,
						"action":           *request.VmPoolTask.VmPoolAction,
					}).Info("Dispatching work request to next available worker")
					worker <- request
				}()
			}
		}
	}()
}
