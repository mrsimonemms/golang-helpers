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

package temporal

import (
	"crypto/tls"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/log"
)

type Options func(*client.Options) error

func NewConnection(options ...Options) (client.Client, error) {
	opts := &client.Options{}
	for _, o := range options {
		if err := o(opts); err != nil {
			return nil, err
		}
	}
	return client.Dial(*opts)
}

func WithAPICredentials(apiKey string) Options {
	return func(o *client.Options) error {
		if apiKey != "" {
			return WithCredentials(client.NewAPIKeyStaticCredentials(apiKey))(o)
		}
		return nil
	}
}

func WithConnectionOptions(connection *client.ConnectionOptions) Options {
	return func(o *client.Options) error {
		o.ConnectionOptions = *connection
		return nil
	}
}

func WithCredentials(credential client.Credentials) Options {
	return func(o *client.Options) error {
		o.Credentials = credential
		return nil
	}
}

func WithDataConverter(cvt converter.DataConverter) Options {
	return func(o *client.Options) error {
		o.DataConverter = cvt
		return nil
	}
}

func WithHostPort(hostPort string) Options {
	return func(o *client.Options) error {
		o.HostPort = hostPort
		return nil
	}
}

func WithLogger(logger log.Logger) Options {
	return func(o *client.Options) error {
		o.Logger = logger
		return nil
	}
}

func WithMetrics(metrics client.MetricsHandler) Options {
	return func(o *client.Options) error {
		o.MetricsHandler = metrics
		return nil
	}
}

func WithNamespace(namespace string) Options {
	return func(o *client.Options) error {
		o.Namespace = namespace
		return nil
	}
}

func WithPrometheusMetrics(listenAddress, prefix string) Options {
	return func(o *client.Options) error {
		metrics, err := NewPrometheusHandler(listenAddress, prefix)
		if err != nil {
			return err
		}
		return WithMetrics(metrics)(o)
	}
}

func WithTLS(enabled bool) Options {
	return func(o *client.Options) error {
		if enabled {
			connectionOpts := &client.ConnectionOptions{}
			connectionOpts.TLS = new(tls.Config)
			return WithConnectionOptions(connectionOpts)(o)
		}
		return nil
	}
}

func WithZerolog(logger *zerolog.Logger) Options {
	return WithLogger(NewZerologHandler(logger))
}
