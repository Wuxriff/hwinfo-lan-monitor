// Command hwinfo-lan-monitor mirrors the HWiNFO sensors window to a web Dashboard on the local network.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/app"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/config"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/gui"
)

func main() {
	headless := flag.Bool("headless", false, "run without the window, logging to the console")
	addr := flag.String("addr", "", "listen address for -headless (default from config.json)")
	flag.Parse()

	cfgPath := config.Path()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("config: %v (using defaults)", err)
	}

	a := app.New()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go a.Run(ctx)

	if !*headless {
		os.Exit(gui.Run(a, cfg, cfgPath))
	}

	listen := cfg.Addr()
	if *addr != "" {
		listen = *addr
	}
	if err := a.Listen(listen); err != nil {
		log.Fatal(err)
	}
	log.Printf("Dashboard: %v", a.URLs())
	<-ctx.Done()
	a.Close()
}
