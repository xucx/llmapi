package server

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/soheilhy/cmux"
	"github.com/xucx/llmapi"
	"golang.org/x/sync/errgroup"

	apiv1 "github.com/xucx/llmapi/api/v1"
	"github.com/xucx/llmapi/internal/server/api/middleware"
	"github.com/xucx/llmapi/internal/server/api/middlewares"
	v1 "github.com/xucx/llmapi/internal/server/api/v1"
	"github.com/xucx/llmapi/log"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc"
)

func Run(ctx context.Context) error {
	models, err := llmapi.NewModels(C.LLM)
	if err != nil {
		return err
	}

	listen, err := net.Listen("tcp", C.Host)
	if err != nil {
		return err
	}

	m := cmux.New(listen)
	grpcListener := m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	httpListener := m.Match(cmux.Any())

	midds := []middleware.Middleware{
		middlewares.NewGzip(),
		middlewares.NewLogger(),
		middlewares.NewAuth(
			middlewares.AuthWithTokens(C.Tokens),
			middlewares.AuthWithHttpTokenGetter(func(ctx echo.Context) (string, error) {
				if ctx.Request().URL.Path == "/api/v1/claude/messages" {
					return ctx.Request().Header.Get("X-Api-Key"), nil
				}
				return middlewares.DefaultAuthHttpHeaderGetter(ctx)
			}),
		),
	}

	grpcOpts := []grpc.ServerOption{}
	for _, m := range midds {
		grpcOpts = append(grpcOpts, grpc.ChainUnaryInterceptor(m.Unary()))
		grpcOpts = append(grpcOpts, grpc.ChainStreamInterceptor(m.Stream()))
	}

	apiService := v1.NewApiService(models)

	grpcServer := grpc.NewServer(grpcOpts...)
	apiv1.RegisterApiServiceServer(grpcServer, apiService)

	httpServer := echo.New()
	httpServer.HideBanner = true
	for _, m := range midds {
		httpServer.Use(m.Http())
	}

	httpApiV1 := httpServer.Group("/api/v1")
	httpApiV1.POST("/openai/completions", apiService.OpenaiCompletion)
	httpApiV1.POST("/claude/messages", apiService.ClaudeCreateMessage)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Infof("gRPC server starting...")
		if err := grpcServer.Serve(grpcListener); err != nil {
			if err != grpc.ErrServerStopped && !isMuxClosedConnError(err) {
				return fmt.Errorf("gRPC serve error: %w", err)
			}
		}
		return nil
	})

	g.Go(func() error {
		log.Infof("HTTP server starting...")
		if err := httpServer.Server.Serve(httpListener); err != nil {
			if err != grpc.ErrServerStopped && !isMuxClosedConnError(err) {
				return fmt.Errorf("HTTP serve error: %w", err)
			}
		}
		return nil
	})

	g.Go(func() error {
		log.Infof("api listen at %s", C.Host)
		if err := m.Serve(); err != nil {
			if !isMuxClosedConnError(err) {
				return fmt.Errorf("api serve error: %w", err)
			}
		}
		return nil
	})

	// exit
	g.Go(func() error {
		<-gctx.Done()
		log.Infow("shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		m.Close()

		done := make(chan struct{})
		go func() {
			defer close(done)
			grpcServer.GracefulStop()
			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				log.Errorf("http server shutdown error: %v", err)
			}
		}()

		select {
		case <-done:
			log.Info("server shutdown")
		case <-shutdownCtx.Done():
			log.Warnf("shutdown timeout, forcing exit")
			grpcServer.Stop()
			httpServer.Close()
		}

		return nil
	})

	return g.Wait()

}

func isMuxClosedConnError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "mux: server closed")
}
