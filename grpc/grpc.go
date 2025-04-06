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

	"github.com/mrsimonemms/golang-helpers/logger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type Options struct {
	HealthChecks  []HealthCheck
	ServerOptions []grpc.ServerOption
}

type HealthCheck func(*health.Server)

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
	logger.Log().WithField("data", data).Info("New stream data received")
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

			logger.Log().WithField("response", res).Info("Command resolved successfully")
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
		os.Exit(1)
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
	var logLevel string
	var port int

	rootCmd := &cobra.Command{
		Use:   name,
		Short: description,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return logger.SetLevel(logLevel)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				return fmt.Errorf("failed to start listener: %w", err)
			}

			serverOpts := make([]grpc.ServerOption, 0)
			for _, o := range opts {
				serverOpts = append(serverOpts, o.ServerOptions...)
			}

			server := grpc.NewServer(serverOpts...)
			// Register reflection service on gRPC server.
			reflection.Register(server)

			// @todo(sje): allow customised health checks
			grpc_health_v1.RegisterHealthServer(server, health.NewServer())

			for _, factory := range serverFactory {
				factory(server)
			}

			logger.Log().WithField("address", lis.Addr()).Info("Server listening")
			return server.Serve(lis)
		},
	}

	rootCmd.PersistentFlags().StringVarP(
		&logLevel,
		"log-level",
		"l",
		logrus.InfoLevel.String(),
		fmt.Sprintf("log level: %s", logger.GetAllLevels()),
	)

	rootCmd.Flags().IntVarP(&port, "port", "p", 3000, "The server port")

	return rootCmd
}
