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

package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/banzaicloud/hollowtrees/action"
	"github.com/banzaicloud/hollowtrees/conf"
	"google.golang.org/grpc"
)

type Plugin interface {
	name() string
	exec(event action.AlertEvent) error
}

func NewPlugin(pc conf.PluginConfig) Plugin {
	switch pc.Type {
	case "grpc":
		return &GrpcPlugin{
			PluginBase{
				Name:    pc.Name,
				Address: pc.Address,
			},
		}
	case "fn":
		return &FnPlugin{
			PluginBase: PluginBase{
				Name:    pc.Name,
				Address: pc.Address,
			},
			App:      pc.Properties["app"],
			Function: pc.Properties["function"],
		}
	default:
		return nil
	}
}

type Plugins []Plugin

type PluginBase struct {
	Name    string
	Address string
}

func (p *PluginBase) name() string {
	return p.Name
}

type GrpcPlugin struct {
	PluginBase
}

func (p *GrpcPlugin) exec(event action.AlertEvent) error {
	conn, err := grpc.Dial(p.Address, grpc.WithInsecure())
	if err != nil {
		log.Errorf("couldn't create GRPC channel to action server: %v", err)
		return err
	}
	client := action.NewActionClient(conn)
	_, err = client.HandleAlert(context.Background(), &event)
	if err != nil {
		log.WithField("eventId", event.EventId).Errorf("Failed to handle alert: %v", err)
		return err
	}
	conn.Close()
	return nil
}

type FnPlugin struct {
	PluginBase
	App      string
	Function string
}

type asyncResponse struct {
	CallID string  `json:"call_id"`
	Err    fnError `json:"error"`
}

type fnError struct {
	Message string `json:"message"`
}

func (p *FnPlugin) exec(event action.AlertEvent) error {
	u := url.URL{Scheme: "http", Host: p.Address}
	u.Path = path.Join(u.Path, "r", p.App, p.Function)

	b, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	// TODO: authentication if needed
	resp, err := http.Post(u.String(), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// this is a sync function's response
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		log.Infof("sync fn function completed, call id: %s, response body: %s", resp.Header["Fn_call_id"][0], string(body))
	} else if resp.StatusCode == 202 {
		// this is an async function's response
		r := &asyncResponse{}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		err = json.Unmarshal(body, r)
		if err != nil {
			return err
		}
		if r.CallID != "" {
			log.Infof("async fn function submitted, call id: %s", r.CallID)
		} else if r.Err.Message != "" {
			return errors.New(r.Err.Message)
		} else {
			return fmt.Errorf("couldn't parse response body returned by the async function: %s", body)
		}
	} else {
		return fmt.Errorf("fn server returned status code %d, expected 200 or 202", resp.StatusCode)
	}

	return nil
}
