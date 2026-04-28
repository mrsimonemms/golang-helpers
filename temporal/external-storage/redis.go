/*
 * Copyright 2023 Simon Emms <simon@simonemms.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package externalstorage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

const defaultRedisDriverName = "temporal.redis"

type RedisDriver struct {
	client     *redis.Client
	driverName string
}

type RedisOptions struct {
	ClientOpts *redis.Options
	DriverName string
}

// Name implements [converter.StorageDriver].
func (r *RedisDriver) Name() string {
	return r.driverName
}

// Retrieve implements [converter.StorageDriver].
func (r *RedisDriver) Retrieve(
	ctx converter.StorageDriverRetrieveContext,
	claims []converter.StorageDriverClaim,
) ([]*common.Payload, error) {
	panic("unimplemented")
}

// Store implements [converter.StorageDriver].
func (r *RedisDriver) Store(
	ctx converter.StorageDriverStoreContext,
	payloads []*common.Payload,
) ([]converter.StorageDriverClaim, error) {
	panic("unimplemented")
}

// Type implements [converter.StorageDriver].
func (r *RedisDriver) Type() string {
	return defaultRedisDriverName
}

func NewRedisStorageDriver(opts *RedisOptions) (converter.StorageDriver, error) {
	if opts == nil {
		opts = &RedisOptions{}
	}
	if opts.ClientOpts == nil {
		opts.ClientOpts = &redis.Options{
			Addr: "localhost:6379",
		}
	}

	client := redis.NewClient(opts.ClientOpts)

	// Test the connection
	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("error connecting to redis: %w", err)
	}

	driverName := opts.DriverName
	if driverName == "" {
		driverName = defaultRedisDriverName
	}

	return &RedisDriver{
		client:     client,
		driverName: driverName,
	}, nil
}

var _ converter.StorageDriver = &RedisDriver{}
