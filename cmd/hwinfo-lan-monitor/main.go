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
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/demo"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/gui"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm"
)

func main() {
	headless := flag.Bool("headless", false, "run without the window, logging to the console")
	addr := flag.String("addr", "", "listen address for -headless (default from config.json)")
	demoMode := flag.String("demo", "", "show synthetic sensors instead of HWiNFO: light or dark (for screenshots)")
	flag.Parse()

	cfgPath := config.Path()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("config: %v (using defaults)", err)
	}

	read := shm.ReadRaw
	opts := gui.Options{}
	if *demoMode != "" {
		read = demo.New().Read
		opts = gui.Options{Demo: true, Dark: *demoMode == "dark"}
	}
	a := app.New(read)
	if opts.Demo {
		a.FixedURLs = []string{"http://192.168.1.50:8086", "http://localhost:8086"}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go a.Run(ctx)

	if !*headless {
		os.Exit(gui.Run(a, cfg, cfgPath, opts))
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
