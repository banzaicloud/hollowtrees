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
	"github.com/goph/emperror"
	"github.com/pkg/errors"
)

// PluginConfig describes a plugin configuration
type PluginConfig struct {
	Name    string `mapstructure:"name"`
	Type    string `mapstructure:"type"`
	Address string `mapstructure:"address"`
}

type PluginConfigs []PluginConfig

// Validate validates plugin configuration
func (c PluginConfig) Validate() error {
	if c.Name == "" {
		return errors.New("name must be set")
	}

	if c.Type != "grpc" {
		return emperror.With(errors.New("invalid plugin type"), "type", c.Type)
	}

	if c.Type == "grpc" && c.Address == "" {
		return errors.New("address must not be empty for a GRPC plugin")
	}

	return nil
}
