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

	engine.NewDispatcher(conf.ReadPlugins(), conf.ReadRules(), poolRequestChan).Start()
	collector := engine.NewCollector(poolRequestChan)

	apiEngine := gin.Default()
	log.Info("Initialized gin router")
	api.ConfigureRoutes(apiEngine, api.NewRouter(collector))
	log.Info("Configured routes")
	apiEngine.Run(":9091")

}
