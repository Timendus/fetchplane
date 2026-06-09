package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/floatplane"
	floatplanesync "github.com/Timendus/fetchplane/internal/floatplane/sync"
	"github.com/Timendus/fetchplane/internal/state"
)

func main() {
	daemon := flag.Bool("daemon", false, "run continuously, polling on FP_POLL_INTERVAL (default 300s)")
	debug := flag.Bool("debug", false, "log HTTP requests and responses")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	if !cfg.Since.IsZero() {
		log.Printf("only downloading videos released after %s", cfg.Since.Format(time.RFC3339))
	}

	st, err := state.Load(cfg.StateFile)
	if err != nil {
		log.Fatalf("state error: %v", err)
	}

	client := floatplane.NewClient()
	client.SetDebug(*debug)
	service := floatplanesync.New(cfg, client, st)

	if *daemon {
		runDaemon(cfg, service, st)
		return
	}

	if err := service.Run(); err != nil {
		log.Fatalf("error: %v", err)
	}
	if err := st.Save(cfg.StateFile); err != nil {
		log.Fatalf("state save error: %v", err)
	}
}

func runDaemon(cfg *config.Config, service *floatplanesync.Service, st *state.State) {
	log.Printf("daemon mode - polling every %ds", cfg.PollInterval)
	ticker := time.NewTicker(time.Duration(cfg.PollInterval) * time.Second)
	defer ticker.Stop()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	for {
		if err := service.Run(); err != nil {
			log.Printf("run error: %v", err)
		}
		if err := st.Save(cfg.StateFile); err != nil {
			log.Printf("state save error: %v", err)
		}

		select {
		case <-ticker.C:
		case sig := <-signals:
			log.Printf("received %s, shutting down", sig)
			return
		}
	}
}
