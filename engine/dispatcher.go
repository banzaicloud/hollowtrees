package engine

import (
	"context"

	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Dispatcher struct {
	Plugins  types.Plugins
	Rules    types.Rules
	Requests chan action.AlertEvent
}

func NewDispatcher(plugins types.Plugins, rules types.Rules, requests chan action.AlertEvent) *Dispatcher {
	return &Dispatcher{
		Plugins:  plugins,
		Rules:    rules,
		Requests: requests,
	}
}

var log *logrus.Entry

func (d *Dispatcher) Start() {
	log = conf.Logger().WithField("package", "engine")
	go func() {
		d.ValidateRules()
		log.Infof("Plugins configured: %v", d.Plugins)
		log.Infof("Rules configured: %v", d.Rules)
		for {
			select {
			case event := <-d.Requests:
				log.WithField("eventId", event.EventId).Infof("Dispatcher received event")
				go func() {
					addresses := d.SelectPlugins(event)
					for _, a := range addresses {
						// send only if all plugins are available?
						log.WithField("eventId", event.EventId).Infof("Sending event to plugin address: %s", a)
						conn, err := grpc.Dial(a, grpc.WithInsecure())
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
				}()
			}
		}
	}()
}

// TODO: unit test
func (d *Dispatcher) ValidateRules() {
	for _, rule := range d.Rules {
		for _, plugin := range rule.Plugins {
			if !d.containsPlugin(plugin) {
				log.Fatalf("Invalid plugin ('%s') configured in rule '%s'", plugin, rule.Name)
			}
		}
	}
}

func (d *Dispatcher) containsPlugin(name string) bool {
	for _, p := range d.Plugins {
		if p.Name == name {
			return true
		}
	}
	return false
}

// TODO: unit test
func (d *Dispatcher) SelectPlugins(event action.AlertEvent) []string {
	var addresses []string
	for _, r := range d.Rules {
		if r.EventType == event.EventType {
			matchesAll := true
			for matchKey, matchValue := range r.Match {
				if dataValue, ok := event.Data[matchKey]; !ok || dataValue != matchValue {
					matchesAll = false
				}
			}
			if matchesAll {
				log.WithField("eventId", event.EventId).Infof("Matching rule found for event: %s", r.Name)
				for _, pn := range r.Plugins {
					for _, p := range d.Plugins {
						if p.Name == pn {
							addresses = append(addresses, p.Address)
						}
					}
				}
			}
		}
	}
	return addresses
}
