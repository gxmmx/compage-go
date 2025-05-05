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

	app "github.com/gxmmx/compage-go/app/core"
	apperrors "github.com/gxmmx/compage-go/errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type PostgresController interface {
	app.Unit
	// Unit() *app.Unit
	Init(ctx context.Context, resilient bool) error
	GetPool() (*pgxpool.Pool, error)
	createPool(ctx context.Context) (*pgxpool.Pool, error)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	app.Unit
	pool *pgxpool.Pool
	lock sync.RWMutex
	log  *slog.Logger

	settings  *Settings
	configmap *ConfigMap
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewController(name string, settings *Settings, configmap *ConfigMap) *Controller {
	controller := &Controller{}

	if settings == nil {
		settings = NewSettings()
	}
	if configmap == nil {
		configmap = NewConfigMap()
	}
	if name == "" {
		name = "postgres"
	}

	controller.settings = settings
	controller.configmap = configmap
	controller.log = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	unit := app.NewUnit(name, app.UnitKindData, controller.runUnit)
	controller.Unit = unit

	return controller
}

// -----------------------------------------------------------------------------
// Interface functions
// -----------------------------------------------------------------------------

func (c *Controller) runUnit(u app.Unit) error {
	c.log = u.GetLogger()
	c.compileConfig()

	err := c.Init(u.GetCtx(), true)
	if err != nil {
		return err
	}
	u.SetHandle(c.pool)
	return nil
}

// -----------------------------------------------------------------------------
// Public functions
// -----------------------------------------------------------------------------

func (c *Controller) Init(ctx context.Context, resilient bool) error {
	if c.pool != nil {
		return nil
	}
	isset := false
	// if not resilient, create connection pool
	if !resilient {
		// Create connection pool
		pool, err := c.createPool(ctx)
		if err != nil {
			// createError := apperrors.Internal(err, "failed to create connection pool, database will not be available")
			// c.log.ErrorContext(ctx, createError.Error())
			return apperrors.Internal(err, "failed to create connection pool, database will not be available")
		}
		c.lock.Lock()
		defer c.lock.Unlock()
		c.pool = pool
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
				pool, err := c.createPool(ctx)
				if err != nil {
					createError := apperrors.Internal(err, "failed to create connection pool, retrying..")
					c.log.ErrorContext(ctx, createError.Error())
					<-ticker.C
					continue loop
				}
				c.lock.Lock()
				defer c.lock.Unlock()
				c.pool = pool
				isset = true
				break loop
			}
		}
	}
	if !isset {
		return apperrors.Internal(nil, "Connection pools was not set")
	}
	return nil
}

func (c *Controller) GetPool() (*pgxpool.Pool, error) {
	if c.pool == nil {
		return nil, apperrors.Internal(nil, "database connection pool is not initialized")
	}
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.pool, nil
}

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

func (c *Controller) createPool(ctx context.Context) (*pgxpool.Pool, error) {
	// Set connection string
	dsn := fmt.Sprintf(
		"user=%s password=%s host=%s port=%d dbname=%s sslmode=%s",
		c.settings.User, c.settings.Pass, c.settings.Host, c.settings.Port, c.settings.Name, c.settings.SslMode,
	)

	// Parse connection string
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database connection parameters: %w", err)
	}

	// Configure connection pool
	config.MaxConns = int32(c.settings.MaxConns)
	config.MinConns = int32(c.settings.MinConns)
	config.MaxConnLifetime = c.settings.MaxConnLifetime
	config.HealthCheckPeriod = c.settings.HealthCheckPeriod
	config.ConnConfig.ConnectTimeout = c.settings.QueryTimeout

	// SSL configuration
	var tlsCaCert *x509.Certificate = c.settings.SslCaCrt
	if c.settings.SslMode != "disable" {
		// If SSL dev mode is enabled, query host for CA certificate.
		if tlsCaCert == nil && c.settings.SslDevMode {
			cert, err := getHostCaCertificate(c.settings.Host, c.settings.Port)
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
				ServerName: c.settings.Host,
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
	if c.settings.SslMode == "disable" {
		c.log.DebugContext(ctx, "Postgres connection established without SSL")
	} else if c.settings.SslCaCrt == nil && tlsCaCert != nil {
		c.log.DebugContext(ctx, "Postgres connection established with SSL using host provided CA certificate in dev mode")
	} else if c.settings.SslCaCrt != nil {
		c.log.DebugContext(ctx, "Postgres connection established with SSL using provided CA certificate")
	} else {
		c.log.DebugContext(ctx, "Postgres connection established with SSL")
	}

	return pool, nil
}
