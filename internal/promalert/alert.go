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
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gopkg.in/go-playground/validator.v9"

	"github.com/banzaicloud/hollowtrees/internal/ce"
	"github.com/banzaicloud/hollowtrees/pkg/auth"
)

type Alerts []Alert

func (alerts Alerts) Validate() error {
	for _, alert := range alerts {
		if alert.Labels["cluster_id"] == "" {
			return errors.New("invalid alert: mandatory 'cluster_id' parameter is missing")
		}
		if alert.Labels["org_id"] == "" {
			return errors.New("invalid alert: mandatory 'org_id' parameter is missing")
		}

		err := validator.New().Struct(alert)
		if err != nil {
			return err
		}
	}

	return nil
}

func (alerts Alerts) Authorize(user *auth.User) error {
	for _, alert := range alerts {
		if alert.Labels["cluster_id"] == "" || alert.Labels["org_id"] == "" || alert.Labels["cluster_id"] != user.ClusterID || alert.Labels["org_id"] != user.OrgID {
			return errors.Errorf("invalid alert: unauthorized")
		}
	}

	return nil
}

// Alert describes an incoming Prometheus alert
type Alert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL" validate:"url"`
}

// convertToCE converts incoming prometheus alert struct to CloudEvent struct
func (a *Alert) convertToCE(cid string) (*ce.Event, error) {
	e := &ce.Event{}

	for k, v := range a.Labels {
		e.Set(k, v)
	}
	e.Set("correlationid", cid)
	e.Set("labels", a.Labels)

	e.Set("id", uuid.NewV4().String())
	e.Set("type", fmt.Sprintf("%s%s", CETypePrefix, a.Labels["alertname"]))
	e.Set("specversion", "0.2")
	u, err := url.Parse(a.GeneratorURL)
	if err != nil {
		return nil, err
	}
	e.Set("source", *u)
	e.Set("time", &a.StartsAt)
	e.Set("eventType", "prometheus")

	return e, nil
}
