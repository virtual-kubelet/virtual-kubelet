package proxy

import (
	"context"
	"errors"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/kv"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/pkg/trace"
	"fmt"
)

type KeyValueStoreProxy interface {
	Get(ctx context.Context, key string) (string, error)
	Put(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

const (

	// defaultNamespace is the first part of the
	// k/v store key (i.e. docker.stuff)
	defaultNamespace = "docker"
	defaultSeparator = "."
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

type VicKeyValueStoreProxy struct {
	client        *client.PortLayer
}

func NewVicKeyValueStoreProxy(plClient *client.PortLayer) KeyValueStoreProxy {
	if plClient == nil {
		return nil
	}

	return &VicKeyValueStoreProxy{
		client: plClient,
	}
}

func (v *VicKeyValueStoreProxy) Get(ctx context.Context, key string) (string, error) {
	op := trace.FromContext(ctx, "Get")
	defer trace.End(trace.Begin(key, op))

	var val string
	param := kv.NewGetValueParamsWithContext(op).WithKey(createNameSpacedKey(key))
	resp, err := v.client.Kv.GetValue(param)
	if err != nil {
		switch err.(type) {
		case *kv.GetValueNotFound:
			return val, ErrKeyNotFound
		default:
			op.Errorf("Error Getting Key/Value: %s", err.Error())
			return val, err
		}
	}
	val = resp.Payload.Value

	return val, nil
}

func (v *VicKeyValueStoreProxy) Put(ctx context.Context, key, value string) error {
	op := trace.FromContext(ctx, "Put")
	defer trace.End(trace.Begin(key, op))

	fullKey := createNameSpacedKey(key)
	keyval := &models.KeyValue{
		Key:   fullKey,
		Value: value,
	}

	param := kv.NewPutValueParamsWithContext(op).WithKey(fullKey).WithKeyValue(keyval)
	_, err := v.client.Kv.PutValue(param)
	if err != nil {
		op.Errorf("Error Putting Key/Value: %s", err)
		return err
	}

	return nil
}

func (v *VicKeyValueStoreProxy) Delete(ctx context.Context, key string) error {
	op := trace.FromContext(ctx, "Put")
	defer trace.End(trace.Begin(key, op))

	param := kv.NewDeleteValueParamsWithContext(op).WithKey(createNameSpacedKey(key))
	_, err := v.client.Kv.DeleteValue(param)
	if err != nil {
		op.Errorf("Error Deleting Key/Value: %s", err)
		return err
	}

	return nil
}

func createNameSpacedKey(key string) string {
	return fmt.Sprintf("%s%s%s", defaultNamespace, defaultSeparator, key)
}
