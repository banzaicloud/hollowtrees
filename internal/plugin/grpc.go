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

package plugin

import (
	"context"

	"google.golang.org/grpc"

	"github.com/banzaicloud/hollowtrees/internal/ce"
	"github.com/banzaicloud/hollowtrees/pkg/grpcplugin/proto"
)

type grpcPlugin struct {
	BasePlugin
	address string
}

// NewGrpcPlugin initializes a grpcPlugin
func NewGrpcPlugin(name string, address string) *grpcPlugin {
	return &grpcPlugin{
		BasePlugin: BasePlugin{
			name: name,
		},
		address: address,
	}
}

// Handle sends the CloudEvent to a GRPC plugin endpoint
func (p *grpcPlugin) Handle(event *ce.Event) error {
	conn, err := grpc.Dial(p.address, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()

	j, err := event.MarshalJSON()
	if err != nil {
		return err
	}

	client := proto.NewEventHandlerClient(conn)
	ez := &proto.CloudEvent{
		Specversion: event.SpecVersion,
		Type:        event.Type,
		Source:      event.Source.String(),
		Id:          event.ID,
		Time:        event.Time.String(),
		Schemaurl:   event.SchemaURL.String(),
		Contenttype: "application/cloudevents+json",
		Extensions:  event.GetExtensions(),
		Data:        j,
	}
	_, err = client.Handle(context.Background(), ez)
	if err != nil {
		return err
	}

	return nil
}
