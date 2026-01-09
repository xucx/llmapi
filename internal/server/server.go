package server

import (
	"context"
	"net"

	"github.com/xucx/llmapi"

	apiv1 "github.com/xucx/llmapi/api/v1"
	"github.com/xucx/llmapi/internal/server/api/middlewares"
	v1 "github.com/xucx/llmapi/internal/server/api/v1"
	"github.com/xucx/llmapi/log"

	"google.golang.org/grpc"
)

func Run(ctx context.Context) error {

	models, err := llmapi.NewModels(C.Model)
	if err != nil {
		return err
	}

	listen, err := net.Listen("tcp", C.Host)
	if err != nil {
		return err
	}

	opts := []grpc.ServerOption{}
	if len(C.Tokens) > 0 {
		auth := middlewares.NewAuth(C.Tokens)
		opts = append(opts, grpc.ChainUnaryInterceptor(auth.Unary()))
		opts = append(opts, grpc.ChainStreamInterceptor(auth.Stream()))
	}

	server := grpc.NewServer(opts...)
	apiv1.RegisterApiServiceServer(server, v1.NewApiService(models))

	ch := make(chan error, 1)
	go func() {
		defer func() {
			log.Info("api exit")
		}()

		log.Infof("api listen at %s", C.Host)
		ch <- server.Serve(listen)
	}()

	for {
		select {
		case err := <-ch:
			return err
		case <-ctx.Done():
			server.GracefulStop()
		}
	}
}
