package logger

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var logger zerolog.Logger

func init() {
	logger = log.With().Timestamp().CallerWithSkipFrameCount(3).Stack().Logger()
}

func SetLogLevel(level string) {
	var l zerolog.Level

	switch strings.ToLower(level) {
	case "error":
		l = zerolog.ErrorLevel
	case "warn":
		l = zerolog.WarnLevel
	case "info":
		l = zerolog.InfoLevel
	case "debug":
		l = zerolog.DebugLevel
	default:
		l = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(l)

}

func Info(message string, args ...any) {
	log.Info().Msgf(message, args...)
}

func DebugJson(message string, data any) {
	value, _ := json.Marshal(data)
	logger.Debug().RawJSON(message, value).Msg("")
}

func Debug(message string, args ...any) {
	logger.Debug().Msgf(message, args...)
}

func Warn(message string, args ...any) {
	log.Warn().Msgf(message, args...)
}

func Error(message string, args ...any) {
	logger.Error().Msgf(message, args...)
}

func Fatal(message string, args ...any) {
	logger.Fatal().Msgf(message, args...)
	os.Exit(1)
}

func Log(message string, args ...any) {
	if len(args) == 0 {
		log.Info().Msg(message)
	} else {
		log.Info().Msgf(message, args...)
	}
}
