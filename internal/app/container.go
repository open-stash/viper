package app

import (
	"context"
	"net/http"

	"github.com/open-stash/viper/config"
	infraBrowserless "github.com/open-stash/viper/internal/infra/browserless"
	infraYouTube "github.com/open-stash/viper/internal/infra/youtube"
	"github.com/open-stash/viper/internal/modules/scrape"
	"github.com/open-stash/viper/internal/modules/scrape/engine"
	"github.com/open-stash/viper/pkg/browserless"
	"github.com/open-stash/viper/pkg/mq"
	"github.com/open-stash/viper/pkg/redis"
	"github.com/open-stash/viper/pkg/s3"
	"github.com/open-stash/viper/pkg/youtube"
	"github.com/rabbitmq/amqp091-go"
)

type Container struct {
	ScrapeHandler *scrape.Handler
	consumer      *mq.Consumer
	RMQConn       *amqp091.Connection
	ScrapeWk      *scrape.ScrapeWorker
}

func NewContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	rmqpConn, consumer, err := setupRabbitMQ(&cfg.RabbitMQ)
	if err != nil {
		return nil, err
	}

	rds, err := redis.New(cfg.Redis.URL)
	if err != nil {
		return nil, err
	}

	pbh, err := mq.NewPublisher(rmqpConn, cfg.RabbitMQ.ExchangeName, cfg.RabbitMQ.RoutingKey)
	if err != nil {
		return nil, err
	}

	ytS := youtube.NewClient(cfg.YouTube.APIKey)
	browserClient := browserless.New(cfg.Browserless.URL, cfg.Browserless.Token, cfg.Browserless.RenderWaitMs, cfg.Browserless.NavTimeoutMs)
	s3Client, err := s3.NewClient(ctx, cfg.S3)
	if err != nil {
		return nil, err
	}

	browserAdapter := infraBrowserless.NewAdapter(browserClient)
	youtubeAdapter := infraYouTube.NewAdapter(ytS)

	scrapS := engine.New(browserAdapter, s3Client, youtubeAdapter, &http.Client{})

	scrapeWorker := scrape.NewScrapeWorker(rds, scrapS)
	scrapeService := scrape.NewService(rds, pbh)
	scrapeHandler := scrape.NewHandler(scrapeService)

	return &Container{
		ScrapeHandler: scrapeHandler,
		consumer:      consumer,
		RMQConn:       rmqpConn,
		ScrapeWk:      scrapeWorker,
	}, nil
}

func setupRabbitMQ(cfg *config.RabbitMQConfig) (*amqp091.Connection, *mq.Consumer, error) {
	rmqpConn, err := mq.NewConn(cfg)
	if err != nil {
		return nil, nil, err
	}
	if err := mq.SetupTopology(rmqpConn, cfg); err != nil {
		return nil, nil, err
	}
	consumer, err := mq.NewConsumer(rmqpConn, cfg.QueueName, cfg.PrefetchCount)
	if err != nil {
		return nil, nil, err
	}
	return rmqpConn, consumer, nil
}

func (c *Container) Shutdown(ctx context.Context) error {
	if c.consumer != nil {
		_ = c.consumer.Shutdown(ctx)
	}
	if c.RMQConn != nil {
		_ = c.RMQConn.Close()
	}
	return nil
}
