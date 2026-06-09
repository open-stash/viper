package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/open-stash/viper/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Init(cfg *config.Config) *zerolog.Logger {

	const prodStr string = "production"

	// Set global level based on environment
	switch cfg.Server.Env {
	case prodStr:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	var baseLogger zerolog.Logger

	if cfg.Server.Env == prodStr {
		baseLogger = zerolog.New(os.Stdout)
	} else {
		baseLogger = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false, // Enable colors
			// Only list parts that are actually set; zerolog renders any unlisted
			// part (e.g. "service"/"err") as %!s(<nil>). The error field is keyed
			// "error" and renders inline after the message as a normal field.
			PartsOrder: []string{
				"time", "level", "caller", "env", "message",
			},
			FormatLevel: func(i any) string {
				return strings.ToUpper(fmt.Sprintf("[%s]", i))
			},
			FormatCaller: func(caller any) string {
				return fmt.Sprintf("(%s)", caller)
			},
		})
	}

	baseLogger = baseLogger.With().
		Timestamp().
		Str("env", cfg.Server.Env).
		Logger() // finalize

	// Add caller info for dev
	if cfg.Server.Env != prodStr {
		baseLogger = baseLogger.With().Caller().Logger()
	}

	log.Logger = baseLogger

	return &baseLogger
}
