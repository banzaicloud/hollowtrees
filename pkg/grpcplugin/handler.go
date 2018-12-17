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

package grpcplugin

import (
	"context"

	"github.com/banzaicloud/hollowtrees/pkg/grpcplugin/proto"
	"github.com/goph/emperror"
)

// EventHandler should be implemented by the plugins that are doing some actions based on alerts
type EventHandler interface {
	Handle(*CloudEvent) (*Result, error)
}

type CloudEvent proto.CloudEvent
type Result proto.Result

type handler struct {
	EventHandler EventHandler
}

// NewHandler returns an initialized handler
func NewHandler(eh EventHandler) *handler {
	return &handler{
		EventHandler: eh,
	}
}

// Handle converts and passes the incoming event to the defined event handler
func (h *handler) Handle(ctx context.Context, ce *proto.CloudEvent) (*proto.Result, error) {
	var e = CloudEvent{
		Specversion: ce.Specversion,
		Type:        ce.Type,
		Source:      ce.Source,
		Id:          ce.Id,
		Time:        ce.Time,
		Contenttype: ce.Contenttype,
		Data:        ce.Data,
	}

	result, err := h.EventHandler.Handle(&e)
	if err != nil {
		return nil, emperror.Wrap(err, "could not handle event")
	}

	return &proto.Result{
		Status: result.Status,
	}, nil
}
