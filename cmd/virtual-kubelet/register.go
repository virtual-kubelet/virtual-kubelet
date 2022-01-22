package main

import (
	"context"

	"github.com/nuczzz/virtual-kubelet/cmd/virtual-kubelet/internal/provider"
	"github.com/nuczzz/virtual-kubelet/cmd/virtual-kubelet/internal/provider/mock"
	"github.com/nuczzz/virtual-kubelet/cmd/virtual-kubelet/internal/provider/nuczzz"
)

func registerProvider(ctx context.Context, s *provider.Store) {
	s.Register(mock.ProviderName, mock.NewMockProvider)            //nolint:errcheck
	s.Register(nuczzz.ProviderName, nuczzz.NewNuczzzProvider(ctx)) //nolint:errcheck
}
