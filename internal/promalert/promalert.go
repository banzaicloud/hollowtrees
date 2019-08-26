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

	"github.com/banzaicloud/hollowtrees/internal/platform/gin/correlationid"
	ginlog "github.com/banzaicloud/hollowtrees/internal/platform/gin/log"
	"github.com/banzaicloud/hollowtrees/internal/platform/log"
	"github.com/banzaicloud/hollowtrees/pkg/auth"
)

const (
	EventTopic   = "cloud.events.incoming"
	CETypePrefix = "prometheus.server.alert."
)

// PromAlertHandler describes a Prometheus alert handler
type PromAlertHandler struct {
	useJWTAuth    bool
	jwtSigningKey string
	listenAddress string

	logger       log.Logger
	errorHandler emperror.Handler
	eb           eventPublisher
}

// New returns an initialized PromAlertHandler
func New(config Config, logger log.Logger, errorHandler emperror.Handler, eb eventPublisher) *PromAlertHandler {
	return &PromAlertHandler{
		useJWTAuth:    config.UseJWTAuth,
		jwtSigningKey: config.JWTSigningKey,
		listenAddress: config.ListenAddress,

		logger:       logger,
		errorHandler: errorHandler,
		eb:           eb,
	}
}

// Run runs the alert handler HTTP listener
func (p *PromAlertHandler) Run() {
	p.logger.WithField("addr", p.listenAddress).WithField("useJWTAuth", p.useJWTAuth).Info("starting prometheus alert handler")

	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(correlationid.Middleware())
	r.Use(ginlog.Middleware(p.logger))
	if p.useJWTAuth {
		r.Use(auth.Handler(p.jwtSigningKey))
	}

	r.POST("/api/v1/alerts", p.handle)

	err := r.Run(p.listenAddress)
	if err != nil {
		p.errorHandler.Handle(err)
	}
}

// handle handles the incoming HTTP request
func (p *PromAlertHandler) handle(c *gin.Context) {
	var alerts Alerts

	log := correlationid.Logger(p.logger, c)

	if err := c.ShouldBindJSON(&alerts); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "failed to process alerts",
			"error":   err.Error(),
		})
		return
	}

	if err := alerts.Validate(); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"status":  http.StatusBadRequest,
			"message": "invalid alert",
			"error":   err.Error(),
		})
		return
	}

	if p.useJWTAuth {
		if err := alerts.Authorize(auth.GetCurrentUser(c)); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  http.StatusUnauthorized,
				"message": "could not process alerts",
				"error":   err.Error(),
			})
			return
		}
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
