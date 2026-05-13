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

// This file contains helper functions to integrate Temporal options with Cobra and Viper for command-line applications.
//
// Example implementation:
/**
cmd := &cobra.Command{
	Use:   "run",
	Short: "Run a Temporal worker",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := temporal.NewConnection(append(
			temporal.ParseCobraOpts(opts.temporal),
			temporal.WithZerolog(&log.Logger),
			temporal.WithPrometheusMetrics(opts.temporal.MetricsListenAddress, opts.temporal.MetricsPrefix, nil),
		)...)
		if err != nil {
			return gh.FatalError{
				Cause: err,
				Msg:   "Unable to create client",
			}
		}
		defer c.Close()

		w := worker.New(c, TaskQueue, worker.Options{})

		// Start the healthcheck server in a separate goroutine
		temporal.NewHealthCheck(cmd.Context(), []string{TaskQueue}, opts.temporal.HealthListenAddress, c)

		if err := w.Run(worker.InterruptCh()); err != nil {
			return gh.FatalError{
				Cause: err,
				Msg:   "Worker stopped",
			}
		}

		return nil
	},
}
*/

import (
	gh "github.com/mrsimonemms/golang-helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.temporal.io/sdk/client"
)

type TemporalOpts struct {
	Address              string
	APIKey               string
	HealthListenAddress  string
	MetricsListenAddress string
	MetricsPrefix        string
	MTLSCertPath         string
	MTLSKeyPath          string
	Namespace            string
	ServerName           string
	TLSEnabled           bool
}

func NewCobraOpts(cmd *cobra.Command, opts *TemporalOpts) *TemporalOpts {
	viper.SetDefault("health_listen_address", "0.0.0.0:3000")
	cmd.Flags().StringVar(
		&opts.HealthListenAddress, "health-listen-address",
		viper.GetString("health_listen_address"), "Address of health server",
	)

	viper.SetDefault("metrics_listen_address", "0.0.0.0:9090")
	cmd.Flags().StringVar(
		&opts.MetricsListenAddress, "metrics-listen-address",
		viper.GetString("metrics_listen_address"), "Address of Prometheus metrics server",
	)

	cmd.Flags().StringVar(
		&opts.MetricsPrefix, "metrics-prefix",
		viper.GetString("metrics_prefix"), "Prefix for metrics",
	)

	viper.SetDefault("temporal_address", client.DefaultHostPort)
	cmd.Flags().StringVarP(
		&opts.Address, "temporal-address", "H",
		viper.GetString("temporal_address"), "Address of the Temporal server",
	)

	cmd.Flags().StringVar(
		&opts.APIKey, "temporal-api-key",
		viper.GetString("temporal_api_key"), "API key for Temporal authentication",
	)
	// Hide the default value to avoid spaffing the API to command line
	gh.HideCommandOutput(cmd, "temporal-api-key")

	cmd.Flags().StringVar(
		&opts.MTLSCertPath, "tls-client-cert-path",
		viper.GetString("temporal_tls_client_cert_path"), "Path to mTLS client cert, usually ending in .pem",
	)

	cmd.Flags().StringVar(
		&opts.MTLSKeyPath, "tls-client-key-path",
		viper.GetString("temporal_tls_client_key_path"), "Path to mTLS client key, usually ending in .key",
	)

	viper.SetDefault("temporal_namespace", client.DefaultNamespace)
	cmd.Flags().StringVarP(
		&opts.Namespace, "temporal-namespace", "n",
		viper.GetString("temporal_namespace"), "Temporal namespace to use",
	)

	cmd.Flags().StringVar(
		&opts.ServerName, "temporal-server-name",
		viper.GetString("temporal_server_name"),
		"Override the TLS server name (SNI) used for certificate validation. "+
			"Required when the endpoint address does not match the certificate hostname, for example AWS PrivateLink.",
	)

	cmd.Flags().BoolVar(
		&opts.TLSEnabled, "temporal-tls",
		viper.GetBool("temporal_tls"), "Enable TLS Temporal connection",
	)

	return opts
}

func ParseCobraOpts(opts *TemporalOpts) []Options {
	return []Options{
		WithHostPort(opts.Address),
		WithNamespace(opts.Namespace),
		WithTLS(opts.TLSEnabled, WithTLSServerName(opts.ServerName)),
		WithAuthDetection(
			opts.APIKey,
			opts.MTLSCertPath,
			opts.MTLSKeyPath,
		),
	}
}
