package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"fmt"

	"github.com/open-stash/viper/config"
	"github.com/open-stash/viper/internal/app"
	"github.com/open-stash/viper/internal/server"
	"github.com/open-stash/viper/pkg/logger"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	log := logger.Init(&cfg)
	log.Info().Msg("logger initialized")

	container, err := app.NewContainer(ctx, &cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize dependencies")
	}

	// start consumer runs in separate go routine
	app.StartConsumer(ctx, container)
	router := app.NewRouter(container)

	srv := server.New(router, fmt.Sprintf("%d", cfg.Server.Port), log)
	srv.Start()

	<-ctx.Done() // wait for the signal
	log.Info().Msg("shutdown signal received")

	// 1. Stop HTTP server (stop accepting requests)
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error().Err(err).Msg("server shutdown failed")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := container.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("dependecies shutdown failed")
	}

	// Shutdown done
	log.Info().Msg("graceful shutdown complete")

}
