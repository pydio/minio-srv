package servicecontext

import (
	"context"
	"sync/atomic"

	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/sql"
)

type contextType int

const (
	serviceColorKey contextType = iota
	serviceNameKey
	requestIDKey
	sessionIDKey
	daoKey
	configKey
)

var serviceColorCount uint64 = 30

// WithServiceColor returns a context which knows its service assigned color
func WithServiceColor(ctx context.Context, serviceColor ...uint64) context.Context {

	var color uint64

	if len(serviceColor) > 0 {
		color = serviceColor[0]
	} else {
		atomic.AddUint64(&serviceColorCount, 1)
		color = atomic.LoadUint64(&serviceColorCount)
	}

	return context.WithValue(ctx, serviceColorKey, color)
}

// WithServiceName returns a context which knows its service name
func WithServiceName(ctx context.Context, serviceName string) context.Context {
	return context.WithValue(ctx, serviceNameKey, serviceName)
}

// WithRequestID returns a context which knows its request ID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithSessionID returns a context which knows its session ID
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

func WithDAO(ctx context.Context, dao sql.DAO) context.Context {
	return context.WithValue(ctx, daoKey, dao)
}

func WithConfig(ctx context.Context, config config.Map) context.Context {
	return context.WithValue(ctx, configKey, config)
}

func GetServiceColor(ctx context.Context) uint64 {
	if color, ok := ctx.Value(serviceColorKey).(uint64); ok {
		return color
	}
	return 0
}

func GetServiceName(ctx context.Context) string {
	if name, ok := ctx.Value(serviceNameKey).(string); ok {
		return name
	}
	return ""
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func GetSessionID(ctx context.Context) string {
	if id, ok := ctx.Value(sessionIDKey).(string); ok {
		return id
	}
	return ""
}

// FromContext returns the log from the context in argument
func GetDAO(ctx context.Context) sql.DAO {
	if db, ok := ctx.Value(daoKey).(sql.DAO); ok {
		return db
	}

	return nil
}

// FromContext returns the log from the context in argument
func GetConfig(ctx context.Context) config.Map {
	if config, ok := ctx.Value(configKey).(config.Map); ok {
		return config
	}

	return nil
}
