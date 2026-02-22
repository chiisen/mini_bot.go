package channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/chiisen/mini_bot/pkg/logger"
)

type Channel interface {
	Start(ctx context.Context) error
}

type Manager struct {
	channels []Channel
}

func NewManager() *Manager {
	return &Manager{
		channels: make([]Channel, 0),
	}
}

func (m *Manager) Register(ch Channel) {
	m.channels = append(m.channels, ch)
}

func (m *Manager) StartAll(ctx context.Context) error {
	if len(m.channels) == 0 {
		return fmt.Errorf("no channels registered")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(m.channels))

	for _, ch := range m.channels {
		wg.Add(1)
		go func(c Channel) {
			defer wg.Done()
			if err := c.Start(ctx); err != nil {
				errCh <- err
			}
		}(ch)
	}

	// In MVP, we just launch and wait until context is cancelled or all error out.
	// A proper implementation might gracefully shutdown everything if one critical channel fails.
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Return the first error if any channel fails immediately, or nil if context gets cancelled gracefully
	select {
	case <-ctx.Done():
		logger.Info("Channels manager shutting down")
		return nil
	case err := <-errCh:
		if err != nil {
			return err
		}
	}
	return nil
}
