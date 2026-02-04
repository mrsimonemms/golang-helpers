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
)

type Options func(*client.Options) error

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
func NewConnectionWithEnvvars(options ...Options) (client.Client, error) {
	clientOptions, err := envconfig.LoadDefaultClientOptions()
	if err != nil {
		return nil, fmt.Errorf("error loading environment config: %w", err)
	}

	return newConnection(&clientOptions, options...)
}

// New Connection
//
// Create a Temporal connection and only use options that are supplied
func NewConnection(options ...Options) (client.Client, error) {
	clientOptions := &client.Options{}
	return newConnection(clientOptions, options...)
}

func WithAPICredentials(apiKey string) Options {
	return func(o *client.Options) error {
		if apiKey != "" {
			return WithCredentials(client.NewAPIKeyStaticCredentials(apiKey))(o)
		}
		return nil
	}
}

func WithAuthDetection(apiKey, certPath, certKey string) Options {
	if apiKey != "" {
		return WithAPICredentials(apiKey)
	}

	if certKey != "" && certPath != "" {
		return WithMTLS(certPath, certKey)
	}

	return WithNoOp()
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
		if hostPort == "" {
			hostPort = client.DefaultHostPort
		}
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

func WithNamespace(namespace string) Options {
	return func(o *client.Options) error {
		if namespace == "" {
			namespace = client.DefaultNamespace
		}
		o.Namespace = namespace
		return nil
	}
}

func WithNoOp() Options {
	return func(o *client.Options) error {
		return nil
	}
}

func WithPrometheusMetrics(listenAddress, prefix string, registry *prom.Registry) Options {
	return func(o *client.Options) error {
		metrics, err := NewPrometheusHandler(listenAddress, prefix, registry)
		if err != nil {
			return err
		}
		return WithMetrics(metrics)(o)
	}
}

func WithTLS(enabled bool) Options {
	return func(o *client.Options) error {
		if enabled {
			connectionOpts := &client.ConnectionOptions{
				TLS: new(tls.Config),
			}
			return WithConnectionOptions(connectionOpts)(o)
		}
		return nil
	}
}

func WithZerolog(logger *zerolog.Logger) Options {
	return WithLogger(NewZerologHandler(logger))
}
