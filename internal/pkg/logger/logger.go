package logger

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Init(env string) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	level := zerolog.InfoLevel
	if env == "development" {
		level = zerolog.DebugLevel
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	zerolog.SetGlobalLevel(level)

	log.Info().Str("env", env).Msg("logger initialized")
}

func WithTraceID(ctx context.Context) *zerolog.Logger {
	var traceID string
	if v := ctx.Value("trace_id"); v != nil {
		if id, ok := v.(string); ok {
			traceID = id
		}
	}
	if traceID == "" {
		traceID = "unknown"
	}
	logger := log.With().Str("trace_id", traceID).Logger()
	return &logger
}
