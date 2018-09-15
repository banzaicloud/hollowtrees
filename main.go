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
	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/api"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/engine"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var log *logrus.Entry

func main() {

	conf.Init()
	log = conf.Logger().WithField("package", "main")
	log.Info("Logger configured.")

	bufferSize := viper.GetInt("global.bufferSize")
	log.Info("Buffer size for tasks: ", bufferSize)

	poolRequestChan := make(chan action.AlertEvent, bufferSize)

	pluginConfigs := conf.ReadPlugins()
	plugins := make(engine.Plugins, len(pluginConfigs))
	for i, p := range pluginConfigs {
		plugins[i] = engine.NewPlugin(p)
	}

	engine.NewDispatcher(plugins, conf.ReadActionFlows(), poolRequestChan).Start()
	collector := engine.NewCollector(poolRequestChan)

	apiEngine := gin.Default()
	log.Info("Initialized gin router")
	api.ConfigureRoutes(apiEngine, api.NewRouter(collector))
	log.Info("Configured routes")

	bindAddr := viper.GetString("global.bindAddr")
	log.Infof("Starting API on %s", bindAddr)
	apiEngine.Run(bindAddr)

}
