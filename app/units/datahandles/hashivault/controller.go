package hashivault

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	// Third party
	vault "github.com/hashicorp/vault/api"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type HashivaultController interface {
	StartNativeRenewer(ctx context.Context) error
	StartClientRenewer(ctx context.Context) error
	GetClient() (*vault.Client, error)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	client  *vault.Client
	renewer *vault.Renewer
	log     *slog.Logger

	settings *Settings
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewController(settings *Settings, logger *slog.Logger) *Controller {
	if settings == nil {
		settings = NewSettings()
	}
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return &Controller{
		settings: settings,
		log:      logger,
	}
}

// -----------------------------------------------------------------------------
// Public functions
// -----------------------------------------------------------------------------

func (m *Controller) StartNativeRenewer(ctx context.Context) error {
	// join host and port
	url := net.JoinHostPort(m.settings.Host, fmt.Sprintf("%d", m.settings.Port))

	// Create a new client
	client, err := vault.NewClient(&vault.Config{
		Address: url,
	})
	if err != nil {
		m.log.Error("Failed to create Vault client", "error", err)
	}

	// Set the client
	m.client = client

	// Start the renewer
	// 	m.renewer, err = vault.NewRenewer(client)
	// 	if err != nil {
	// 		m.log.Error("Failed to create Vault renewer", "error", err)
	// 		return
	// 	}

	// go m.renewer.Renew(ctx)
	return nil
}
