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

package plugin

import (
	"errors"

	"github.com/goph/emperror"
	"github.com/spf13/viper"

	"github.com/banzaicloud/hollowtrees/internal/platform/log"
)

// PluginManager describes what a plugin manager implementation must provide
type PluginManager interface {
	Add(plugin ...EventHandlerPlugin)
	GetByNames(names ...string) (map[string]EventHandlerPlugin, error)
	GetByName(name string) (EventHandlerPlugin, error)
}

// Manager is a PluginManager implementation
type Manager struct {
	logger       log.Logger
	errorHandler emperror.Handler

	plugins map[string]EventHandlerPlugin
}

// NewManager returns an initialized Manager
func NewManager(logger log.Logger, errorHandler emperror.Handler) *Manager {
	m := &Manager{
		logger:       logger,
		errorHandler: errorHandler,
	}

	plugins := make(map[string]EventHandlerPlugin)
	m.plugins = plugins

	return m
}

// Add add an initialized plugin
func (m *Manager) Add(plugins ...EventHandlerPlugin) {
	for _, plugin := range plugins {
		m.plugins[plugin.GetName()] = plugin
	}
}

// GetByNames returns a map of plugins by their names
func (m *Manager) GetByNames(names ...string) (map[string]EventHandlerPlugin, error) {
	plugins := make(map[string]EventHandlerPlugin)

	for _, name := range names {
		p, err := m.GetByName(name)
		if err != nil {
			return nil, err
		}
		plugins[p.GetName()] = p
	}

	return plugins, nil
}

// GetByName returns a plugin by it's name
func (m *Manager) GetByName(name string) (EventHandlerPlugin, error) {
	p := m.plugins[name]
	if p == nil {
		return nil, emperror.With(errors.New("plugin not found"), "name", name)
	}

	return p, nil
}

// LoadFromConfig loads plugins from configuration
func (m *Manager) LoadFromConfig(v *viper.Viper) error {
	var plugins PluginConfigs

	err := viper.UnmarshalKey("plugins", &plugins)
	if err != nil {
		return emperror.Wrap(err, "could not unmarshal plugin configs")
	}

	if len(plugins) == 0 {
		return emperror.Wrap(err, "no plugins were defined")
	}

	for _, plugin := range plugins {
		err := plugin.Validate()
		if err != nil {
			return emperror.WrapWith(err, "invalid plugin configuration", "plugin", plugin.Name)
		}
		switch plugin.Type {
		case "grpc":
			m.Add(NewGrpcPlugin(plugin.Name, plugin.Address))
		}
	}

	return nil
}
