package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/ocagent"
	"github.com/buoyantio/emojivoto/emojivoto-emoji-svc/api"
	"github.com/buoyantio/emojivoto/emojivoto-emoji-svc/cmd/options"
	"github.com/buoyantio/emojivoto/emojivoto-emoji-svc/emoji"
	"github.com/buoyantio/emojivoto/emojivoto-emoji-svc/utils/cache"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
)

var (
	grpcPort    = os.Getenv("GRPC_PORT")
	promPort    = os.Getenv("PROM_PORT")
	ocagentHost = os.Getenv("OC_AGENT_HOST")

	redisPort     = os.Getenv("REDIS_PORT")
	redisPassword = os.Getenv("REDIS_PASSWORD")
	redisHost     = os.Getenv("REDIS_HOST")
)

func main() {

	options.UseRedis = os.Getenv("USE_REDIS") == "ON"

	if grpcPort == "" {
		log.Fatalf("GRPC_PORT (currently [%s]) environment variable must me set to run the server.", grpcPort)
	}

	if options.UseRedis {
		if redisPort == "" {
			redisPort = "6379"
		}
		err := cache.InitCache(fmt.Sprintf("%s:%s", redisHost, redisPort),
			redisPassword, 0, 120)
		if err != nil {
			log.Fatalf("Failed to initialize redis client: %s", err)
		}

	}

	oce, err := ocagent.NewExporter(
		ocagent.WithInsecure(),
		ocagent.WithReconnectionPeriod(5*time.Second),
		ocagent.WithAddress(ocagentHost),
		ocagent.WithServiceName("voting"))
	if err != nil {
		log.Fatalf("Failed to create ocagent-exporter: %v", err)
	}
	trace.RegisterExporter(oce)

	allEmoji := emoji.NewAllEmoji()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		panic(err)
	}

	errs := make(chan error, 1)

	if promPort != "" {
		// Start prometheus server
		go func() {
			log.Printf("Starting prom metrics on PROM_PORT=[%s]", promPort)
			http.Handle("/metrics", promhttp.Handler())
			err := http.ListenAndServe(fmt.Sprintf(":%s", promPort), nil)
			errs <- err
		}()
	}

	// Start grpc server
	go func() {
		grpc_prometheus.EnableHandlingTimeHistogram()
		grpcServer := grpc.NewServer(
			grpc.StatsHandler(&ocgrpc.ServerHandler{}),
			grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
			grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		)
		api.NewGrpServer(grpcServer, allEmoji)
		log.Printf("Starting grpc server on GRPC_PORT=[%s]", grpcPort)
		err := grpcServer.Serve(lis)
		errs <- err
	}()

	// Catch shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGQUIT)
		s := <-sig
		errs <- fmt.Errorf("caught signal %v", s)
	}()

	log.Fatal(<-errs)
}
