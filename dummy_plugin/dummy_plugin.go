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
	"fmt"

	as "github.com/banzaicloud/hollowtrees/actionserver"
)

// DummyAlertHandler : dummy implementation of AlertHandler
type DummyAlertHandler struct {
}

func newDummyAlertHandler() *DummyAlertHandler {
	return &DummyAlertHandler{}
}

// Handle : dummy implementation that returns the alert event's name
func (d *DummyAlertHandler) Handle(event *as.AlertEvent) (*as.ActionResult, error) {
	fmt.Printf("got GRPC request, handling alert: %#v\n", event)
	return &as.ActionResult{Status: "ok"}, nil
}

func main() {
	fmt.Println("Starting Hollowtrees ActionServer")
	as.Serve(":9093", newDummyAlertHandler())
}
