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

package main

import (
	"flag"
	"fmt"

	gp "github.com/banzaicloud/hollowtrees/pkg/grpcplugin"
)

// dummyEventHandler dummy implementation of EventHandler
type dummyEventHandler struct{}

// Handle dummy implementation
func (d *dummyEventHandler) Handle(event *gp.CloudEvent) (*gp.Result, error) {
	fmt.Printf("got GRPC request, handling alert: %s\n", event.Data)

	return &gp.Result{Status: "ok"}, nil
}

var listenAddr string

func init() {
	flag.StringVar(&listenAddr, "listen-addr", ":9091", "address to listen on")
}

func main() {
	flag.Parse()

	fmt.Printf("Hollowtrees Dummy GRPC EventHandler Plugin listening on %s\n", listenAddr)
	err := gp.Serve(listenAddr, &dummyEventHandler{})
	if err != nil {
		panic(err)
	}
}
