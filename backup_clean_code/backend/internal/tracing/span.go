package tracing

import (
	"context"

	"gitlab.anyinfra.ru/golang-core/tracer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Span struct{ trace.Span }

func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, Span) {
	ctx, sp := tracer.StartSpan(ctx, name)
	if len(attrs) > 0 {
		sp.SetAttributes(attrs...)
	}
	return ctx, Span{sp}
}

func (s Span) Error(err error) {
	if err == nil || s.Span == nil {
		return
	}
	s.RecordError(err)
	s.SetStatus(codes.Error, err.Error())
}

func Attrs(kv ...attribute.KeyValue) []attribute.KeyValue { return kv }
