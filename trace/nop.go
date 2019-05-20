package trace

import (
	"context"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

type nopTracer struct{}

func (nopTracer) StartSpan(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, &nopSpan{}
}

type nopSpan struct{}

func (nopSpan) End()               {}
func (nopSpan) SetStatus(error)    {}
func (nopSpan) Logger() log.Logger { return nil }

func (nopSpan) WithField(ctx context.Context, _ string, _ interface{}) context.Context { return ctx }
func (nopSpan) WithFields(ctx context.Context, _ log.Fields) context.Context           { return ctx }
