package app

import "context"

type Hook func(ctx context.Context) error

type Hooks struct {
	onStart    []Hook
	onShutdown []Hook
}

func (h *Hooks) OnStart(fn Hook) {
	h.onStart = append(h.onStart, fn)
}

func (h *Hooks) OnShutdown(fn Hook) {
	h.onShutdown = append([]Hook{fn}, h.onShutdown...)
}

func (h *Hooks) RunStart(ctx context.Context) error {
	for _, fn := range h.onStart {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (h *Hooks) RunShutdown(ctx context.Context) error {
	var lastErr error
	for _, fn := range h.onShutdown {
		if err := fn(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
