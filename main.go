package main

import (
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/banzaicloud/hollowtrees/api"
	"github.com/banzaicloud/hollowtrees/monitor"
)

var log *logrus.Logger

func main() {

	conf.Init()

	log = conf.Logger()
	log.Info("Logger configured.")

	monitor.Start()

	router := gin.Default()
	api.ConfigureRoutes(router)
	router.Run(":9090")

}
