package promalert

import (
	"fmt"
	"net/url"
	"time"

	"github.com/satori/go.uuid"

	"github.com/banzaicloud/hollowtrees/internal/ce"
)

// Alert describes an incoming Prometheus alert
type Alert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
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
