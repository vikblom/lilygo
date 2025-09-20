package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/activation"
	slogmulti "github.com/samber/slog-multi"
	"github.com/vikblom/lilygo/pkg/api"
	"github.com/vikblom/lilygo/pkg/db"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	_ "embed"
)

func configureOtel(ctx context.Context) error {
	// Create resource.
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName("lilygo"),
			semconv.ServiceVersion("0.1.0"),
		),
	)
	if err != nil {
		return fmt.Errorf("resources: %w", err)
	}

	// LOGS
	//
	// Create a logger provider.
	// You can pass this instance directly when creating bridges.
	logExporter, err := otlploghttp.New(ctx, otlploghttp.WithInsecure())
	if err != nil {
		return fmt.Errorf("otlploghttp: %w", err)
	}
	lp := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	)
	// Handle shutdown properly so nothing leaks.
	go func() {
		<-ctx.Done()
		if err := lp.Shutdown(context.Background()); err != nil {
			slog.Warn("log provider shutdown", "error", err)
		}
	}()
	// Use it with SLOG.
	global.SetLoggerProvider(lp)
	logger := slog.New(
		slogmulti.Fanout(
			otelslog.NewHandler("lilygo", otelslog.WithLoggerProvider(lp)),
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}),
			// ...
		),
	)
	slog.SetDefault(logger)

	// METRICS
	//
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithInsecure())
	if err != nil {
		return fmt.Errorf("otlpmetrichttp: %w", err)
	}
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)
	go func() {
		<-ctx.Done()
		if err := mp.Shutdown(context.Background()); err != nil {
			slog.Warn("metric provider shutdown", "error", err)
		}
	}()
	// Baseline metrics of the Go runtime.
	err = runtime.Start()
	if err != nil {
		return fmt.Errorf("runtime metrics: %w", err)
	}
	otel.SetMeterProvider(mp)

	return nil
}

func run(ctx context.Context) error {
	err := configureOtel(ctx)
	if err != nil {
		return fmt.Errorf("configure otel: %w", err)
	}

	sha := "(dev)"
	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, v := range info.Settings {
			if v.Key == "vcs.revision" {
				sha = v.Value
			}
		}
	}
	slog.Info(fmt.Sprintf("lilygo: %s", sha))

	// Let systemd juggle sockets during service restarts.
	// https://vincent.bernat.ch/en/blog/2018-systemd-golang-socket-activation
	var l net.Listener
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		// Run by systemd.
		slog.Info("systemd listener")
		listeners, err := activation.Listeners()
		if err != nil {
			return err
		}
		if len(listeners) != 1 {
			return fmt.Errorf("unexpected number of socket activation (%d != 1)", len(listeners))
		}
		l = listeners[0]
	} else {
		// Running locally.
		slog.Info("net listener")
		l, err = net.Listen("tcp", ":9000")
		if err != nil {
			return err
		}
	}

	db, err := db.New("db.sqlite")
	if err != nil {
		return err
	}

	api, err := api.New(db)
	if err != nil {
		return err
	}

	srv := http.Server{
		Handler:  api,
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelDebug),
	}
	go func() {
		<-ctx.Done()
		slog.Info("shutdown")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.SetKeepAlivesEnabled(false)
		err := srv.Shutdown(ctx)
		if err != nil {
			slog.Info("force shutdown")
			srv.Close()
		}
	}()
	err = srv.Serve(l)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return ctx.Err()
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()

	err := run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error(fmt.Sprintf("run: %s", err.Error()))
	}
}
