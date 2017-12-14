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
func (*DummyAlertHandler) Handle(event *as.AlertEvent) (*as.ActionResult, error) {
	fmt.Printf("got GRPC request, handling alert: %v\n", event.AlertName)
	ar := as.ActionResult{
		Status: event.AlertName,
	}
	return &ar, nil
}

func main() {
	fmt.Println("Starting Hollowtrees ActionServer")
	as.Serve(newDummyAlertHandler())
}
