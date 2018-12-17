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

package log

import (
	"github.com/pkg/errors"
)

// Config holds details necessary for logging.
type Config struct {
	// Format specifies the output log format.
	// Accepted values are: json, logfmt
	Format string

	// Level is the minimum log level that should appear on the output.
	Level string

	// NoColor makes sure that no log output gets colorized.
	NoColor bool
}

// Validate validates the configuration.
func (c Config) Validate() error {
	if c.Format == "" {
		return errors.New("log format is required")
	}

	if c.Format != "json" && c.Format != "logfmt" {
		return errors.New("invalid log format: " + c.Format)
	}

	return nil
}
