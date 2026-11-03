// Command user-management-api serves the REST API (Fiber) and the gRPC API
// from one process. main is the composition root: it creates the shared
// infrastructure and hands it to the routers.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"google.golang.org/grpc"

	"github.com/SarawinT/user-management-api/app/adapters/repository/mongodb"
	"github.com/SarawinT/user-management-api/app/adapters/security"
	"github.com/SarawinT/user-management-api/app/constants"
	"github.com/SarawinT/user-management-api/app/database_init"
	"github.com/SarawinT/user-management-api/app/models/response"
	"github.com/SarawinT/user-management-api/app/routers"
	"github.com/SarawinT/user-management-api/config"
	"github.com/SarawinT/user-management-api/logz"
	"github.com/SarawinT/user-management-api/middleware"
)

const (
	appName         = "user-management-api"
	shutdownTimeout = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config/config.yaml", "path to the YAML config file")
	flag.Parse()

	cfg, err := config.InitialConfig(*configPath)
	if err != nil {
		return err
	}
	logger := logz.Init(cfg.Log.Level, appName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mongoClient, err := database_init.NewMongoClient(ctx, cfg.Mongo.URI)
	if err != nil {
		return err
	}
	defer disconnectMongo(mongoClient, logger)

	db := mongoClient.Database(cfg.Mongo.Database)
	if err := mongodb.EnsureUserIndexes(ctx, db); err != nil {
		return err
	}
	logger.Info("connected to MongoDB", "database", cfg.Mongo.Database)

	jwtToken, err := security.NewJwtToken(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.TTL)
	if err != nil {
		return fmt.Errorf("jwt.secret: %w", err)
	}

	middlewares := middleware.New(logger, jwtToken)

	// REST
	app := initFiber(middlewares)
	healthCheck(app, mongoClient)
	routers.AuthRouter(app, middlewares, db, jwtToken)
	routers.UserRouter(app, middlewares, db)

	// gRPC
	grpcSrv := routers.GrpcRouter(logger, db, jwtToken, cfg.Server.GRPCReflection)
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}

	errCh := make(chan error, 2)
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
		logger.Info("HTTP server listening", "addr", addr)
		if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			errCh <- fmt.Errorf("http: %w", err)
		}
	}()
	go func() {
		logger.Info("gRPC server listening", "addr", grpcLis.Addr().String())
		if err := grpcSrv.Serve(grpcLis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errCh <- fmt.Errorf("grpc: %w", err)
		}
	}()

	return gracefulShutdown(ctx, logger, app, grpcSrv, errCh)
}

func initFiber(m *middleware.Middleware) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      appName,
		ErrorHandler: m.ErrorHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    1 << 20, // 1 MiB
	})
	// Order matters: the logger is outermost so it also records requests
	// that panic (converted to 500 by Recover) or are rejected early.
	app.Use(m.AttachRequestLogger())
	app.Use(m.Recover())
	app.Use(m.SetHeaderID())
	return app
}

// healthCheck serves GET /health: 200 when MongoDB answers a ping, 503
// otherwise.
func healthCheck(app *fiber.App, client *mongo.Client) {
	app.Get("/health", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := client.Ping(ctx, readpref.Primary()); err != nil {
			return response.SendErrorResponse(c, fiber.StatusServiceUnavailable,
				constants.CodeServiceUnavailable, constants.ResponseMessage(constants.CodeServiceUnavailable))
		}
		return response.SendSuccessResponse(c, fiber.StatusOK, nil)
	})
}

// gracefulShutdown waits for a signal or for either server to fail, then
// stops both, draining in-flight requests within shutdownTimeout.
func gracefulShutdown(ctx context.Context, logger *slog.Logger, app *fiber.App, grpcSrv *grpc.Server, errCh <-chan error) error {
	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-errCh:
		logger.Error("server failed, shutting down", "err", runErr)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("http shutdown", "err", err)
	}

	done := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		grpcSrv.Stop()
	}

	logger.Info("servers stopped")
	return runErr
}

func disconnectMongo(client *mongo.Client, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		logger.Error("mongo disconnect", "err", err)
	}
}
