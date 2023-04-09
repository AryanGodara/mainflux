// Copyright (c) Mainflux
// SPDX-License-Identifier: Apache-2.0

package grpc_test

import (
	"fmt"
	"log"
	"net"
	"os"
	"testing"

	"github.com/mainflux/mainflux"
	"github.com/mainflux/mainflux/pkg/uuid"
	"github.com/mainflux/mainflux/things"
	grpcapi "github.com/mainflux/mainflux/things/api/auth/grpc"
	"github.com/mainflux/mainflux/things/mocks"
	"github.com/opentracing/opentracing-go/mocktracer"
	"google.golang.org/grpc"
)

const (
	port  = 7000
	token = "token"
	wrong = "wrong"
	email = "john.doe@email.com"
)

var svc things.Service

func TestMain(m *testing.M) {
	serverErr := make(chan error)
	testRes := make(chan int)
	done := make(chan bool)
	endTest := make(chan int)

	server := startServer(serverErr, done, endTest)

	go func() {
		for {
			select {
			case <-testRes:
				return
			case err := <-serverErr:
				if err != nil {
					log.Fatalf("gPRC Server Terminated")
				}
			}
		}
	}()

	code := m.Run()
	testRes <- code

	server.Stop()

	close(serverErr)
	close(done)

	os.Exit(code)
}

func startServer(serverErr chan error, done chan bool, endTest chan int) *grpc.Server {
	svc = newService(map[string]string{token: email})
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("got unexpected error while creating new listerner: %s", err)
	}

	server := grpc.NewServer()
	mainflux.RegisterThingsServiceServer(server, grpcapi.NewServer(mocktracer.New(), svc))

	go func(done chan bool, endTest chan int, server *grpc.Server) {
		for {
			select {
			case serverErr <- server.Serve(listener):
				close(serverErr)
				return
			case <-done:
				return
			}

		}
	}(done, endTest, server)

	return server
}

func newService(tokens map[string]string) things.Service {
	policies := []mocks.MockSubjectSet{{Object: "users", Relation: "member"}}
	auth := mocks.NewAuthService(tokens, map[string][]mocks.MockSubjectSet{email: policies})
	conns := make(chan mocks.Connection)
	thingsRepo := mocks.NewThingRepository(conns)
	channelsRepo := mocks.NewChannelRepository(thingsRepo, conns)
	chanCache := mocks.NewChannelCache()
	thingCache := mocks.NewThingCache()
	idProvider := uuid.NewMock()

	return things.New(auth, thingsRepo, channelsRepo, chanCache, thingCache, idProvider)
}
