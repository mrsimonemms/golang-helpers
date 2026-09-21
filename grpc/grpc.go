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

package grpc

import (
	"fmt"
	"net"
	"os"
	"time"

	golanghelpers "github.com/mrsimonemms/golang-helpers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type Options struct {
	HealthChecks  map[string]HealthCheck
	ServerOptions []grpc.ServerOption
}

type HealthCheck struct {
	Timeout *time.Duration
	Check   HealthCheckFn
}

type HealthCheckFn func(*health.Server) grpc_health_v1.HealthCheckResponse_ServingStatus

type ServerFactory func(server *grpc.Server)

type Server struct {
	RootCmd *cobra.Command
	RunCmd  *cobra.Command
}

type Listener[T any] struct {
	Flags func(*cobra.Command)
	Run   func(*cobra.Command, []string) (*T, error)
}

// StreamResponse has the same interface as the gRPC streaming server which is useful for local development
type StreamResponse[T any] struct {
	grpc.ServerStream
}

// Send is the only method on the StreamResponse. Any data received is sent directly to the terminal logger.
func (f *StreamResponse[T]) Send(data *T) error {
	log.Info().Any("data", data).Msg("New stream data received")
	return nil
}

func NewGRPCCommand[T any](s *Server, command string, f Listener[T]) *Server {
	cmd := &cobra.Command{
		Use:   command,
		Short: fmt.Sprintf(`Run the "%q" gRPC command`, command),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := f.Run(cmd, args)
			if err != nil {
				return err
			}

			log.Info().Any("response", res).Msg("Command resolved successfully")
			return nil
		},
	}
	if f.Flags != nil {
		f.Flags(cmd)
	}

	s.RunCmd.AddCommand(cmd)

	return s
}

func (s *Server) Execute() {
	s.RootCmd.AddCommand(s.RunCmd)

	err := s.RootCmd.Execute()
	if err != nil {
		os.Exit(golanghelpers.HandleFatalError(err))
	}
}

func New(name, description string, serverFactory []ServerFactory, opts ...Options) *Server {
	rootCmd := newRootCmd(name, description, serverFactory, opts...)

	return &Server{
		RootCmd: rootCmd,
		RunCmd: &cobra.Command{
			Use: "run",
			//nolint:lll // Allow long message for exact CLI output
			Short: `Debug a gRPC command by running it as single, standalone calls. Configure all your input parameters as Cobra flags and watch it fly.

Any response from the command will be sent to the console. In production, this will be returned via gRPC.`,
		},
	}
}

func newRootCmd(name, description string, serverFactory []ServerFactory, opts ...Options) *cobra.Command {
	viper.AutomaticEnv()

	var logLevel string
	var port int

	rootCmd := &cobra.Command{
		Use:           name,
		Short:         description,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level, err := zerolog.ParseLevel(logLevel)
			if err != nil {
				return golanghelpers.FatalError{
					Cause: err,
					Msg:   "Error setting the log level",
				}
			}
			zerolog.SetGlobalLevel(level)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			//nolint:noctx
			lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return fmt.Errorf("failed to start listener: %w", err)
			}

			serverOpts := make([]grpc.ServerOption, 0)
			healthchecks := map[string]HealthCheck{}
			for _, o := range opts {
				serverOpts = append(serverOpts, o.ServerOptions...)
				for k, v := range o.HealthChecks {
					if _, ok := healthchecks[k]; ok {
						return fmt.Errorf("health check already registered: %s", k)
					}
					healthchecks[k] = v
				}
			}

			server := grpc.NewServer(serverOpts...)
			// Register reflection service on gRPC server.
			reflection.Register(server)

			healthcheck := health.NewServer()
			grpc_health_v1.RegisterHealthServer(server, healthcheck)

			for service, check := range healthchecks {
				go func() {
					if check.Timeout == nil {
						check.Timeout = golanghelpers.Ptr(time.Second * 10)
					}

					for {
						// Run health check
						status := check.Check(healthcheck)

						l := log.With().
							Str("status", status.String()).
							Str("service", service).
							Dur("timeout", *check.Timeout).
							Logger()

						if status == grpc_health_v1.HealthCheckResponse_SERVING {
							l.Debug().Msg("Running health check")
						} else {
							l.Error().Msg("Health check failed")
						}

						healthcheck.SetServingStatus(service, status)

						time.Sleep(*check.Timeout)
					}
				}()
			}

			for _, factory := range serverFactory {
				factory(server)
			}

			log.Info().Str("address", lis.Addr().String()).Msg("Server listening")
			return server.Serve(lis)
		},
	}

	viper.SetDefault("log_level", zerolog.InfoLevel.String())
	rootCmd.PersistentFlags().StringVarP(
		&logLevel,
		"log-level",
		"l",
		viper.GetString("log_level"),
		"Set log level",
	)

	viper.SetDefault("port", 3000)
	rootCmd.Flags().IntVarP(&port, "port", "p", viper.GetInt("port"), "The server port")

	return rootCmd
}
