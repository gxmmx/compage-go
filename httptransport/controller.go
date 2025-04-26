package httptransport

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"

	apperrors "github.com/gxmmx/compage-go/errors"

	mux "github.com/gorilla/mux"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type HTTPController interface {
	AddHandler(path string, method string, handler http.HandlerFunc)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	requestCtx context.Context
	log        *slog.Logger

	router *mux.Router
	server *http.Server
	addr   string

	settings *Settings
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewController(settings *Settings, logger *slog.Logger, requestCtx context.Context) *Controller {
	if settings == nil {
		settings = NewSettings()
	}
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	if requestCtx == nil {
		requestCtx = context.Background()
	}

	controller := &Controller{
		requestCtx: requestCtx,
		settings:   settings,
		log:        logger,
	}

	// Create router and server
	addr := net.JoinHostPort(settings.Host, strconv.Itoa(settings.Port))
	router := mux.NewRouter()

	// Set global middleware
	router.Use(controller.contextMiddleware(requestCtx))
	router.Use(controller.logMiddleware)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
		// TODO: Look into logger and error logger
		// TODO: look into base context
	}

	if settings.SslEnabled {
		if settings.SslCrt == nil || settings.SslKey == nil {
			settings.SslEnabled = false
		} else {
			tlsCert := tls.Certificate{
				Certificate: [][]byte{settings.SslCrt.Raw},
				PrivateKey:  settings.SslKey,
			}
			tlsConfig := &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{tlsCert},
			}
			server.TLSConfig = tlsConfig
		}
	}

	// Set server
	controller.addr = addr
	controller.router = router
	controller.server = server

	return controller
}

// -----------------------------------------------------------------------------
// Public functions
// -----------------------------------------------------------------------------

func (c *Controller) AddHandler(path string, method string, handler http.HandlerFunc) {
	c.router.HandleFunc(path, handler).Methods(method)
}

func (c *Controller) Start(ctx context.Context) error {
	c.log.DebugContext(ctx, "Starting HTTP server", "address", c.server.Addr, "tls", c.settings.SslEnabled)
	errCh := make(chan error, 1)
	go func() {
		var err error

		if c.settings.SslEnabled {
			// Start https server
			listener, lerr := tls.Listen("tcp", c.addr, c.server.TLSConfig)
			if lerr != nil {
				errCh <- apperrors.Internal(lerr, "transport listener failed")
			}
			err = c.server.Serve(listener)
		} else {
			// start http server
			err = c.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			errCh <- apperrors.Internal(err, "transport server failed")
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return c.Stop(context.Background())
	case err := <-errCh:
		return err
	}
}

func (c *Controller) Stop(ctx context.Context) error {
	if err := c.server.Shutdown(ctx); err != nil {
		return apperrors.Internal(err, "transport failed to shut down")
	}
	return nil
}
