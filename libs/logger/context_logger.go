package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Field names for standardized logging
const (
	FieldRequestID      = "request_id"
	FieldUserID         = "user_id"
	FieldOrganizationID = "org_id"
	FieldMethod         = "method"
	FieldPath           = "path"
	FieldStatusCode     = "status"
	FieldDuration       = "duration_ms"
	FieldError          = "error"
	FieldComponent      = "component"
)

// Config holds logger configuration
type Config struct {
	Level      string
	Pretty     bool
	TimeFormat string
	Output     io.Writer
}

// DefaultConfig returns a default logger configuration
func DefaultConfig() Config {
	return Config{
		Level:      "info",
		Pretty:     false,
		TimeFormat: time.RFC3339,
		Output:     os.Stdout,
	}
}

// Setup configures the global zerolog logger
func Setup(cfg Config) {
	// Set time format
	zerolog.TimeFieldFormat = cfg.TimeFormat

	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output (pretty or standard JSON)
	var output io.Writer = cfg.Output
	if cfg.Pretty {
		output = zerolog.ConsoleWriter{
			Out:        cfg.Output,
			TimeFormat: cfg.TimeFormat,
		}
	}

	// Set global logger
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

// New creates a new logger instance with the given component name
func New(component string) zerolog.Logger {
	return log.With().Str(FieldComponent, component).Logger()
}

// FromContext extracts a logger from context or returns the default logger
func FromContext(ctx context.Context) zerolog.Logger {
	if ctx == nil {
		return log.Logger
	}

	logger := zerolog.Ctx(ctx)
	if logger.GetLevel() == zerolog.Disabled {
		return log.Logger
	}

	return *logger
}

// WithContext adds a logger to context
func WithContext(ctx context.Context, logger zerolog.Logger) context.Context {
	return logger.WithContext(ctx)
}
