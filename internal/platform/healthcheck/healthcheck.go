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

package healthcheck

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/goph/emperror"

	"github.com/banzaicloud/hollowtrees/internal/platform/log"
)

// New runs the health check endpoint
func New(config Config, logger log.Logger, errorHandler emperror.Handler) {
	logger.WithFields(log.Fields{"addr": config.ListenAddress, "endpoint": config.Endpoint}).Info("starting health check http server")

	r := gin.New()
	r.GET(config.Endpoint, func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	err := r.Run(config.ListenAddress)
	if err != nil {
		errorHandler.Handle(err)
	}
}
