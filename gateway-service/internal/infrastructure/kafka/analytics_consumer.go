package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
	kafkago "github.com/segmentio/kafka-go"
)

type VerificationEventHandler interface {
	HandleVerificationEvent(ctx context.Context, event *model.VerificationEvent) error
}

type VerificationEventConsumer struct {
	reader  *kafkago.Reader
	handler VerificationEventHandler
	logger  *slog.Logger
}

func NewVerificationEventConsumer(config *Config, handler VerificationEventHandler, logger *slog.Logger) *VerificationEventConsumer {
	return &VerificationEventConsumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: config.Brokers,
			GroupID: config.VerificationEventsGroup,
			Topic:   config.VerificationEventsTopic,
			MaxWait: 500 * time.Millisecond,
		}),
		handler: handler,
		logger:  logger,
	}
}

func (c *VerificationEventConsumer) Start(ctx context.Context) {
	c.logger.Info("starting kafka verification analytics consumer")

	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("stopping kafka verification analytics consumer", "reason", ctx.Err())
				return
			}

			c.logger.Error("failed to fetch kafka verification analytics message", "error", err)
			continue
		}

		var event model.VerificationEvent
		if err := json.Unmarshal(message.Value, &event); err != nil {
			c.logger.Error("failed to decode verification analytics event", "error", err)
			if commitErr := c.reader.CommitMessages(ctx, message); commitErr != nil {
				c.logger.Error("failed to commit malformed verification analytics message", "error", commitErr)
			}
			continue
		}

		if err := c.handler.HandleVerificationEvent(ctx, &event); err != nil {
			c.logger.Error("failed to persist verification analytics event", "event_id", event.EventID, "error", err)
			continue
		}

		if err := c.reader.CommitMessages(ctx, message); err != nil {
			c.logger.Error("failed to commit verification analytics message", "error", err)
		}
	}
}

func (c *VerificationEventConsumer) Close() error {
	return c.reader.Close()
}
