package api

import (
	"context"
	"testing"
	"time"
)

func TestContainerExecContext_CancelCancelsHandlerContext(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	gotCtx := make(chan context.Context, 1)

	c := &containerExecContext{
		ctx:       parent,
		cancel:    parentCancel,
		namespace: "ns",
		pod:       "pod",
		container: "c",
		h: func(ctx context.Context, namespace, podName, containerName string, cmd []string, attach AttachIO) error {
			gotCtx <- ctx
			return nil
		},
	}

	if err := c.ExecInContainer("", "", "c", []string{"true"}, nil, nil, nil, false, nil, 0); err != nil {
		t.Fatalf("ExecInContainer returned error: %v", err)
	}

	handlerCtx := <-gotCtx
	if handlerCtx == nil {
		t.Fatal("handler received nil context")
	}

	select {
	case <-handlerCtx.Done():
		t.Fatal("ctx cancelled prematurely")
	default:
	}

	c.Cancel()

	select {
	case <-handlerCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("handler context not cancelled after Cancel()")
	}
}

func TestContainerAttachContext_CancelCancelsHandlerContext(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	gotCtx := make(chan context.Context, 1)

	c := &containerAttachContext{
		ctx:       parent,
		cancel:    parentCancel,
		namespace: "ns",
		pod:       "pod",
		container: "c",
		h: func(ctx context.Context, namespace, podName, containerName string, attach AttachIO) error {
			gotCtx <- ctx
			return nil
		},
	}

	if err := c.AttachToContainer("", "", "c", nil, nil, nil, false, nil, 0); err != nil {
		t.Fatalf("AttachToContainer returned error: %v", err)
	}

	handlerCtx := <-gotCtx
	if handlerCtx == nil {
		t.Fatal("handler received nil context")
	}

	select {
	case <-handlerCtx.Done():
		t.Fatal("ctx cancelled prematurely")
	default:
	}

	c.Cancel()

	select {
	case <-handlerCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("handler context not cancelled after Cancel()")
	}
}
