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
	"time"

	"github.com/goph/emperror"
	"github.com/pkg/errors"

	"github.com/banzaicloud/hollowtrees/internal/plugin"
)

// FlowConfig holds configuration values for an action flow
type FlowConfig struct {
	Name    string   `mapstructure:"name"`
	Plugins []string `mapstructure:"plugins"`

	Description   string            `mapstructure:"description"`
	AllowedEvents []string          `mapstructure:"allowedEvents"`
	GroupBy       []string          `mapstructure:"groupBy"`
	Filters       map[string]string `mapstructure:"filters"`
	Cooldown      time.Duration     `mapstructure:"cooldown"`
}

type FlowConfigs map[string]FlowConfig

// Validate validates flow configuration
func (c FlowConfig) Validate(plugins plugin.PluginManager, id string) error {
	if c.Name == "" {
		return errors.New("name must be set")
	}

	if len(c.Plugins) == 0 {
		return emperror.WrapWith(errors.New("no plugins defined"), "invalid flow config", "flow", id)
	}

	_, err := plugins.GetByNames(c.Plugins...)
	if err != nil {
		return emperror.WrapWith(err, "invalid flow", "flow", id)
	}

	return nil
}
