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
	daemon := flag.Bool("daemon", false, "run continuously, polling on pollInterval from config.yaml (default 600s)")
	debug := flag.Bool("debug", false, "log HTTP requests and responses")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	if cfg.MaxDaysToDownload > 0 {
		log.Printf("only downloading missing videos from the last %d day(s)", cfg.MaxDaysToDownload)
	}
	if cfg.RemoveOlderThanDays > 0 {
		log.Printf("removing managed videos older than %d day(s)", cfg.RemoveOlderThanDays)
	}

	st, err := state.Load(cfg.SessionFile)
	if err != nil {
		log.Fatalf("session error: %v", err)
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
	if err := st.Save(cfg.SessionFile); err != nil {
		log.Fatalf("session save error: %v", err)
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
		if err := st.Save(cfg.SessionFile); err != nil {
			log.Printf("session save error: %v", err)
		}

		select {
		case <-ticker.C:
		case sig := <-signals:
			log.Printf("received %s, shutting down", sig)
			return
		}
	}
}
