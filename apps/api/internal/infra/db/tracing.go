package db

import (
	"context"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type PoolTracingConfig struct {
	Tracer             trace.Tracer
	DBStatementEnabled bool
}

type queryTracer struct {
	tracer             trace.Tracer
	dbName             string
	dbStatementEnabled bool
}

func newQueryTracer(databaseURL string, cfg PoolTracingConfig) *queryTracer {
	return &queryTracer{
		tracer:             cfg.Tracer,
		dbName:             parseDatabaseName(databaseURL),
		dbStatementEnabled: cfg.DBStatementEnabled,
	}
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if t.tracer == nil {
		return ctx
	}

	ctx, span := t.tracer.Start(ctx, "postgres.query", trace.WithSpanKind(trace.SpanKindClient))
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "postgresql"),
	}
	if t.dbName != "" {
		attrs = append(attrs, attribute.String("db.name", t.dbName))
	}
	if t.dbStatementEnabled {
		attrs = append(attrs, attribute.String("db.statement", sanitizeStatement(data.SQL)))
	}
	span.SetAttributes(attrs...)
	return context.WithValue(ctx, traceSpanContextKey{}, span)
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, ok := ctx.Value(traceSpanContextKey{}).(trace.Span)
	if !ok || span == nil {
		return
	}
	defer span.End()

	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
	}
}

type traceSpanContextKey struct{}

func parseDatabaseName(databaseURL string) string {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return ""
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	return name
}

func sanitizeStatement(sql string) string {
	sql = strings.Join(strings.Fields(sql), " ")
	if len(sql) > 512 {
		return sql[:512]
	}
	return sql
}
