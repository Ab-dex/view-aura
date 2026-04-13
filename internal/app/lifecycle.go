package app

import (
	"context"
	"fmt"
	"net/http"
)

func (a *App) Run() error {
	if err := a.Hooks.RunStart(context.Background()); err != nil {
		return err
	}
	if err := a.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (a *App) Shutdown(ctx context.Context) error {

	shutdownCtx, cancel := context.WithTimeout(ctx, a.Config.App.ShutdownTimeout)
	defer cancel()

	if err := a.Server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("HTTP server shutdown error: %v\n", err)
	}

	if err := a.Hooks.RunShutdown(ctx); err != nil {
		return err
	}

	if a.DB != nil {
		a.DB.Close()
	}

	return nil
}
