package config

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	RabbitMQ    RabbitMQConfig    `mapstructure:"rabbitmq"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Browserless BrowserlessConfig `mapstructure:"browserless"`
	S3          S3Config          `mapstructure:"s3"`
	YouTube     YouTubeConfig     `mapstructure:"youtube"`
	ScrapingAnt ScrapingAntConfig `mapstructure:"scrapingant"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Env  string `mapstructure:"env"`
}

type RabbitMQConfig struct {
	BrokerLink    string `mapstructure:"broker_link"`
	ExchangeName  string `mapstructure:"exchange_name"`
	ExchangeType  string `mapstructure:"exchange_type"`
	QueueName     string `mapstructure:"queue_name"`
	RoutingKey    string `mapstructure:"routing_key"`
	WorkerCount   int    `mapstructure:"worker_count"`
	PrefetchCount int    `mapstructure:"prefetch_count"`
}

type RedisConfig struct {
	URL string `mapstructure:"url"`
}

type BrowserlessConfig struct {
	URL          string `mapstructure:"url"`
	Token        string `mapstructure:"token"`
	RenderWaitMs int    `mapstructure:"render_wait_ms"` // settle delay (ms) after the wait condition before capture; 0 = client default
	NavTimeoutMs int    `mapstructure:"nav_timeout_ms"` // upper bound (ms) on goto; after it we fall back to a load capture; 0 = client default
}

type S3Config struct {
	Region     string `mapstructure:"region"`
	Endpoint   string `mapstructure:"endpoint"`
	AccessKey  string `mapstructure:"access_key"`
	SecretKey  string `mapstructure:"secret_key"`
	BucketName string `mapstructure:"bucket"`
}

type YouTubeConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// ScrapingAntConfig — the residential-proxy / JS-render fallback provider. Empty
// api_key disables the Reddit path and the proxy last-resort fallback.
type ScrapingAntConfig struct {
	APIKey string `mapstructure:"api_key"`
}
