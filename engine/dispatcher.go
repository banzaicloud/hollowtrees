package engine

import (
	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/sirupsen/logrus"
)

type Dispatcher struct {
	Plugins  Plugins
	Rules    types.Rules
	Requests chan action.AlertEvent
}

func NewDispatcher(plugins Plugins, rules types.Rules, requests chan action.AlertEvent) *Dispatcher {
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
				log.WithField("eventId", event.EventId).Infof("Dispatcher received event: %#v", event)
				go func() {
					plugins := d.SelectPlugins(event)
					log.WithField("eventId", event.EventId).Debugf("plugins selected for event: %#v", plugins)
					for _, p := range plugins {
						log.WithField("eventId", event.EventId).Infof("Sending event to plugin: %#v", p)
						err := p.exec(event)
						if err != nil {
							log.WithField("eventId", event.EventId).Errorf("failed to execute plugin %s for event: %v", p.name(), err)
						}
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
		if p.name() == name {
			return true
		}
	}
	return false
}

// TODO: unit test
func (d *Dispatcher) SelectPlugins(event action.AlertEvent) (plugins Plugins) {
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
						if p.name() == pn {
							plugins = append(plugins, p)
						}
					}
				}
			}
		}
	}
	return
}
