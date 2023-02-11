package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/mainflux/mainflux/internal"
	authClient "github.com/mainflux/mainflux/internal/clients/grpc/auth"
	thingsClient "github.com/mainflux/mainflux/internal/clients/grpc/things"
	influxDBClient "github.com/mainflux/mainflux/internal/clients/influxdb"
	"github.com/mainflux/mainflux/internal/env"
	"github.com/mainflux/mainflux/internal/server"
	httpserver "github.com/mainflux/mainflux/internal/server/http"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	influxdata "github.com/influxdata/influxdb-client-go/v2"
	"github.com/mainflux/mainflux"
	authapi "github.com/mainflux/mainflux/auth/api/grpc"
	"github.com/mainflux/mainflux/logger"
	"github.com/mainflux/mainflux/pkg/errors"
	"github.com/mainflux/mainflux/readers"
	"github.com/mainflux/mainflux/readers/api"
	"github.com/mainflux/mainflux/readers/influxdb"
	thingsapi "github.com/mainflux/mainflux/things/api/auth/grpc"
	opentracing "github.com/opentracing/opentracing-go"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	jconfig "github.com/uber/jaeger-client-go/config"
	"golang.org/x/sync/errgroup"
)

const (
	svcName           = "influxdb-reader"
	envPrefix         = "MF_INFLUX_READER_"
	envPrefixHttp     = "MF_INFLUX_READER_HTTP_"
	envPrefixInfluxdb = "MF_INFLUXDB_"
	defSvcHttpPort    = "8180"
)

type config struct {
	LogLevel  string `env:"MF_INFLUX_READER_LOG_LEVEL"  envDefault:"info"`
	JaegerURL string `env:"MF_JAEGER_URL"               envDefault:"localhost:6831"`
}

func main() {
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	stopWaitTime = 5 * time.Second

	defLogLevel          = "error"
	defPort              = "8180"
	defDB                = "mainflux"
	defDBHost            = "localhost"
	defDBPort            = "8086"
	defDBUser            = "mainflux"
	defDBPass            = "mainflux"
	defDBBucket          = "mainflux-bucket"
	defDBOrg             = "mainflux"
	defDBToken           = "mainflux-token"
	defClientTLS         = "false"
	defCACerts           = ""
	defServerCert        = ""
	defServerKey         = ""
	defJaegerURL         = ""
	defThingsAuthURL     = "localhost:8183"
	defThingsAuthTimeout = "1s"
	defUsersAuthURL      = "localhost:8181"
	defUsersAuthTimeout  = "1s"

	envLogLevel          = "MF_INFLUX_READER_LOG_LEVEL"
	envPort              = "MF_INFLUX_READER_PORT"
	envDB                = "MF_INFLUXDB_DB"
	envDBHost            = "MF_INFLUXDB_HOST"
	envDBPort            = "MF_INFLUXDB_PORT"
	envDBUser            = "MF_INFLUXDB_ADMIN_USER"
	envDBPass            = "MF_INFLUXDB_ADMIN_PASSWORD"
	envDBBucket          = "MF_INFLUXDB_BUCKET"
	envDBOrg             = "MF_INFLUXDB_ORG"
	envDBToken           = "MF_INFLUXDB_TOKEN"
	envClientTLS         = "MF_INFLUX_READER_CLIENT_TLS"
	envCACerts           = "MF_INFLUX_READER_CA_CERTS"
	envServerCert        = "MF_INFLUX_READER_SERVER_CERT"
	envServerKey         = "MF_INFLUX_READER_SERVER_KEY"
	envJaegerURL         = "MF_JAEGER_URL"
	envThingsAuthURL     = "MF_THINGS_AUTH_GRPC_URL"
	envThingsAuthTimeout = "MF_THINGS_AUTH_GRPC_TIMEOUT"
	envAuthURL           = "MF_AUTH_GRPC_URL"
	envUsersAuthTimeout  = "MF_AUTH_GRPC_TIMEOUT"
)

type config struct {
	logLevel          string
	port              string
	dbName            string
	dbHost            string
	dbPort            string
	dbUser            string
	dbPass            string
	dbBucket          string
	dbOrg             string
	dbToken           string
	dbUrl             string
	clientTLS         bool
	caCerts           string
	serverCert        string
	serverKey         string
	jaegerURL         string
	thingsAuthURL     string
	usersAuthURL      string
	thingsAuthTimeout time.Duration
	usersAuthTimeout  time.Duration
}

