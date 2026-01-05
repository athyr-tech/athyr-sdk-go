package athyr

import (
	"context"

	athyr "github.com/athyr-tech/athyr-sdk-go/api/v1"
)

// KVBucket provides key-value operations on a specific bucket.
type KVBucket interface {
	Get(ctx context.Context, key string) (*KVEntry, error)
	Put(ctx context.Context, key string, value []byte) (uint64, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// KV returns a KVBucket for the given bucket name.
func (c *agent) KV(bucket string) KVBucket {
	return &kvBucket{
		client: c,
		bucket: bucket,
	}
}

// kvBucket implements KVBucket.
type kvBucket struct {
	client *agent
	bucket string
}

func (k *kvBucket) Get(ctx context.Context, key string) (*KVEntry, error) {
	if err := k.client.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := k.client.athyr.KVGet(ctx, &athyr.KVGetRequest{
		AgentId: k.client.agentID,
		Bucket:  k.bucket,
		Key:     key,
	})
	if err != nil {
		return nil, err
	}

	return &KVEntry{
		Value:    resp.Value,
		Revision: resp.Revision,
	}, nil
}

func (k *kvBucket) Put(ctx context.Context, key string, value []byte) (uint64, error) {
	if err := k.client.checkConnected(); err != nil {
		return 0, err
	}

	resp, err := k.client.athyr.KVPut(ctx, &athyr.KVPutRequest{
		AgentId: k.client.agentID,
		Bucket:  k.bucket,
		Key:     key,
		Value:   value,
	})
	if err != nil {
		return 0, err
	}

	return resp.Revision, nil
}

func (k *kvBucket) Delete(ctx context.Context, key string) error {
	if err := k.client.checkConnected(); err != nil {
		return err
	}

	_, err := k.client.athyr.KVDelete(ctx, &athyr.KVDeleteRequest{
		AgentId: k.client.agentID,
		Bucket:  k.bucket,
		Key:     key,
	})
	return err
}

func (k *kvBucket) List(ctx context.Context, prefix string) ([]string, error) {
	if err := k.client.checkConnected(); err != nil {
		return nil, err
	}

	resp, err := k.client.athyr.KVList(ctx, &athyr.KVListRequest{
		AgentId: k.client.agentID,
		Bucket:  k.bucket,
		Prefix:  prefix,
	})
	if err != nil {
		return nil, err
	}

	return resp.Keys, nil
}
