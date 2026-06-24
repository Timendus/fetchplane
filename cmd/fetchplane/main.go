package main

import (
	"flag"
	"log"

	"github.com/Timendus/fetchplane/internal/config"
	"github.com/Timendus/fetchplane/internal/floatplane"
	"github.com/Timendus/fetchplane/internal/floatplane/api"
	"github.com/Timendus/fetchplane/internal/state"
	"github.com/Timendus/fetchplane/internal/webui"
)

func main() {
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

	client := api.NewClient()
	client.SetDebug(*debug)
	service := floatplane.New(cfg, client, st)
	service.Start()

	server, err := webui.New(cfg, service)
	if err != nil {
		log.Fatalf("web server error: %v", err)
	}

	log.Printf("web UI listening on %s", cfg.Listen)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("web server error: %v", err)
	}
}