func main() {
	cfg, repoCfg := loadConfigs()
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	logger, err := logger.New(os.Stdout, cfg.logLevel)
	if err != nil {
		log.Fatalf(err.Error())
	}
	conn := connectToThings(cfg, logger)
	defer conn.Close()

	tc, tcHandler, err := thingsClient.Setup(envPrefix, cfg.JaegerURL)
	thingsTracer, thingsCloser := initJaeger("things", cfg.jaegerURL, logger)
	defer thingsCloser.Close()

	tc := thingsapi.NewClient(conn, thingsTracer, cfg.thingsAuthTimeout)

	authTracer, authCloser := initJaeger("auth", cfg.jaegerURL, logger)
	defer authCloser.Close()

	authConn := connectToAuth(cfg, logger)
	defer authConn.Close()

	auth := authapi.NewClient(authTracer, authConn, cfg.usersAuthTimeout)

	client, err := connectToInfluxDB(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer tcHandler.Close()
	logger.Info("Successfully connected to things grpc server " + tcHandler.Secure())

	auth, authHandler, err := authClient.Setup(envPrefix, cfg.JaegerURL)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer authHandler.Close()
	logger.Info("Successfully connected to auth grpc server " + authHandler.Secure())

	influxDBConfig := influxDBClient.Config{}
	if err := env.Parse(&influxDBConfig, env.Options{Prefix: envPrefixInfluxdb}); err != nil {
		log.Fatalf("failed to load InfluxDB client configuration from environment variable : %s", err.Error())
	}
	influxDBConfig.DBUrl = fmt.Sprintf("%s://%s:%s", influxDBConfig.Protocol, influxDBConfig.Host, influxDBConfig.Port)

	repocfg := influxdb.RepoConfig{
		Bucket: influxDBConfig.Bucket,
		Org:    influxDBConfig.Org,
	}

	client, err := influxDBClient.Connect(influxDBConfig)
	if err != nil {
		log.Fatalf("failed to connect to InfluxDB : %s", err.Error())
		logger.Error(fmt.Sprintf("Failed to create InfluxDB client: %s", err))
		os.Exit(1)
	}
	defer client.Close()

	repo := newService(client, repocfg, logger)

	httpServerConfig := server.Config{Port: defSvcHttpPort}
	if err := env.Parse(&httpServerConfig, env.Options{Prefix: envPrefixHttp, AltPrefix: envPrefix}); err != nil {
		log.Fatalf("failed to load %s HTTP server configuration : %s", svcName, err.Error())
	}
	hs := httpserver.New(ctx, cancel, svcName, httpServerConfig, api.MakeHandler(repo, tc, auth, svcName, logger), logger)
	repo := newService(client, repoCfg, logger)

	g.Go(func() error {
		return startHTTPServer(ctx, repo, tc, auth, cfg, logger)
	})

	g.Go(func() error {
		if sig := errors.SignalHandler(ctx); sig != nil {
			cancel()
			logger.Info(fmt.Sprintf("InfluxDB reader service shutdown by signal: %s", sig))
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		logger.Error(fmt.Sprintf("InfluxDB reader service terminated: %s", err))
	}
}

func newService(client influxdb2.Client, repocfg influxdb.RepoConfig, logger logger.Logger) readers.MessageRepository {
	repo := influxdb.New(client, repocfg)
func connectToAuth(cfg config, logger logger.Logger) *grpc.ClientConn {
	var opts []grpc.DialOption
	logger.Info("Connecting to auth via gRPC")
	if cfg.clientTLS {
		if cfg.caCerts != "" {
			tpc, err := credentials.NewClientTLSFromFile(cfg.caCerts, "")
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to create tls credentials: %s", err))
				os.Exit(1)
			}
			opts = append(opts, grpc.WithTransportCredentials(tpc))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		logger.Info("gRPC communication is not encrypted")
	}

	conn, err := grpc.Dial(cfg.usersAuthURL, opts...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to connect to auth service: %s", err))
		os.Exit(1)
	}
	logger.Info("Established gRPC connection to auth via gRPC")
	return conn
}

func connectToInfluxDB(cfg config) (influxdata.Client, error) {
	client := influxdata.NewClient(cfg.dbUrl, cfg.dbToken)
	_, err := client.Ready(context.Background())
	return client, err
}

func loadConfigs() (config, influxdb.RepoConfig) {
	tls, err := strconv.ParseBool(mainflux.Env(envClientTLS, defClientTLS))
	if err != nil {
		log.Fatalf("Invalid value passed for %s\n", envClientTLS)
	}

	authTimeout, err := time.ParseDuration(mainflux.Env(envThingsAuthTimeout, defThingsAuthTimeout))
	if err != nil {
		log.Fatalf("Invalid %s value: %s", envThingsAuthTimeout, err.Error())
	}

	userAuthTimeout, err := time.ParseDuration(mainflux.Env(envUsersAuthTimeout, defUsersAuthTimeout))
	if err != nil {
		log.Fatalf("Invalid %s value: %s", envThingsAuthTimeout, err.Error())
	}

	cfg := config{
		logLevel:          mainflux.Env(envLogLevel, defLogLevel),
		port:              mainflux.Env(envPort, defPort),
		dbName:            mainflux.Env(envDB, defDB),
		dbHost:            mainflux.Env(envDBHost, defDBHost),
		dbPort:            mainflux.Env(envDBPort, defDBPort),
		dbUser:            mainflux.Env(envDBUser, defDBUser),
		dbPass:            mainflux.Env(envDBPass, defDBPass),
		dbBucket:          mainflux.Env(envDBBucket, defDBBucket),
		dbOrg:             mainflux.Env(envDBOrg, defDBOrg),
		dbToken:           mainflux.Env(envDBToken, defDBToken),
		clientTLS:         tls,
		caCerts:           mainflux.Env(envCACerts, defCACerts),
		serverCert:        mainflux.Env(envServerCert, defServerCert),
		serverKey:         mainflux.Env(envServerKey, defServerKey),
		jaegerURL:         mainflux.Env(envJaegerURL, defJaegerURL),
		thingsAuthURL:     mainflux.Env(envThingsAuthURL, defThingsAuthURL),
		thingsAuthTimeout: authTimeout,
		usersAuthURL:      mainflux.Env(envAuthURL, defUsersAuthURL),
		usersAuthTimeout:  userAuthTimeout,
	}

	cfg.dbUrl = fmt.Sprintf("http://%s:%s", cfg.dbHost, cfg.dbPort)

	repoCfg := influxdb.RepoConfig{
		Bucket: cfg.dbBucket,
		Org:    cfg.dbOrg,
	}
	return cfg, repoCfg
}

func connectToThings(cfg config, logger logger.Logger) *grpc.ClientConn {
	var opts []grpc.DialOption
	logger.Info("connecting to things via gRPC")
	if cfg.clientTLS {
		if cfg.caCerts != "" {
			tpc, err := credentials.NewClientTLSFromFile(cfg.caCerts, "")
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to load certs: %s", err))
				os.Exit(1)
			}
			opts = append(opts, grpc.WithTransportCredentials(tpc))
		}
	} else {
		logger.Info("gRPC communication is not encrypted")
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(cfg.thingsAuthURL, opts...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to connect to things service: %s", err))
		os.Exit(1)
	}
	logger.Info(fmt.Sprintf("Established gRPC connection to things via gRPC: %s", cfg.thingsAuthURL))
	return conn
}

func initJaeger(svcName, url string, logger logger.Logger) (opentracing.Tracer, io.Closer) {
	if url == "" {
		return opentracing.NoopTracer{}, ioutil.NopCloser(nil)
	}

	tracer, closer, err := jconfig.Configuration{
		ServiceName: svcName,
		Sampler: &jconfig.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jconfig.ReporterConfig{
			LocalAgentHostPort: url,
			LogSpans:           true,
		},
	}.NewTracer()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to init Jaeger client: %s", err))
		os.Exit(1)
	}

	return tracer, closer
}

func newService(client influxdata.Client, repoCfg influxdb.RepoConfig, logger logger.Logger) readers.MessageRepository {
	repo := influxdb.New(client, repoCfg)
	repo = api.LoggingMiddleware(repo, logger)
	repo = api.MetricsMiddleware(
		repo,
		kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "influxdb",
			Subsystem: "message_reader",
			Name:      "request_count",
			Help:      "Number of requests received.",
		}, []string{"method"}),
		kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "influxdb",
			Subsystem: "message_reader",
			Name:      "request_latency_microseconds",
			Help:      "Total duration of requests in microseconds.",
		}, []string{"method"}),
	)

	return repo
}

