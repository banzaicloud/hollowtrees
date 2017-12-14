package monitor

import (
	"context"

	"github.com/banzaicloud/hollowtrees/action"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Dispatcher struct {
	PluginAddress string
	Requests      chan VmPoolRequest
	Results       chan VmPoolRequest
	VmPoolManager VmPoolManager
}

func NewDispatcher(pluginAddress string, requests chan VmPoolRequest, results chan VmPoolRequest, manager VmPoolManager) *Dispatcher {
	return &Dispatcher{
		PluginAddress: pluginAddress,
		Results:       results,
		Requests:      requests,
		VmPoolManager: manager,
	}
}

func (d *Dispatcher) Start() {
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
					conn, err := grpc.Dial(d.PluginAddress, grpc.WithInsecure())
					if err != nil {
						log.Fatalf("couldn't create GRPC channel to action server: %v", err)
					}
					defer conn.Close()
					client := action.NewActionClient(conn)
					result, err := client.HandleAlert(context.Background(), &action.AlertEvent{
						AlertName: "spot-termination-notice",
					})
					log.Info(result.GetStatus)
					if err != nil {
						log.Errorf("Failed to handle action: %v", err)
					}
				}()
			}
		}
	}()
}
