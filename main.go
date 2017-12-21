package main

import (
	"github.com/banzaicloud/hollowtrees/api"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/engine"
	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var log *logrus.Entry

func main() {

	conf.Init()
	log = conf.Logger().WithField("package", "main")
	log.Info("Logger configured.")

	bufferSize := viper.GetInt("dev.engine.bufferSize")
	log.Info("Buffer size for tasks: ", bufferSize)
	pluginAddress := viper.GetString("dev.plugin.address")
	log.Info("Address of action plugin: ", pluginAddress)

	poolRequestChan := make(chan types.AlertRequest, bufferSize)
	engine.NewDispatcher(pluginAddress, poolRequestChan).Start()
	collector := engine.NewCollector(poolRequestChan)

	apiEngine := gin.Default()
	log.Info("Initialized gin router")
	api.ConfigureRoutes(apiEngine, api.NewRouter(collector))
	log.Info("Configured routes")
	apiEngine.Run(":9091")

}
