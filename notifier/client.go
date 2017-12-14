package main

import (
	"context"
	"log"

	"github.com/banzaicloud/hollowtrees/action"
	"google.golang.org/grpc"
)

func main() {
	serverAddr := "localhost:8888"
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("couldn't create GRPC channel to action server: %v", err)
	}
	defer conn.Close()

	client := action.NewActionClient(conn)
	client.HandleAlert(context.Background(), &action.AlertEvent{
		AlertName: "spot-termination-notice",
	})
}
