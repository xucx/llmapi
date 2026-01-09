package middlewares

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	AuthHeader = "Authorization"
)

type Auth struct {
	tokens []string
}

func NewAuth(tokens []string) *Auth {
	return &Auth{tokens: tokens}
}

func (a Auth) checkAuth(ctx context.Context) error {

	token, err := a.token(ctx)
	if err != nil {
		return err
	}

	ts := strings.Split(token, " ")
	if len(ts) == 2 {
		if ts[0] != "Bearer" {
			return errors.New("invalid token format")
		}
		for _, t := range a.tokens {
			if ts[1] == t {
				return nil
			}
		}
	}

	return errors.New("auth fail")
}

func (a Auth) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response interface{}, err error) {
		if err := a.checkAuth(ctx); err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "Unauthenticated: %v", err)
		}
		return handler(ctx, req)
	}
}

func (a Auth) Stream() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := a.checkAuth(stream.Context()); err != nil {
			return status.Errorf(codes.Unauthenticated, "Unauthenticated: %v", err)
		}

		return handler(srv, stream)
	}
}

func (a Auth) token(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		tokens := md.Get(AuthHeader)
		if len(tokens) > 0 {
			return tokens[0], nil
		}
	}

	return "", errors.New("authorization not found")
}
