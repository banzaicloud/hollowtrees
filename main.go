package main

import (
	"time"

	"github.com/banzaicloud/hollowtrees/api"
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/banzaicloud/hollowtrees/monitor"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var log *logrus.Entry

func init() {
	log = conf.Logger().WithField("package", "main")
	log.Info("Logger configured.")
}

func main() {

	region := viper.GetString("dev.aws.region")
	log.Info("Region to monitor: ", region)
	bufferSize := viper.GetInt("dev.monitor.bufferSize")
	log.Info("Buffer size for tasks: ", bufferSize)
	nrOfProcessors := viper.GetInt("dev.monitor.processors")
	log.Info("Number of processors to handle tasks: ", nrOfProcessors)
	monitorInterval := viper.GetDuration("dev.monitor.intervalInSeconds")
	log.Info("Monitor interval: ", monitorInterval)
	reevaluateInterval := viper.GetDuration("dev.monitor.reevaluateIntervalInSeconds")
	log.Info("Reevaluation interval: ", reevaluateInterval)

	monitor.Start(region, bufferSize, nrOfProcessors, monitorInterval*time.Second, reevaluateInterval*time.Second)
	log.Info("Started VM pool monitor")

	router := gin.Default()
	log.Info("Initialized gin router")
	api.ConfigureRoutes(router)
	log.Info("Configured routes")
	router.Run(":9090")

}
