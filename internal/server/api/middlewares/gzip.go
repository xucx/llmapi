package middlewares

import (
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/xucx/llmapi/internal/server/api/middleware"
)

type Gzip struct {
	middleware.NopMiddleware
}

func NewGzip() *Gzip {
	return &Gzip{}
}

func (*Gzip) Http() echo.MiddlewareFunc {
	return echomiddleware.Gzip()
}
