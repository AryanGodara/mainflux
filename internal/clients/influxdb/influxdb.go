package influxdb

import (
	"context"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/mainflux/mainflux/internal/env"
	"github.com/mainflux/mainflux/pkg/errors"
)

var (
	errConnect = errors.New("failed to create InfluxDB client")
	errConfig  = errors.New("failed to load InfluxDB client configuration from environment variable")
)

type Config struct {
	Protocol           string        `env:"PROTOCOL"              envDefault:"http"`
	Host               string        `env:"HOST"                  envDefault:"localhost"`
	Port               string        `env:"PORT"                  envDefault:"8086"`
	Username           string        `env:"ADMIN_USER"            envDefault:"mainflux"`
	Password           string        `env:"ADMIN_PASSWORD"        envDefault:"mainflux"`
	DbName             string        `env:"DB"                    envDefault:"mainflux"`
	Bucket             string        `env:"BUCKET"                envDefault:"mainflux-bucket"`
	Org                string        `env:"ORG"                   envDefault:"mainflux"`
	Token              string        `env:"TOKEN"                 envDefault:"mainflux-token"`
	DBUrl              string        `env:"DBURL"                   envDefault:""`
	UserAgent          string        `env:"USER_AGENT"            envDefault:"InfluxDBClient"`
	Timeout            time.Duration `env:"TIMEOUT"` // Influxdb client configuration by default there is no timeout duration , this field will not have fallback default timeout duration Reference: https://pkg.go.dev/github.com/influxdata/influxdb@v1.10.0/client/v2#HTTPConfig
	InsecureSkipVerify bool          `env:"INSECURE_SKIP_VERIFY"  envDefault:"false"`
}

// Setup load configuration from environment variable, create InfluxDB client and connect to InfluxDB server
func Setup(envPrefix string, ctx context.Context) (influxdb2.Client, error) {
	config := Config{}
	if err := env.Parse(&config, env.Options{Prefix: envPrefix}); err != nil {
		return nil, errors.Wrap(errConfig, err)
	}
	return Connect(config, ctx)
}

// Connect create InfluxDB client and connect to InfluxDB server
func Connect(config Config, ctx context.Context) (influxdb2.Client, error) {
	client := influxdb2.NewClient(config.DBUrl, config.Token)
	ctx, cancel := context.WithTimeout(ctx, 1000*time.Millisecond)
	defer cancel()
	if _, err := client.Ready(ctx); err != nil {
		return nil, errors.Wrap(errConnect, err)
	}
	return client, nil
}
