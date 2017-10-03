package log

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pydio/services/common/service/context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type correlationIDType int

var (
	logger    *zap.Logger
	stdLogger *zap.Logger
)

func init() {
	var err error

	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err = config.Build()
	if err != nil {
		log.Fatal("Could not initalise logger")
	}

	stdConfig := zap.NewDevelopmentConfig()
	stdConfig.EncoderConfig.EncodeName = noOpNameEncoder
	stdConfig.EncoderConfig.EncodeLevel = noOpLevelEncoder
	stdConfig.EncoderConfig.EncodeTime = noOpTimeEncoder
	stdConfig.EncoderConfig.EncodeCaller = noOpCallerEncoder
	stdConfig.EncoderConfig.EncodeDuration = noOpDurationEncoder

	stdLogger, err = stdConfig.Build()
	if err != nil {
		log.Fatal("Could not initalise standard logger")
	}

	defer stdLogger.Sync()
	defer logger.Sync()

	log.SetOutput(&logwriter{})
}

// Logger returns a zap logger with as much context as possible
func Logger(ctx context.Context) *zap.Logger {
	newLogger := logger
	if ctx != nil {
		if serviceName := servicecontext.GetServiceName(ctx); serviceName != "" {
			if serviceColor := servicecontext.GetServiceColor(ctx); serviceColor > 0 {
				newLogger = newLogger.Named(fmt.Sprintf("\x1b[%dm%s\x1b[0m", serviceColor, serviceName))
			} else {
				newLogger = newLogger.Named(serviceName)
			}
		}
		if ctxReqID := servicecontext.GetRequestID(ctx); ctxReqID != "" {
			newLogger = newLogger.With(zap.String("rqID", ctxReqID))
		}
		if ctxSessionID := servicecontext.GetSessionID(ctx); ctxSessionID != "" {
			newLogger = newLogger.With(zap.String("sessionID", ctxSessionID))
		}
	} else {
		newLogger = stdLogger
	}
	return newLogger
}

type logwriter struct{}

func (lw *logwriter) Write(p []byte) (n int, err error) {
	Logger(context.Background()).Info(string(p))

	return len(p), nil
}

func noOpNameEncoder(loggerName string, enc zapcore.PrimitiveArrayEncoder)            {}
func noOpLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder)         {}
func noOpTimeEncoder(time time.Time, enc zapcore.PrimitiveArrayEncoder)               {}
func noOpCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {}
func noOpDurationEncoder(duration time.Duration, enc zapcore.PrimitiveArrayEncoder)   {}
