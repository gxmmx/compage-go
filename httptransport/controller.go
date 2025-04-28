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
	utils "github.com/gxmmx/compage-go/utils"

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
	started bool
	log     *slog.Logger

	addr       string
	baseRouter *mux.Router
	subRouters map[string]*mux.Router
	server     *http.Server

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
		settings: settings,
		log:      logger,
	}

	// Create router
	addr := net.JoinHostPort(settings.Host, strconv.Itoa(settings.Port))
	baseRouter := mux.NewRouter()

	// set prefix
	if settings.Prefix != "" {
		baseRouter = baseRouter.PathPrefix(utils.EnsureLeadingSlash(settings.Prefix)).Subrouter()
	}

	// Set global middleware
	// baseRouter.Use(controller.contextMiddleware(requestCtx))
	baseRouter.Use(controller.requestIDMiddleware)
	baseRouter.Use(controller.logMiddleware)

	// Set handler for not found
	baseRouter.NotFoundHandler = notFoundHandler()

	// Create server
	server := &http.Server{
		Addr:    addr,
		Handler: baseRouter,
		BaseContext: func(_ net.Listener) context.Context {
			return requestCtx
		},
		// TODO: Look into error logger - > api errors not logged propperly
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
	controller.baseRouter = baseRouter
	controller.server = server

	return controller
}

// -----------------------------------------------------------------------------
// Public functions
// -----------------------------------------------------------------------------

func (c *Controller) AddMiddleware(sub string, f mux.MiddlewareFunc) {
	if c.started {
		panic("Cannot add middleware after server started")
	}
	sub = c.ensureSubRouter(sub)
	c.subRouters[sub].Use(f)
}

func (c *Controller) AddHandler(sub string, path string, method string, handler http.HandlerFunc) {
	if c.started {
		panic("Cannot add handler after server started")
	}
	sub = c.ensureSubRouter(sub)
	c.subRouters[sub].HandleFunc(path, handler).Methods(method).Name(path)
	// routePath := ""
	// err := c.baseRouter.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
	// 	path, pathErr := route.GetPathTemplate()
	// 	if pathErr != nil {
	// 		return pathErr
	// 	}
	// 	routePath = path
	// 	return nil
	// })
	// if err != nil {
	// 	fmt.Println("error retrieving route path:", err)
	// } else {
	// 	fmt.Println("real route path:", routePath)
	// }
}

func (c *Controller) Start(ctx context.Context) error {
	c.started = true
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

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

func (c *Controller) ensureSubRouter(sub string) string {
	if sub == "" {
		sub = "/v1"
	}
	sub = utils.EnsureLeadingSlash(sub)
	if c.subRouters == nil {
		c.subRouters = make(map[string]*mux.Router)
	}
	if _, ok := c.subRouters[sub]; !ok {
		subRouter := c.baseRouter.PathPrefix(sub).Subrouter()
		c.subRouters[sub] = subRouter
	}
	return sub
}
