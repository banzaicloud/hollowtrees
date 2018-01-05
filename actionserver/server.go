package actionserver

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/banzaicloud/hollowtrees/action"
	"google.golang.org/grpc"
)

type actionServer struct {
	AlertHandler AlertHandler
}

// AlertEvent contains the properties of the Alert that was triggered
type AlertEvent action.AlertEvent

// ActionResult contains the properties of the Alert that was triggered
type ActionResult action.ActionResult

// AlertHandler : should be implemented by the plugins that are doing some actions based on alerts
type AlertHandler interface {
	Handle(*AlertEvent) (*ActionResult, error)
}

func newServer() actionServer {
	as := actionServer{}
	return as
}

func (as *actionServer) HandleAlert(ctx context.Context, event *action.AlertEvent) (*action.ActionResult, error) {
	fmt.Println(ctx)
	var e = AlertEvent{
		EventType: event.EventType,
		Resource:  event.Resource,
		EventId:   event.EventId,
		Data:      event.Data,
	}

	result, err := as.AlertHandler.Handle(&e)
	if err != nil {
		return nil, err
	}

	r := &action.ActionResult{
		Status: result.Status,
	}
	return r, nil
}

func (as *actionServer) register(ah AlertHandler) {
	as.AlertHandler = ah
}

// Serve : registers the AlertHandler and starts the GRPC server
func Serve(port int, ah AlertHandler) {
	as := newServer()
	as.register(ah)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	action.RegisterActionServer(grpcServer, &as)
	grpcServer.Serve(lis)
}
