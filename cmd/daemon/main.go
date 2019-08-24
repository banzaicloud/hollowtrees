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

package main

import (
	"fmt"
	"os"
	"sync"

	evbus "github.com/asaskevich/EventBus"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"

	"github.com/banzaicloud/hollowtrees/internal/flows"
	"github.com/banzaicloud/hollowtrees/internal/platform/config"
	"github.com/banzaicloud/hollowtrees/internal/platform/healthcheck"
	"github.com/banzaicloud/hollowtrees/internal/platform/log"
	"github.com/banzaicloud/hollowtrees/internal/plugin"
	"github.com/banzaicloud/hollowtrees/internal/promalert"
)

// nolint: gochecknoinits
func init() {
	pflag.Bool("version", false, "Show version information")
	pflag.Bool("dump-config", false, "Dump configuration to the console")
}

func main() {
	// Loads and validates configuration
	configure()

	// Show version if asked for
	if viper.GetBool("version") {
		fmt.Printf("%s version %s (%s) built on %s\n", config.FriendlyServiceName, version, commitHash, buildDate)
		os.Exit(0)
	}

	// Dump config if asked for
	if viper.GetBool("dump-config") {
		c := viper.AllSettings()
		y, err := yaml.Marshal(c)
		if err != nil {
			panic(errors.Wrap(err, "failed to dump configuration"))
		}
		fmt.Print(string(y))
		os.Exit(0)
	}

	// Create logger
	logger := log.NewLogger(configuration.Log)

	// Create error handler
	errorHandler := config.ErrorHandler(logger)

	// Create event bus
	eventBus := evbus.New()

	// Create plugin manager
	pluginManager := plugin.NewManager(logger, errorHandler)
	err := pluginManager.LoadFromConfig(viper.GetViper())
	if err != nil {
		errorHandler.Handle(err)
		os.Exit(2)
	}
	// Add internal demo plugin
	pluginManager.Add(plugin.NewInternalPlugin("internal-demo", logger))

	// Create flow manager
	flowManager := flows.NewManager(logger, errorHandler, flows.NewEventDispatcher(eventBus), pluginManager)
	err = flowManager.LoadFlows(viper.GetViper())
	if err != nil {
		errorHandler.Handle(err)
		os.Exit(2)
	}

	var wg sync.WaitGroup

	// Starts health check HTTP server
	wg.Add(1)
	go func() {
		healthcheck.New(configuration.Healthcheck, logger, errorHandler)
	}()

	// Starts prometheus alert manager
	wg.Add(1)
	go func() {
		promalert.New(configuration.Promalert, logger, errorHandler, promalert.NewEventDispatcher(eventBus)).Run()
	}()

	logger.Infof("%s started", config.FriendlyServiceName)

	wg.Wait()
}
