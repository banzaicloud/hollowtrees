package api

import (
	"fmt"
	"net/http"

	"github.com/banzaicloud/hollowtrees/engine"
	"github.com/banzaicloud/hollowtrees/engine/types"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/go-playground/validator.v8"
)

var log = logrus.New().WithField("package", "api")

type Router struct {
	Collector *engine.Collector
}

func NewRouter(collector *engine.Collector) *Router {
	return &Router{
		Collector: collector,
	}
}

func ConfigureRoutes(engine *gin.Engine, router *Router) {
	log.Info("configuring routes")
	v1 := engine.Group("/api/v1/")
	{
		v1.POST("/alerts", router.handleAlert)
	}
}

func (r *Router) handleAlert(c *gin.Context) {
	alerts := make([]types.Alert, 1)
	if err := c.BindJSON(&alerts); err != nil {
		if ve, ok := err.(validator.ValidationErrors); !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": "Failed to process alert", "error": ve.Error()})
			fmt.Println(err)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "message": "Validation failed", "error": ve.Error()})
		}
		return
	}

	log.Infof("Received alerts: %#v", alerts)
	r.Collector.Collect(alerts)
	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "data": "ok"})
}
