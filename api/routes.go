package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var log = logrus.New().WithField("package", "api")

func ConfigureRoutes(router *gin.Engine) {
	log.Info("configuring routes")
	v1 := router.Group("/api/v1/")
	{
		v1.POST("/alerts", handleAlert)
	}
}

func handleAlert(c *gin.Context) {
	log.Info("handling alert")
	rawData, _ := c.GetRawData()
	log.Info(string(rawData))
	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "data": "ok"})
}
