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
	"github.com/banzaicloud/hollowtrees/internal/ce"
	"github.com/banzaicloud/hollowtrees/internal/platform/log"
)

type internalPlugin struct {
	BasePlugin
	logger log.Logger
}

// NewInternalPlugin returns an initialized internalPlugin
func NewInternalPlugin(name string, logger log.Logger) *internalPlugin {
	return &internalPlugin{
		BasePlugin: BasePlugin{
			name: name,
		},
		logger: logger,
	}
}

// Handle handles
func (p *internalPlugin) Handle(event *ce.Event) error {
	p.logger.Infof("internal-demo-plugin: %s", event.Type)

	return nil
}
