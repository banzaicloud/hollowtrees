package engine

import (
	"time"

	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/sirupsen/logrus"
)

type Dispatcher struct {
	Plugins           Plugins
	ActionFlows       types.ActionFlows
	Requests          chan action.AlertEvent
	ConcurrencyLimits map[string]chan bool
}

func NewDispatcher(plugins Plugins, actionFlows types.ActionFlows, requests chan action.AlertEvent) *Dispatcher {
	validateFlows(actionFlows, plugins)
	cl := make(map[string]chan bool, len(actionFlows))
	for _, af := range actionFlows {
		cl[af.EventType] = make(chan bool, af.ConcurrentFlows)
	}
	return &Dispatcher{
		Plugins:           plugins,
		ActionFlows:       actionFlows,
		Requests:          requests,
		ConcurrencyLimits: cl,
	}
}

var log *logrus.Entry

func (d *Dispatcher) Start() {
	log = conf.Logger().WithField("package", "engine")
	go func() {
		log.Infof("Plugins configured: %v", d.Plugins)
		log.Infof("ActionFlows configured: %v", d.ActionFlows)
		for {
			select {
			case event := <-d.Requests:
				log.WithField("eventId", event.EventId).Infof("Dispatcher received event: %#v", event)
				flow := d.SelectActionFlow(event)
				if flow == nil {
					log.Infof("no matching action flow found for event %s", event.EventId)
					continue
				}
				sem := d.ConcurrencyLimits[event.EventType]
				go func(flow *types.ActionFlow, sem chan bool) {
					sem <- true
					d.executeActionFlow(flow, event)
					<-sem
				}(flow, sem)
			}
		}
	}()
}
func (d *Dispatcher) executeActionFlow(flow *types.ActionFlow, event action.AlertEvent) {
	var plugins []Plugin
	for _, pn := range flow.Plugins {
		for _, p := range d.Plugins {
			if p.name() == pn {
				plugins = append(plugins, p)
			}
		}
	}
	log.WithField("eventId", event.EventId).Debugf("plugins selected for event: %#v", plugins)
	for _, p := range plugins {
		log.WithField("eventId", event.EventId).Infof("Sending event to plugin: %#v", p)
		err := p.exec(event)
		if err != nil {
			log.WithField("eventId", event.EventId).Errorf("failed to execute plugin %s for event: %v", p.name(), err)
			return
		}
	}
	if flow.Cooldown > 0 {
		log.Infof("Starting cooldown: %v", flow.Cooldown)
		timer := time.NewTimer(flow.Cooldown)
		<-timer.C
		log.Infof("Cooldown finished")
	}
}

func validateFlows(actionFlows types.ActionFlows, plugins Plugins) {
	for _, af := range actionFlows {
		for _, plugin := range af.Plugins {
			if !containsPlugin(plugins, plugin) {
				log.Fatalf("Invalid plugin ('%s') configured in action flow '%s'", plugin, af.Name)
			}
		}
	}
}

func containsPlugin(plugins Plugins, name string) bool {
	for _, p := range plugins {
		if p.name() == name {
			return true
		}
	}
	return false
}

func (d *Dispatcher) SelectActionFlow(event action.AlertEvent) *types.ActionFlow {
	for _, af := range d.ActionFlows {
		if af.EventType == event.EventType {
			matchesAll := true
			for matchKey, matchValue := range af.Match {
				if dataValue, ok := event.Data[matchKey]; !ok || dataValue != matchValue {
					matchesAll = false
				}
			}
			if matchesAll {
				log.WithField("eventId", event.EventId).Infof("Matching action flow found for event: %s", af.Name)
				return af
			}
		}
	}
	return nil
}
