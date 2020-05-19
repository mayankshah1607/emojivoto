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

	"github.com/buoyantio/emojivoto/emojivoto-voting-svc/api"
	"github.com/buoyantio/emojivoto/emojivoto-voting-svc/cmd/options"
	"github.com/buoyantio/emojivoto/emojivoto-voting-svc/utils/mysql"
	"github.com/buoyantio/emojivoto/emojivoto-voting-svc/voting"

	"contrib.go.opencensus.io/exporter/ocagent"
	_ "github.com/go-sql-driver/mysql"
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

	// MySQL configurations
	mysqlPort = os.Getenv("MYSQL_PORT")
	mysqlHost = os.Getenv("MYSQL_HOST")
	mysqlUser = os.Getenv("MYSQL_USER")
	mysqlPass = os.Getenv("MYSQL_PASS")
)

func main() {
	// initialize feature options here
	options.UseMySQL = os.Getenv("USE_MYSQL") == "ON"

	if grpcPort == "" {
		log.Fatalf("GRPC_PORT (currently [%s]) environment variable must me set to run the server.", grpcPort)
	}

	if options.UseMySQL {
		if mysqlPort == "" {
			mysqlPort = "3306" // default
		}

		if mysqlHost == "" {
			log.Fatalf("MYSQL_HOST environment variable must be set to connect to an instance of MySQL server")
		}

		if err := mysql.InitDB(mysqlPort, mysqlHost, mysqlUser, mysqlPass); err != nil {
			log.Fatal(err)
		}

		log.Println("Successfully established connection to MySQL server")

		if err := mysql.InitTables(); err != nil {
			log.Fatalf("Error creating votes table: %s", err)
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

	poll := voting.NewPoll()

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
		api.NewGrpServer(grpcServer, poll)
		grpc_prometheus.Register(grpcServer)
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
