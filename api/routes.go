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
	base := engine.Group("/")
	{
		base.GET("/health", router.health)
	}
	v1 := engine.Group("/api/v1/")
	{
		v1.POST("/alerts", router.handleAlert)
	}
}

func (r *Router) health(c *gin.Context) {
	c.JSON(http.StatusOK, "ok")
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

	log.Debugf("Received alerts: %#v", alerts)
	r.Collector.Collect(alerts)
	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "data": "ok"})
}
