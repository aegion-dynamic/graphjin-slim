package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// WatchSignals installs the standard graceful-shutdown signal handler.
func WatchSignals(log *zap.SugaredLogger, shutdown func(context.Context) error) {
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals
		log.Info("shutdown signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			log.Warnf("graceful shutdown failed: %s", err)
		}
	}()
}
