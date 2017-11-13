package main

import (
	"github.com/banzaicloud/hollowtrees/conf"
	"github.com/sirupsen/logrus"

	"github.com/banzaicloud/hollowtrees/recommender"
	"github.com/gin-gonic/gin"
)

var log *logrus.Logger

func main() {

	conf.Init()

	log = conf.Logger()
	log.Info("Logger configured.")

	router := gin.Default()
	v1 := router.Group("/api/v1/")
	{
		v1.GET("/recommender/:region", recommender.RecommendSpotInstanceTypes)
	}
	router.Run(":9090")
}
