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

package flows

import (
	"github.com/goph/emperror"
	"github.com/spf13/viper"

	"github.com/banzaicloud/hollowtrees/internal/platform/log"
	"github.com/banzaicloud/hollowtrees/internal/plugin"
)

// FlowManager is used for managing action flows
type FlowManager interface {
	Logger() log.Logger
	ErrorHandler() emperror.Handler
	Plugins() plugin.PluginManager
}

// Manager describes a FlowManager implementation
type Manager struct {
	logger       log.Logger
	errorHandler emperror.Handler
	dispatcher   eventSubscriber
	plugins      plugin.PluginManager
}

// NewManager returns an initialized FlowManager implementation
func NewManager(logger log.Logger, errorHandler emperror.Handler, dispatcher flowEventDispatcher, plugins plugin.PluginManager) *Manager {
	return &Manager{
		logger:       logger,
		errorHandler: errorHandler,
		dispatcher:   dispatcher,
		plugins:      plugins,
	}
}

// Logger returns the logger
func (m *Manager) Logger() log.Logger {
	return m.logger
}

// ErrorHandler returns the error handler
func (m *Manager) ErrorHandler() emperror.Handler {
	return m.errorHandler
}

// Plugins returns the plugin manager
func (m *Manager) Plugins() plugin.PluginManager {
	return m.plugins
}

// LoadFlows loads flow definitions from config,  initializes Flows
// and subscribes them to the event dispatcher
func (m *Manager) LoadFlows(v *viper.Viper) error {
	var flows FlowConfigs

	err := viper.UnmarshalKey("flows", &flows)
	if err != nil {
		return emperror.Wrap(err, "could not unmarshal flow configs")
	}

	for id, config := range flows {
		err := config.Validate(m.plugins, id)
		if err != nil {
			return emperror.WrapWith(err, "could not load flow", "flow", id)
		}

		f := NewFlow(m, NewInMemFlowStore(), id, config.Name,
			Description(config.Description),
			AllowedEvents(config.AllowedEvents),
			Cooldown(config.Cooldown),
			GroupBy(config.GroupBy),
			Plugins(config.Plugins),
			Filters(config.Filters),
		)

		err = m.dispatcher.SubscribeAsync(CEIncomingTopic, f)
		if err != nil {
			m.errorHandler.Handle(emperror.WrapWith(err, "could not subscribe to event dispatcher", "flow", id))
		}
	}

	return nil
}
