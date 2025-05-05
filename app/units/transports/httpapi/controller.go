package httpapi

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"

	app "github.com/gxmmx/compage-go/app/core"
	appctx "github.com/gxmmx/compage-go/ctx"
	apperrors "github.com/gxmmx/compage-go/errors"
	utils "github.com/gxmmx/compage-go/utils"

	mux "github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type HttpApiController interface {
	app.Unit
	SetInitFunc(f InitHandlerFunc)
	AddHandler(path string, method string, handler http.HandlerFunc)
	AddMiddleware(path string, f mux.MiddlewareFunc)
	Start(ctx context.Context) error
	stop(ctx context.Context) error
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	app.Unit
	started bool
	log     *slog.Logger

	subMiddlewares []subMiddleware
	subHandlers    []subHandler

	addr       string
	rctx       context.Context
	baseRouter *mux.Router
	subRouters map[string]*mux.Router
	server     *http.Server

	settings  *Settings
	configmap *ConfigMap
	inith     InitHandlerFunc
}

type InitHandlerFunc func(*Controller) error

type subMiddleware struct {
	sub string
	f   mux.MiddlewareFunc
}

type subHandler struct {
	sub    string
	path   string
	method string
	f      http.HandlerFunc
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
		name = "httpapi"
	}

	controller.settings = settings
	controller.configmap = configmap
	controller.log = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	controller.subMiddlewares = make([]subMiddleware, 0)
	controller.subHandlers = make([]subHandler, 0)

	unit := app.NewUnit(name, app.UnitKindPort, controller.runUnit)
	controller.Unit = unit

	return controller
}

// -----------------------------------------------------------------------------
// Interface functions
// -----------------------------------------------------------------------------

func (c *Controller) runUnit(u app.Unit) error {
	c.log = u.GetLogger()
	c.compileConfig()
	err := c.inith(c)
	if err != nil {
		return err
	}
	err = c.Start(u.GetCtx())
	if err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Public functions
// -----------------------------------------------------------------------------

func (c *Controller) SetInitFunc(f InitHandlerFunc) {
	if c.started {
		panic("Cannot set init function after server started")
	}
	c.inith = f
}

func (c *Controller) AddMiddleware(sub string, f mux.MiddlewareFunc) {
	if c.started {
		panic("Cannot add middleware after server started")
	}
	c.subMiddlewares = append(c.subMiddlewares, subMiddleware{
		sub: sub,
		f:   f,
	})
	// sub = c.ensureSubRouter(sub)
	// c.subRouters[sub].Use(f)
}

func (c *Controller) AddHandler(sub string, path string, method string, handler http.HandlerFunc) {
	if c.started {
		panic("Cannot add handler after server started")
	}
	c.subHandlers = append(c.subHandlers, subHandler{
		sub:    sub,
		path:   path,
		method: method,
		f:      handler,
	})

	// sub = c.ensureSubRouter(sub)
	// c.subRouters[sub].HandleFunc(path, handler).Methods(method).Name(path)

	// DEBUG DO NOT USE
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
	rctx, _ := appctx.Child(ctx, "request")
	c.rctx = rctx
	c.initialize()

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
		return c.stop(context.Background())
	case err := <-errCh:
		return err
	}
}

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

func (c *Controller) initialize() {
	// Create router
	addr := net.JoinHostPort(c.settings.Host, strconv.Itoa(c.settings.Port))
	baseRouter := mux.NewRouter()

	// set prefix
	if c.settings.Prefix != "" {
		baseRouter = baseRouter.PathPrefix(utils.EnsureLeadingSlash(c.settings.Prefix)).Subrouter()
	}

	// Set global middleware
	baseRouter.Use(c.requestIDMiddleware)
	baseRouter.Use(c.logMiddleware)

	// Set handler for not found
	baseRouter.NotFoundHandler = notFoundHandler()
	baseRouter.MethodNotAllowedHandler = methodNotAllowedHandler()

	// Create server
	server := &http.Server{
		Addr:    addr,
		Handler: baseRouter,
		BaseContext: func(_ net.Listener) context.Context {
			return c.rctx
		},
		// TODO: Look into error logger - > api errors not logged propperly
	}

	if c.settings.SslEnabled {
		if len(c.settings.SslCrt) == 0 || c.settings.SslKey == nil {
			c.settings.SslEnabled = false
		} else {
			// Append ca cert to the chain if provided
			if c.settings.SslCaCrt != nil {
				c.settings.SslCrt = append(c.settings.SslCrt, c.settings.SslCaCrt)
			}
			// Create TLS config
			tlsCert := tls.Certificate{
				Certificate: utils.CertificatesToByteSlice(c.settings.SslCrt),
				PrivateKey:  c.settings.SslKey,
			}
			tlsConfig := &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{tlsCert},
			}
			server.TLSConfig = tlsConfig
		}
	}

	// Set server
	c.addr = addr
	c.baseRouter = baseRouter
	c.server = server

	// Set swagger
	// TODO: Look into adding swagger to base instead of subs independently

	// Set handlers
	for _, handler := range c.subHandlers {
		sub := c.ensureSubRouter(handler.sub)
		c.subRouters[sub].HandleFunc(handler.path, handler.f).Methods(handler.method).Name(handler.path)
	}
	// Set middlewares
	for _, middleware := range c.subMiddlewares {
		sub := c.ensureSubRouter(middleware.sub)
		c.subRouters[sub].Use(middleware.f)
	}
}

func (c *Controller) stop(ctx context.Context) error {
	if err := c.server.Shutdown(ctx); err != nil {
		return apperrors.Internal(err, "transport failed to shut down")
	}
	return nil
}

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
		// TODO: Add swagger to subrouter if enabled
		if c.settings.SwaggerEnabled {
			subRouter.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
		}
		c.subRouters[sub] = subRouter
	}
	return sub
}
