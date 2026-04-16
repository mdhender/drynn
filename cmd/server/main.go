package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/config"
	"github.com/mdhender/drynn/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(drynn.Version().Core())
		return
	}

	configPath := flag.String("config", config.DefaultPath(), "path to the server config file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadPath(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	app, err := server.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
