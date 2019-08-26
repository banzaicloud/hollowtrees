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

package promalert

import "github.com/pkg/errors"

type Config struct {
	// HTTP listen address
	ListenAddress string

	// JWT auth
	UseJWTAuth bool

	// JWT signing key
	JWTSigningKey string
}

// Validate checks that the configuration is valid.
func (c Config) Validate() error {
	if c.ListenAddress == "" {
		return errors.New("listen address must not be empty")
	}

	if c.UseJWTAuth && c.JWTSigningKey == "" {
		return errors.New("JWTSigningKey must be set if JWT auth is enabled")
	}

	return nil
}
