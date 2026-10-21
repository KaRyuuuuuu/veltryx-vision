package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/veltryx/vision/apps/encoder"
	"github.com/veltryx/vision/internal/camera"
)

func main() {
	addr := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	simulate := flag.Int("simulate", 0, "explicit number of simulated cameras (0 uses Aravis)")
	configPath := flag.String("config", "local-data/config.json", "persistent JSON configuration")
	data := flag.String("screenshots", "local-data/screenshots", "capture directory")
	preset := flag.String("preset", "veryfast", "x264 CPU preset")
	bitrate := flag.Int("bitrate", 2500, "H.264 kbps per camera")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	var p camera.Provider
	var err error
	if *simulate < 0 || *simulate > 32 {
		slog.Error("simulate must be between 0 and 32")
		os.Exit(1)
	}
	if *simulate > 0 {
		p = camera.Simulation{Count: *simulate}
		slog.Warn("simulation mode enabled", "cameras", *simulate)
	} else {
		p, err = camera.NewAravis()
		if err != nil {
			slog.Error("camera initialization", "error", err)
			os.Exit(1)
		}
	}
	ec := encoder.Default()
	ec.Preset = *preset
	ec.BitrateKbps = *bitrate
	if err = ec.Validate(); err != nil {
		slog.Error("encoder configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := NewServer(ctx, p, *configPath, *data, ec)
	if err != nil {
		slog.Error("initialization", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: *addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	go func() {
		slog.Info("HTTP listening", "address", *addr)
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			slog.Error("HTTP server", "error", e)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	app.Close()
}
