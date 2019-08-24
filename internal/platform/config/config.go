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

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/goph/emperror"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/banzaicloud/hollowtrees/internal/platform/healthcheck"
	"github.com/banzaicloud/hollowtrees/internal/platform/log"
	"github.com/banzaicloud/hollowtrees/internal/promalert"
)

const (
	// ServiceName is an identifier-like name used anywhere this app needs to be identified.
	ServiceName = "hollowtrees"

	// FriendlyServiceName is the visible name of the service.
	FriendlyServiceName = "Hollowtrees"

	// ConfigEnvPrefix defines the prefix that ENVIRONMENT variables will use
	ConfigEnvPrefix = "HT"
)

// Config holds any kind of configuration that comes from the outside world and
// is necessary for running the application
type Config struct {
	// Meaningful values are recommended (eg. production, development, staging, release/123, etc)
	Environment string

	// Turns on some debug functionality (eg. more verbose logs)
	Debug bool

	// Log configuration
	Log log.Config

	// Healthcheck configuration
	Healthcheck healthcheck.Config

	// Prometheus alert handler configuration
	Promalert promalert.Config
}

// Validate validates the configuration
func (c Config) Validate() error {
	err := c.Log.Validate()
	if err != nil {
		return emperror.Wrap(err, "could not validate log config")
	}

	err = c.Promalert.Validate()
	if err != nil {
		return emperror.Wrap(err, "could not validate promalert config")
	}

	err = c.Healthcheck.Validate()
	if err != nil {
		return emperror.Wrap(err, "could not validate healthcheck config")
	}

	return nil
}

// Configure configures some defaults in the Viper instance
func Configure(v *viper.Viper, p *pflag.FlagSet) {
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/config")
	p.Init(FriendlyServiceName, pflag.ExitOnError)
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", FriendlyServiceName)
		pflag.PrintDefaults()
	}
	v.BindPFlags(p) // nolint:errcheck

	v.SetEnvPrefix(ConfigEnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Log configuration
	v.SetDefault("log.format", "logfmt")
	v.SetDefault("log.level", "info")

	// Healthcheck HTTP endpoint
	v.SetDefault("healthcheck.listenAddress", ":8082")
	v.SetDefault("healthcheck.endpoint", "/healthz")

	// Prometheus alert handler
	v.SetDefault("promalert.listenAddress", ":8081")
	v.SetDefault("promalert.useJWTAuth", false)
	v.SetDefault("promalert.jwtSigningKey", "")
}
