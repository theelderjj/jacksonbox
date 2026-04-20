// Command server is the trivia engine entrypoint. See docs/DESIGN.md §16.
package main

import (
	"context"
	"flag"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jj/trivia/internal/auth"
	"github.com/jj/trivia/internal/games/drawful"
	"github.com/jj/trivia/internal/gateway"
	"github.com/jj/trivia/internal/obs"
	"github.com/jj/trivia/internal/room"
)

func main() {
	var (
		addr     = flag.String("addr", ":8080", "HTTP listen address")
		logLevel = flag.String("log", "info", "log level: debug|info|warn|error")
		seed     = flag.Int64("seed", time.Now().UnixNano(), "prompt rng seed")
	)
	flag.Parse()

	log := obs.New(*logLevel)
	drawful.Register()
	drawful.Seed(*seed)

	store := auth.NewStore()
	rng := rand.New(rand.NewSource(*seed))

	mgr, err := room.NewManager(room.Config{
		Log:     log,
		Store:   store,
		Builder: drawful.BuildPhases,
		Defaults: room.Defaults{
			ApplyFor:         drawful.DisconnectDefaults(rng),
			BuildRevealSteps: drawful.BuildRevealSteps,
			RerollPrompt:     drawful.RerollPrompt,
		},
	})
	if err != nil {
		log.Error("manager init failed", "err", err)
		os.Exit(1)
	}

	gw := gateway.New(mgr, log, gateway.DefaultConfig())

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		// Readiness is trivial here — the manager is ready as soon as NewManager returns.
		// When room crashes mark themselves Closed, /readyz stays 200 because the
		// process can still serve new rooms; a sibling /rooms endpoint would be
		// the place to report per-room health.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/ws", gw.Handle)

	// Periodic auth sweep (token TTL cleanup). Cheap, every minute.
	stopSweep := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-stopSweep:
				return
			case <-t.C:
				n := store.Sweep(time.Now())
				if n > 0 {
					log.Debug("auth sweep", "evicted", n)
				}
			}
		}
	}()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		log.Info("listening", "addr", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server exit", "err", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutdown requested")

	// §12: target 5s graceful shutdown.
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	close(stopSweep)
	mgr.Shutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", "err", err)
	}
	log.Info("bye")
}
