// Copyright © 2018 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import (
	"path"
	"sync"
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
	FlowStore         FlowStore
	mux               sync.Mutex
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
		FlowStore:         NewInMemFlowStore(),
	}
}

var log *logrus.Entry

type ActionFlowState struct {
	eventId string
	status  string
	tries   int
}

func (d *Dispatcher) Start() {
	log = conf.Logger().WithField("package", "engine")
	go func() {
		log.Infof("Plugins configured: %v", d.Plugins)
		log.Infof("ActionFlows configured: %v", d.ActionFlows)
		for {
			select {
			case event := <-d.Requests:
				log.WithField("eventId", event.EventId).Debugf("Dispatcher received event: %#v", event)
				flow := d.SelectActionFlow(event)
				if flow == nil {
					log.Infof("no matching action flow found for event %s", event.EventId)
					continue
				}
				if len(flow.GroupBy) > 0 { // if there's no grouping, every alert is processed without storing them
					key := getEventKey(event, flow.GroupBy)
					log.WithField("eventId", event.EventId).Debugf("cache key is %s", key)
					d.mux.Lock()
					attempts := 1
					afs, err := d.FlowStore.getState(key)
					if err != nil {
						log.WithField("eventId", afs.eventId).Errorf("couldn't get flow state, event won't be processed")
						continue
					}
					if afs != nil {
						if afs.status != "failed" || afs.tries >= flow.Retries {
							// won't process event, it's thrown away
							log.WithField("eventId", afs.eventId).Debugf("%s is already processed", key)
							d.mux.Unlock()
							continue
						}
						attempts = afs.tries + 1
					}
					afs = &ActionFlowState{
						eventId: event.EventId,
						status:  "in-progress",
						tries:   attempts,
					}
					if err := d.FlowStore.setState(key, afs, flow.RepeatCooldown); err != nil {
						log.WithField("eventId", afs.eventId).Errorf("couldn't store flow state, event won't be processed")
						continue
					}
					log.WithField("eventId", event.EventId).Debugf("put %s=%v in cache", key, *afs)
					d.mux.Unlock()
				}
				sem := d.ConcurrencyLimits[event.EventType]
				go func(flow *types.ActionFlow, event action.AlertEvent, sem chan bool) {
					sem <- true
					s := "success"
					err := d.executeActionFlow(flow, event)
					if err != nil {
						log.WithField("eventId", event.EventId).Errorf("failed to execute action flow: %v", err)
						s = "failed"
					}
					if len(flow.GroupBy) > 0 {
						key := getEventKey(event, flow.GroupBy)
						afs, err := d.FlowStore.getState(key)
						if err != nil {
							log.WithField("eventId", afs.eventId).Errorf("couldn't store flow state")
						} else if afs != nil {
							afs.status = s
							if err := d.FlowStore.setState(key, afs, flow.RepeatCooldown); err != nil {
								log.WithField("eventId", afs.eventId).Errorf("couldn't store flow state")
							}
						} else {
							log.WithField("eventId", event.EventId).Errorf("couldn't find flow in cache: %v", err)
						}
					}
					<-sem
				}(flow, event, sem)
			}
		}
	}()
}

func getEventKey(event action.AlertEvent, groupBy []string) string {
	key := event.EventType
	for _, g := range groupBy {
		key = path.Join(key, event.Data[g])
	}
	return key
}
func (d *Dispatcher) executeActionFlow(flow *types.ActionFlow, event action.AlertEvent) error {
	var plugins []Plugin
	for _, pn := range flow.Plugins {
		for _, p := range d.Plugins {
			if p.name() == pn {
				log.WithField("eventId", event.EventId).Debugf("plugin selected for event: %s", pn)
				plugins = append(plugins, p)
			}
		}
	}
	for _, p := range plugins {
		log.WithField("eventId", event.EventId).Infof("Sending event to plugin: %#v", p.name())
		err := p.exec(event)
		if err != nil {
			log.WithField("eventId", event.EventId).Errorf("failed to execute plugin %s", p.name())
			return err
		}
	}
	if flow.Cooldown > 0 {
		log.Infof("Starting cooldown: %v", flow.Cooldown)
		timer := time.NewTimer(flow.Cooldown)
		<-timer.C
		log.Infof("Cooldown finished")
	}
	return nil
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
				log.WithField("eventId", event.EventId).Debugf("Matching action flow found for event: %s", af.Name)
				return af
			}
		}
	}
	return nil
}