func startHTTPServer(ctx context.Context, repo readers.MessageRepository, tc mainflux.ThingsServiceClient, ac mainflux.AuthServiceClient, cfg config, logger logger.Logger) error {
	p := fmt.Sprintf(":%s", cfg.port)
	errCh := make(chan error)
	server := &http.Server{Addr: p, Handler: api.MakeHandler(repo, tc, ac, "influxdb-reader", logger)}
	switch {
	case cfg.serverCert != "" || cfg.serverKey != "":
		logger.Info(fmt.Sprintf("InfluxDB reader service started using https on port %s with cert %s key %s",
			cfg.port, cfg.serverCert, cfg.serverKey))
		go func() {
			errCh <- server.ListenAndServeTLS(cfg.serverCert, cfg.serverKey)
		}()
	default:
		logger.Info(fmt.Sprintf("InfluxDB reader service started, exposed port %s", cfg.port))
		go func() {
			errCh <- server.ListenAndServe()
		}()
	}

	select {
	case <-ctx.Done():
		ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), stopWaitTime)
		defer cancelShutdown()
		if err := server.Shutdown(ctxShutdown); err != nil {
			logger.Error(fmt.Sprintf("InfluxDB reader service error occurred during shutdown at %s: %s", p, err))
			return fmt.Errorf("influxDB reader service occurred during shutdown at %s: %w", p, err)
		}
		logger.Info(fmt.Sprintf("InfluxDB reader service  shutdown of http at %s", p))
		return nil
	case err := <-errCh:
		return err
	}
}
