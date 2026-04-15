package dedup

import (
	"context"
	"log/slog"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/metrics"

	"github.com/redis/go-redis/v9"
)

const (
	defaultTTL    = 5 * time.Minute
	defaultPrefix = "dedup:"
)

type Config struct {
	TTL    time.Duration
	Prefix string
}

func Middleware(client *redis.Client, cfg Config, logger *slog.Logger, metricSet *metrics.Metrics) channel.UpdateMiddleware {
	if cfg.TTL == 0 {
		cfg.TTL = defaultTTL
	}
	if cfg.Prefix == "" {
		cfg.Prefix = defaultPrefix
	}

	return func(next channel.UpdateHandlerFunc) channel.UpdateHandlerFunc {
		return func(ctx context.Context, u channel.Update) error {
			if u.PlatformUpdateID == "" {
				return next(ctx, u)
			}

			key := cfg.Prefix + u.PlatformUpdateID

			ok, err := client.SetNX(ctx, key, "1", cfg.TTL).Result()
			if err != nil {
				if metricSet != nil {
					metricSet.DedupChecksTotal.WithLabelValues(string(u.ChannelType), "redis_error").Inc()
				}
				logger.Warn("dedup: redis error, processing anyway",
					slog.String("update_id", u.PlatformUpdateID),
					slog.Any("error", err))
				return next(ctx, u)
			}
			if !ok {
				if metricSet != nil {
					metricSet.DedupChecksTotal.WithLabelValues(string(u.ChannelType), "duplicate").Inc()
				}
				logger.Debug("dedup: duplicate update skipped",
					slog.String("update_id", u.PlatformUpdateID))
				return nil
			}

			if metricSet != nil {
				metricSet.DedupChecksTotal.WithLabelValues(string(u.ChannelType), "new").Inc()
			}
			return next(ctx, u)
		}
	}
}
