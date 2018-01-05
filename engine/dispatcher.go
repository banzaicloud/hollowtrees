package engine

import (
	"context"

	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Dispatcher struct {
	PluginAddress string
	Requests      chan action.AlertEvent
}

func NewDispatcher(pluginAddress string, requests chan action.AlertEvent) *Dispatcher {
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
				log.Info("Received alerts request")
				go func() {
					conn, err := grpc.Dial(d.PluginAddress, grpc.WithInsecure())
					if err != nil {
						log.Fatalf("couldn't create GRPC channel to action server: %v", err)
					}
					defer conn.Close()
					client := action.NewActionClient(conn)
					result, err := client.HandleAlert(context.Background(), &request)
					log.Infof("status: %s", result.GetStatus())
					if err != nil {
						log.Errorf("Failed to handle action: %v", err)
					}
				}()
			}
		}
	}()
}
