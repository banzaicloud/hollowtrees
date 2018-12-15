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

package promalert

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/goph/emperror"
	validator "gopkg.in/go-playground/validator.v8"

	"github.com/banzaicloud/hollowtrees/internal/platform/gin/correlationid"
	ginlog "github.com/banzaicloud/hollowtrees/internal/platform/gin/log"
	"github.com/banzaicloud/hollowtrees/internal/platform/log"
)

const (
	EventTopic   = "cloud.events.incoming"
	CETypePrefix = "prometheus.server.alert."
)

// PromAlertHandler describes a Prometheus alert handler
type PromAlertHandler struct {
	logger       log.Logger
	errorHandler emperror.Handler
	eb           eventPublisher
}

// New returns an initialized PromAlertHandler
func New(logger log.Logger, errorHandler emperror.Handler, eb eventPublisher) *PromAlertHandler {
	return &PromAlertHandler{
		logger:       logger,
		errorHandler: errorHandler,
		eb:           eb,
	}
}

// Run runs the alert handler HTTP listener
func (p *PromAlertHandler) Run(addr string) {
	p.logger.WithField("addr", addr).Info("starting prometheus alert handler")

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(correlationid.Middleware())
	r.Use(ginlog.Middleware(p.logger))

	r.POST("/api/v1/alerts", p.handle)

	err := r.Run(addr)
	if err != nil {
		p.errorHandler.Handle(err)
	}
}

// handle handles the incoming HTTP request
func (p *PromAlertHandler) handle(c *gin.Context) {
	var alerts []Alert

	log := correlationid.Logger(p.logger, c)

	if err := c.BindJSON(&alerts); err != nil {
		if ve, ok := err.(validator.ValidationErrors); !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  http.StatusInternalServerError,
				"message": "Failed to process alert",
				"error":   ve.Error(),
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  http.StatusBadRequest,
				"message": "Validation failed",
				"error":   ve.Error(),
			})
		}
		return
	}

	log.WithField("alert-count", len(alerts)).Debug("alerts received")

	cid := c.GetString(correlationid.ContextKey)
	p.publishAlerts(alerts, cid)

	c.JSON(http.StatusOK, gin.H{
		"status": http.StatusOK,
		"data":   "ok",
	})
}

// publishAlerts publishing incoming alerts through the event dispatcher
func (p *PromAlertHandler) publishAlerts(alerts []Alert, cid string) {
	for _, alert := range alerts {
		event, err := alert.convertToCE(cid)
		if err != nil {
			p.errorHandler.Handle(err)
			continue
		}
		p.eb.Publish(EventTopic, event)
	}
}
