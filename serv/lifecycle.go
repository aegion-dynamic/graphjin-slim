package serv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	httpapi "github.com/aegion-dynamic/graphjin-slim/serv/v3/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var version string

const (
	defaultHP = httpapi.DefaultHostPort
)

// Initialize the watcher for the graphjin config file
func initConfigWatcher(s1 *HttpService) {
	s := s1.Load().(*graphjinService)
	if s.conf.Serv.Production {
		return
	}

	go func() {
		err := startConfigWatcher(s1)
		if err != nil {
			s.log.Fatalf("error in config file watcher: %s", err)
		}
	}()
}

// Initialize the hot deploy watcher
// func initHotDeployWatcher(s1 *HttpService) {
// 	s := s1.Load().(*graphjinService)
// 	go func() {
// 		err := startHotDeployWatcher(s1)
// 		if err != nil {
// 			s.log.Fatalf("error in hot deploy watcher: %s", err)
// 		}
// 	}()
// }

// Start the HTTP server
func startHTTP(s1 *HttpService) {
	s := s1.Load().(*graphjinService)

	r := http.NewServeMux()
	routes, err := routesHandler(s1, r, s.namespace)
	if err != nil {
		s.log.Fatalf("error setting up routes: %s", err)
	}

	srv := &http.Server{
		Addr:              s.conf.hostPort,
		Handler:           routes,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Publish srv under srvMu so a concurrent Shutdown (signal handler or an
	// external caller, e.g. demo mode) observes it safely.
	s.srvMu.Lock()
	s.srv = srv
	s.srvMu.Unlock()

	// Standalone graceful shutdown: catch SIGINT/SIGTERM and stop the server
	// so Serve (below) returns. Callers that manage their own lifecycle
	// (e.g. demo mode) drive this via HttpService.Shutdown instead; running
	// both paths together is safe since Shutdown is idempotent.
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		s.log.Info("shutdown signal received")

		// Bounded shutdown: don't wait forever on lingering connections
		// (e.g. MCP SSE streams or idle keep-alives).
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s1.Shutdown(shutdownCtx) //nolint:errcheck
	}()

	ver := version
	// dep := s.conf.name

	if ver == "" {
		ver = "not-set"
	}

	fields := []zapcore.Field{
		zap.String("version", ver),
		zap.String("host-port", s.conf.hostPort),
		zap.String("app-name", s.conf.AppName),
		zap.String("env", os.Getenv("GO_ENV")),
		// zap.Bool("hot-deploy", s.conf.HotDeploy),
		zap.Bool("production", s.conf.Core.Production),
		zap.String("server", "graphjin-slim"),
	}

	if s.namespace != nil {
		fields = append(fields, zap.String("namespace", *s.namespace))
	}

	// if s.conf.HotDeploy {
	// 	fields = append(fields, zap.String("deployment-name", dep))
	// }

	s.zlog.Info("GraphJin started", fields...)
	printDevModeInfo(s)

	l, err := net.Listen("tcp", s.conf.hostPort)
	if err != nil {
		s.log.Fatalf("failed to init port: %s", err)
	}

	// signal we are open for business.
	s.state = servListening

	if err := srv.Serve(l); err != http.ErrServerClosed {
		s.log.Fatalf("failed to start: %s", err)
	}

	// Serve returned because Shutdown (signal handler above or an external
	// HttpService.Shutdown) was requested. Release the service resources.
	s.closeServResources()
	s.log.Info("shutdown complete")
}

// printDevModeInfo prints useful development information on startup
func printDevModeInfo(s *graphjinService) {
	if s.conf.Serv.Production {
		return
	}

	hostPort := s.conf.hostPort
	displayHost := hostPort
	if strings.HasPrefix(hostPort, "0.0.0.0:") {
		displayHost = "localhost" + hostPort[7:]
	}

	fmt.Println()
	fmt.Println("Development Server URLs")
	fmt.Println("───────────────────────")

	if s.conf.WebUI {
		fmt.Printf("  Web UI:      http://%s/\n", displayHost)
	}
	fmt.Printf("  GraphQL:     http://%s/api/v1/graphql\n", displayHost)
	fmt.Printf("  REST API:    http://%s/api/v1/rest/<name>\n", displayHost)
	fmt.Println()
}
