package redis

import (
	"context"
	"fmt"
	"net"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func NewClient(ctx context.Context, redisURL string) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}

type LoginRateLimitStore struct {
	client   *goredis.Client
	tracer   trace.Tracer
	peerName string
	peerPort int
}

func NewLoginRateLimitStore(client *goredis.Client) *LoginRateLimitStore {
	return NewLoginRateLimitStoreWithTracer(client, nil)
}

func NewLoginRateLimitStoreWithTracer(client *goredis.Client, tracer trace.Tracer) *LoginRateLimitStore {
	if tracer == nil {
		tracer = otel.Tracer("sessionflow/redis")
	}
	store := &LoginRateLimitStore{
		client: client,
		tracer: tracer,
	}

	if client != nil && client.Options() != nil {
		host, port, err := net.SplitHostPort(client.Options().Addr)
		if err == nil {
			store.peerName = host
			if parsed, perr := parsePort(port); perr == nil {
				store.peerPort = parsed
			}
		} else {
			store.peerName = client.Options().Addr
		}
	}

	return store
}

func (s *LoginRateLimitStore) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	ctx, span := s.tracer.Start(ctx, "redis.ratelimit.increment", trace.WithSpanKind(trace.SpanKindClient))
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "INCR+EXPIRENX"),
	}
	if s.peerName != "" {
		attrs = append(attrs, attribute.String("net.peer.name", s.peerName))
	}
	if s.peerPort > 0 {
		attrs = append(attrs, attribute.Int("net.peer.port", s.peerPort))
	}
	span.SetAttributes(attrs...)
	defer span.End()

	pipeline := s.client.TxPipeline()
	incrCmd := pipeline.Incr(ctx, key)
	pipeline.ExpireNX(ctx, key, window)

	if _, err := pipeline.Exec(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, fmt.Errorf("increment rate limit key: %w", err)
	}

	return incrCmd.Val(), nil
}

func parsePort(raw string) (int, error) {
	var port int
	_, err := fmt.Sscanf(raw, "%d", &port)
	if err != nil {
		return 0, err
	}
	return port, nil
}
