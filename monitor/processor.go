package monitor

import (
	"github.com/sirupsen/logrus"
)

type PoolProcessor struct {
	ID             int
	Request        chan VmPoolRequest
	ProcessorQueue chan chan VmPoolRequest
	Results        chan VmPoolRequest
	QuitChan       chan bool
	VmPoolManager  VmPoolManager
}

func NewPoolProcessor(id int, processorQueue chan chan VmPoolRequest, results chan VmPoolRequest, manager VmPoolManager) PoolProcessor {
	return PoolProcessor{
		ID:             id,
		Request:        make(chan VmPoolRequest),
		ProcessorQueue: processorQueue,
		Results:        results,
		QuitChan:       make(chan bool),
		VmPoolManager:  manager}
}

func (p *PoolProcessor) Start() {
	go func() {
		for {
			p.ProcessorQueue <- p.Request
			select {
			case request := <-p.Request:
				log.WithFields(logrus.Fields{
					"processor": p.ID,
					"taskID":    request.VmPoolTask.TaskID,
					"vmPool":    *request.VmPoolTask.VmPoolName,
					"action":    *request.VmPoolTask.VmPoolAction,
				}).Info("Received request")

				if err := p.VmPoolManager.UpdateVmPool(request.VmPoolTask); err != nil {
					log.WithFields(logrus.Fields{
						"processor": p.ID,
						"taskID":    request.VmPoolTask.TaskID,
						"vmPool":    *request.VmPoolTask.VmPoolName,
						"action":    *request.VmPoolTask.VmPoolAction,
					}).Error("Failed to finish task on VM pool: ", err.Error())
				} else {
					log.WithFields(logrus.Fields{
						"processor": p.ID,
						"taskID":    request.VmPoolTask.TaskID,
						"vmPool":    *request.VmPoolTask.VmPoolName,
					}).Info("Task finished on VM pool successfully")
				}
				p.Results <- request

			case <-p.QuitChan:
				log.WithFields(logrus.Fields{
					"processor": p.ID,
				}).Info("PoolProcessor stopping")
				return
			}
		}
	}()
}

func (p *PoolProcessor) Stop() {
	go func() {
		p.QuitChan <- true
	}()
}
