package main

import (
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/monitor"
	"github.com/sirupsen/logrus"

	"github.com/banzaicloud/hollowtrees/api"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var log *logrus.Logger

func main() {

	//conf.Init()

	log = conf.Logger()
	log.Info("Logger configured.")

	monitor.Start(viper.GetString("dev.aws.region"))

	router := gin.Default()
	api.ConfigureRoutes(router)
	router.Run(":9090")

}
