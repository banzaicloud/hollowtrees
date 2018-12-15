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

import "time"

// Option sets configuration on the Flow
type Option interface {
	apply(*Flow)
}

// Cooldown time that passes after an event flow is successfully finished
type Cooldown time.Duration

func (o Cooldown) apply(f *Flow) {
	f.cooldown = time.Duration(o)
}

// AllowedEvents defines allowed event types for the flow
type AllowedEvents []string

func (o AllowedEvents) apply(f *Flow) {
	f.allowedEvents = []string(o)
}

// GroupBy categorizes subsequent events as the same if all the corresponding values of these attributes match
type GroupBy []string

func (o GroupBy) apply(f *Flow) {
	f.groupBy = []string(o)
}

// Plugins defines the plugins to execute in an event flow
type Plugins []string

func (o Plugins) apply(f *Flow) {
	f.plugins = []string(o)
}

// Filters defines simple filter on event values
type Filters map[string]string

func (o Filters) apply(f *Flow) {
	f.filters = map[string]string(o)
}

// Description sets the description of the action flow
type Description string

func (o Description) apply(f *Flow) {
	f.description = string(o)
}
