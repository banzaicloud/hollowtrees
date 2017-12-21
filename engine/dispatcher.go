package engine

import (
	"context"

	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Dispatcher struct {
	PluginAddress string
	Requests      chan types.AlertRequest
}

func NewDispatcher(pluginAddress string, requests chan types.AlertRequest) *Dispatcher {
	return &Dispatcher{
		PluginAddress: pluginAddress,
		Requests:      requests,
	}
}

var log *logrus.Entry

func (d *Dispatcher) Start() {
	log = conf.Logger().WithField("package", "engine")
	go func() {
		for {
			select {
			case request := <-d.Requests:
				log.WithFields(logrus.Fields{
					"alertGroupKey": request.AlertInfo.GroupKey,
				}).Info("Received alert request")
				go func() {
					conn, err := grpc.Dial(d.PluginAddress, grpc.WithInsecure())
					if err != nil {
						log.Fatalf("couldn't create GRPC channel to action server: %v", err)
					}
					defer conn.Close()
					client := action.NewActionClient(conn)
					// TODO: convert alertinfo to events
					result, err := client.HandleAlert(context.Background(), &action.AlertEvent{
						EventId:   uuid.NewV4().String(),
						EventType: request.AlertInfo.Alerts[0].Labels["alertname"],
						Resource: &action.Resource{
							ResourceType: "aws-asg-exporter", //TODO: plugin.name
							ResourceId:   request.AlertInfo.Alerts[0].Labels["instance"],
						},
					})
					log.Infof("status: %s", result.GetStatus())
					if err != nil {
						log.Errorf("Failed to handle action: %v", err)
					}
				}()
			}
		}
	}()
}
