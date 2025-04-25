package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	apperrors "github.com/gxmmx/compage-go/errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type PostgresController interface {
	Init(ctx context.Context, resilient bool)
	GetPool() (*pgxpool.Pool, error)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	pool *pgxpool.Pool
	lock sync.RWMutex
	log  *slog.Logger

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

func (m *Controller) Init(ctx context.Context, resilient bool) {
	if m.pool != nil {
		return
	}
	// if not resilient, create connection pool
	if !resilient {
		// Create connection pool
		pool, err := m.createPool(ctx)
		if err != nil {
			createError := apperrors.Internal(err, "failed to create connection pool, database will not be available")
			m.log.ErrorContext(ctx, createError.Error())
			return
		}
		m.lock.Lock()
		defer m.lock.Unlock()
		m.pool = pool
	} else {
		// Create connection pool with retries
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			default:
				pool, err := m.createPool(ctx)
				if err != nil {
					createError := apperrors.Internal(err, "failed to create connection pool, retrying..")
					m.log.ErrorContext(ctx, createError.Error())
					<-ticker.C
					continue loop
				}
				m.lock.Lock()
				defer m.lock.Unlock()
				m.pool = pool
				break loop
			}
		}
	}
}

func (m *Controller) GetPool() (*pgxpool.Pool, error) {
	if m.pool == nil {
		return nil, apperrors.Internal(nil, "database connection pool is not initialized")
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.pool, nil
}

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

func (m *Controller) createPool(ctx context.Context) (*pgxpool.Pool, error) {
	// Set connection string
	dsn := fmt.Sprintf(
		"user=%s password=%s host=%s port=%d dbname=%s sslmode=%s",
		m.settings.User, m.settings.Pass, m.settings.Host, m.settings.Port, m.settings.Name, m.settings.SslMode,
	)

	// Parse connection string
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database connection parameters: %w", err)
	}

	// Configure connection pool
	config.MaxConns = int32(m.settings.MaxConns)
	config.MinConns = int32(m.settings.MinConns)
	config.MaxConnLifetime = m.settings.MaxConnLifetime
	config.HealthCheckPeriod = m.settings.HealthCheckPeriod
	config.ConnConfig.ConnectTimeout = m.settings.QueryTimeout

	// SSL configuration
	var tlsCaCert *x509.Certificate = m.settings.SslCaCrt
	if m.settings.SslMode != "disable" {
		// If SSL dev mode is enabled, query host for CA certificate.
		if tlsCaCert == nil && m.settings.SslDevMode {
			cert, err := getHostCaCertificate(m.settings.Host, m.settings.Port)
			if err != nil {
				return nil, fmt.Errorf("failed to get database CA certificate for dev: %w", err)
			}
			if cert != nil {
				tlsCaCert = cert
			}
		}

		// Set CA certificate if passed or found
		if tlsCaCert != nil {
			certPool := x509.NewCertPool()
			certPool.AddCert(tlsCaCert)

			config.ConnConfig.TLSConfig = &tls.Config{
				RootCAs:    certPool,
				ServerName: m.settings.Host,
			}
		}
	}

	// Create pool
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	// Debug log connection type
	if m.settings.SslMode == "disable" {
		m.log.DebugContext(ctx, "Postgres connection established without SSL")
	} else if m.settings.SslCaCrt == nil && tlsCaCert != nil {
		m.log.DebugContext(ctx, "Postgres connection established with SSL using host provided CA certificate in dev mode")
	} else if m.settings.SslCaCrt != nil {
		m.log.DebugContext(ctx, "Postgres connection established with SSL using provided CA certificate")
	} else {
		m.log.DebugContext(ctx, "Postgres connection established with SSL")
	}

	return pool, nil
}
