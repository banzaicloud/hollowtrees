package engine

import (
	"context"

	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/conf"
	"google.golang.org/grpc"
)

type Plugin interface {
	name() string
	exec(event action.AlertEvent)
}

func NewPlugin(pc conf.PluginConfig) Plugin {
	switch pc.Type {
	case "grpc":
		return &GrpcPlugin{
			PluginBase{
				Name:    pc.Name,
				Address: pc.Address,
			},
		}
	case "fn":
		return &FnPlugin{
			PluginBase: PluginBase{
				Name:    pc.Name,
				Address: pc.Address,
			},
			App:      pc.Properties["app"],
			Function: pc.Properties["function"],
		}
	default:
		return nil
	}
}

type Plugins []Plugin

type PluginBase struct {
	Name    string
	Address string
}

func (p *PluginBase) name() string {
	return p.Name
}

type GrpcPlugin struct {
	PluginBase
}

func (p *GrpcPlugin) exec(event action.AlertEvent) {
	conn, err := grpc.Dial(p.Address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("couldn't create GRPC channel to action server: %v", err)
	}
	client := action.NewActionClient(conn)
	_, err = client.HandleAlert(context.Background(), &event)
	if err != nil {
		log.WithField("eventId", event.EventId).Errorf("Failed to handle alert: %v", err)
	}
	conn.Close()
}

type FnPlugin struct {
	PluginBase
	App      string
	Function string
}

func (p *FnPlugin) exec(event action.AlertEvent) {
	// TODO
}
