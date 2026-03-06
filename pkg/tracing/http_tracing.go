package tracing

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// HTTPMiddleware returns a Gin middleware for tracing.
func HTTPMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
