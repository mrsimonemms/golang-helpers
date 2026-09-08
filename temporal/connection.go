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
	"fmt"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/envconfig"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
)

// Deprecated: use github.com/zigflow/helpers instead.
type Options func(*client.Options) error

// Deprecated: use github.com/zigflow/helpers instead.
type TLSOptions func(*tls.Config) error

// Create a connection to Temporal
func newConnection(clientOptions *client.Options, options ...Options) (client.Client, error) {
	for _, o := range options {
		if err := o(clientOptions); err != nil {
			return nil, err
		}
	}
	return client.Dial(*clientOptions)
}

// NewConnectionWithEnvvars
//
// Create a Temporal connection, with the Temporal environment config loader as
// the starting point. This is experimental.
//
// @link https://docs.temporal.io/develop/environment-configuration#sdk-usage-example-go
//
// Deprecated: use github.com/zigflow/helpers instead.
func NewConnectionWithEnvvars(options ...Options) (client.Client, error) {
	clientOptions, err := envconfig.LoadDefaultClientOptions()
	if err != nil {
		return nil, fmt.Errorf("error loading environment config: %w", err)
	}

	return newConnection(&clientOptions, options...)
}

// New Connection
//
// Create a Temporal connection and only use options that are supplied.
//
// Deprecated: use github.com/zigflow/helpers instead.
func NewConnection(options ...Options) (client.Client, error) {
	clientOptions := &client.Options{}
	return newConnection(clientOptions, options...)
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithAPICredentials(apiKey string) Options {
	return func(o *client.Options) error {
		if apiKey != "" {
			return WithCredentials(client.NewAPIKeyStaticCredentials(apiKey))(o)
		}
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithAuthDetection(apiKey, certPath, certKey string) Options {
	if apiKey != "" {
		return WithAPICredentials(apiKey)
	}

	if certKey != "" && certPath != "" {
		return WithMTLS(certPath, certKey)
	}

	return WithNoOp()
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithConnectionOptions(connection *client.ConnectionOptions) Options {
	return func(o *client.Options) error {
		o.ConnectionOptions = *connection
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithCredentials(credential client.Credentials) Options {
	return func(o *client.Options) error {
		o.Credentials = credential
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithDataConverter(cvt converter.DataConverter) Options {
	return func(o *client.Options) error {
		o.DataConverter = cvt
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithDataAndFailureConverter(cvt converter.DataConverter) Options {
	return func(o *client.Options) error {
		if err := WithDataConverter(cvt)(o); err != nil {
			return err
		}

		return WithFailureConverter(cvt)(o)
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithExternalStorage(st converter.ExternalStorage) Options {
	return func(o *client.Options) error {
		o.ExternalStorage = st
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithFailureConverter(cvt converter.DataConverter) Options {
	return func(o *client.Options) error {
		o.FailureConverter = temporal.NewDefaultFailureConverter(
			temporal.DefaultFailureConverterOptions{
				DataConverter:          cvt,
				EncodeCommonAttributes: true,
			},
		)
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithHostPort(hostPort string) Options {
	return func(o *client.Options) error {
		if hostPort == "" {
			hostPort = client.DefaultHostPort
		}
		o.HostPort = hostPort
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithLogger(logger log.Logger) Options {
	return func(o *client.Options) error {
		o.Logger = logger
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithMetrics(metrics client.MetricsHandler) Options {
	return func(o *client.Options) error {
		o.MetricsHandler = metrics
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithMTLS(certPath, certKey string) Options {
	return func(o *client.Options) error {
		// Use the crypto/tls package to create a cert object
		cert, err := tls.LoadX509KeyPair(certPath, certKey)
		if err != nil {
			return fmt.Errorf("error loading tls key pair: %w", err)
		}

		return WithCredentials(client.NewMTLSCredentials(cert))(o)
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithNamespace(namespace string) Options {
	return func(o *client.Options) error {
		if namespace == "" {
			namespace = client.DefaultNamespace
		}
		o.Namespace = namespace
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithNoOp() Options {
	return func(o *client.Options) error {
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithPrometheusMetrics(listenAddress, prefix string, registry *prom.Registry) Options {
	return func(o *client.Options) error {
		metrics, err := NewPrometheusHandler(listenAddress, prefix, registry)
		if err != nil {
			return err
		}
		return WithMetrics(metrics)(o)
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithTLS(enabled bool, tlsOpts ...TLSOptions) Options {
	return func(o *client.Options) error {
		if enabled {
			tlsConfig := new(tls.Config)

			for _, opt := range tlsOpts {
				if err := opt(tlsConfig); err != nil {
					return fmt.Errorf("error configuring tls options: %w", err)
				}
			}

			connectionOpts := &client.ConnectionOptions{
				TLS: tlsConfig,
			}
			return WithConnectionOptions(connectionOpts)(o)
		}
		return nil
	}
}

// Deprecated: use github.com/zigflow/helpers instead.
func WithZerolog(logger *zerolog.Logger) Options {
	return WithLogger(NewZerologHandler(logger))
}

// TLS options
//
// Deprecated: use github.com/zigflow/helpers instead.
func WithTLSServerName(serverName string) TLSOptions {
	return func(c *tls.Config) error {
		if serverName == "" {
			return nil
		}
		c.ServerName = serverName
		return nil
	}
}
