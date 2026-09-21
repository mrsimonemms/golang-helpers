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

package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	golanghelpers "github.com/mrsimonemms/golang-helpers"
	helpersgrpc "github.com/mrsimonemms/golang-helpers/grpc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	logLevelFlag  = "log-level"
	nameFlag      = "name"
	levelDebug    = "debug"
	levelError    = "error"
	levelInfo     = "info"
	healthService = "svc"

	// invalidLevel is rejected by zerolog.ParseLevel
	invalidLevel    = "bananas"
	invalidLevelErr = "Unknown Level String: 'bananas', defaulting to NoLevel"

	// executeSubprocessEnv marks the re-executed copy of TestServerExecute
	executeSubprocessEnv = "GRPC_TEST_SERVER_EXECUTE"
)

type payload struct {
	Name string `json:"name"`
}

// logBuffer is a concurrency-safe log sink. The health check goroutine logs
// from a goroutine the tests don't own, so the buffer has to be guarded
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *logBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	return bytes.Clone(b.buf.Bytes())
}

func (b *logBuffer) String() string {
	return string(b.Bytes())
}

// captureLog swaps the global zerolog logger for one writing to an in-memory
// buffer. The global level is restored too as the command under test sets it
func captureLog(t *testing.T) *logBuffer {
	t.Helper()

	out := new(logBuffer)
	prevLogger := log.Logger
	prevLevel := zerolog.GlobalLevel()
	t.Cleanup(func() {
		log.Logger = prevLogger
		zerolog.SetGlobalLevel(prevLevel)
	})
	log.Logger = zerolog.New(out)

	return out
}

// decodeLog decodes a single structured log line. Field ordering isn't part of
// the contract, so the tests assert against the decoded fields
func decodeLog(t *testing.T, out *logBuffer) map[string]any {
	t.Helper()

	logged := map[string]any{}
	assert.NoError(t, json.Unmarshal(out.Bytes(), &logged))

	return logged
}

// decodeLogLines decodes every structured log line in the buffer
func decodeLogLines(t *testing.T, out *logBuffer) []map[string]any {
	t.Helper()

	lines := make([]map[string]any, 0)
	for _, line := range bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}

		logged := map[string]any{}
		assert.NoError(t, json.Unmarshal(line, &logged))
		lines = append(lines, logged)
	}

	return lines
}

// parseLogLines decodes the structured log lines out of a stream that may also
// carry unstructured output, such as a subprocess's stderr
func parseLogLines(b []byte) []map[string]any {
	lines := make([]map[string]any, 0)
	for _, line := range bytes.Split(b, []byte("\n")) {
		logged := map[string]any{}
		if err := json.Unmarshal(line, &logged); err != nil {
			continue
		}
		lines = append(lines, logged)
	}

	return lines
}

// findLog returns the first decoded log line with the given message
func findLog(lines []map[string]any, message string) map[string]any {
	for _, line := range lines {
		if line["message"] == message {
			return line
		}
	}

	return nil
}

// resetViper isolates tests from the global viper state that newRootCmd
// mutates via AutomaticEnv and SetDefault
func resetViper(t *testing.T) {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestStreamResponseSend(t *testing.T) {
	tests := []struct {
		Name  string
		Data  *payload
		Value any
	}{
		{
			Name:  "Populated payload",
			Data:  &payload{Name: "hello"},
			Value: map[string]any{nameFlag: "hello"},
		},
		{
			Name:  "Nil payload",
			Data:  nil,
			Value: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			out := captureLog(t)

			s := &helpersgrpc.StreamResponse[payload]{}

			assert.NoError(t, s.Send(test.Data))

			logged := decodeLog(t, out)
			assert.Equal(t, levelInfo, logged["level"])
			assert.Equal(t, "New stream data received", logged["message"])
			assert.Equal(t, test.Value, logged["data"])
		})
	}
}

func TestNewGRPCCommand(t *testing.T) {
	tests := []struct {
		Name       string
		Run        func(*cobra.Command, []string) (*payload, error)
		Flags      func(*cobra.Command)
		ExpectErr  string
		ExpectLog  bool
		ExpectResp any
	}{
		{
			Name: "Successful command",
			Run: func(*cobra.Command, []string) (*payload, error) {
				return &payload{Name: "world"}, nil
			},
			ExpectLog:  true,
			ExpectResp: map[string]any{nameFlag: "world"},
		},
		{
			Name: "Successful command with nil response",
			Run: func(*cobra.Command, []string) (*payload, error) {
				return nil, nil
			},
			ExpectLog:  true,
			ExpectResp: nil,
		},
		{
			Name: "Failed command",
			Run: func(*cobra.Command, []string) (*payload, error) {
				return nil, fmt.Errorf("some error")
			},
			ExpectErr: "some error",
		},
		{
			Name: "Flags are applied to the new command",
			Run: func(cmd *cobra.Command, _ []string) (*payload, error) {
				name, err := cmd.Flags().GetString(nameFlag)
				if err != nil {
					return nil, err
				}
				return &payload{Name: name}, nil
			},
			Flags: func(cmd *cobra.Command) {
				cmd.Flags().String(nameFlag, "flagged", "")
			},
			ExpectLog:  true,
			ExpectResp: map[string]any{nameFlag: "flagged"},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			resetViper(t)
			out := captureLog(t)

			s := helpersgrpc.New("app", "description", nil)
			res := helpersgrpc.NewGRPCCommand(s, "mycmd", helpersgrpc.Listener[payload]{
				Flags: test.Flags,
				Run:   test.Run,
			})

			// The same server is returned so calls can be chained
			assert.Same(t, s, res)

			cmd, _, err := s.RunCmd.Find([]string{"mycmd"})
			assert.NoError(t, err)
			assert.Equal(t, "mycmd", cmd.Use)
			assert.Same(t, s.RunCmd, cmd.Parent())

			if test.Flags != nil {
				assert.NotNil(t, cmd.Flags().Lookup(nameFlag))
			}

			err = cmd.RunE(cmd, []string{})

			if test.ExpectErr != "" {
				assert.EqualError(t, err, test.ExpectErr)
			} else {
				assert.NoError(t, err)
			}

			if !test.ExpectLog {
				// A failing command logs nothing - the error is returned
				assert.Empty(t, out.String())
				return
			}

			logged := decodeLog(t, out)
			assert.Equal(t, levelInfo, logged["level"])
			assert.Equal(t, "Command resolved successfully", logged["message"])
			assert.Equal(t, test.ExpectResp, logged["response"])
		})
	}
}

func TestNew(t *testing.T) {
	resetViper(t)

	s := helpersgrpc.New("app", "description", nil)

	assert.Equal(t, "app", s.RootCmd.Use)
	assert.Equal(t, "description", s.RootCmd.Short)
	assert.Equal(t, "run", s.RunCmd.Use)

	// The run command is only attached by Execute
	assert.Empty(t, s.RootCmd.Commands())

	logLevel := s.RootCmd.PersistentFlags().Lookup(logLevelFlag)
	assert.NotNil(t, logLevel)
	assert.Equal(t, "l", logLevel.Shorthand)
	assert.Equal(t, levelInfo, logLevel.DefValue)

	port := s.RootCmd.Flags().Lookup("port")
	assert.NotNil(t, port)
	assert.Equal(t, "p", port.Shorthand)
	assert.Equal(t, "3000", port.DefValue)
}

func TestRootCmdLogLevel(t *testing.T) {
	tests := []struct {
		Name     string
		LogLevel string
		Level    zerolog.Level
		ErrMsg   string
	}{
		{
			Name:     "Trace",
			LogLevel: "trace",
			Level:    zerolog.TraceLevel,
		},
		{
			Name:     "Debug",
			LogLevel: levelDebug,
			Level:    zerolog.DebugLevel,
		},
		{
			Name:     "Info",
			LogLevel: levelInfo,
			Level:    zerolog.InfoLevel,
		},
		{
			Name:     "Warn",
			LogLevel: "warn",
			Level:    zerolog.WarnLevel,
		},
		{
			Name:     "Error",
			LogLevel: levelError,
			Level:    zerolog.ErrorLevel,
		},
		{
			Name:     "Mixed case is accepted",
			LogLevel: "DeBuG",
			Level:    zerolog.DebugLevel,
		},
		{
			Name:     "Unknown level is rejected",
			LogLevel: invalidLevel,
			ErrMsg:   invalidLevelErr,
		},
		{
			// zerolog has no "warning" alias, so only "warn" is accepted
			Name:     "Warning is not a valid level",
			LogLevel: "warning",
			ErrMsg:   "Unknown Level String: 'warning', defaulting to NoLevel",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			resetViper(t)
			captureLog(t)
			zerolog.SetGlobalLevel(zerolog.InfoLevel)

			s := helpersgrpc.New("app", "description", nil)
			assert.NoError(t, s.RootCmd.PersistentFlags().Set(logLevelFlag, test.LogLevel))

			err := s.RootCmd.PersistentPreRunE(s.RootCmd, []string{})

			if test.ErrMsg == "" {
				assert.NoError(t, err)
				assert.Equal(t, test.Level, zerolog.GlobalLevel())
				return
			}

			// The failure is wrapped so HandleFatalError can render it
			var fatal golanghelpers.FatalError
			assert.ErrorAs(t, err, &fatal)
			assert.Equal(t, "Error setting the log level", fatal.Msg)
			assert.EqualError(t, fatal.Cause, test.ErrMsg)

			// An unparseable level leaves the global level untouched
			assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
		})
	}
}

// TestRootCmdLogLevelEmpty covers the behaviour that an empty log level maps to
// zerolog's NoLevel, which suppresses every level below it
func TestRootCmdLogLevelEmpty(t *testing.T) {
	resetViper(t)
	out := captureLog(t)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	s := helpersgrpc.New("app", "description", nil)
	assert.NoError(t, s.RootCmd.PersistentFlags().Set(logLevelFlag, ""))

	assert.NoError(t, s.RootCmd.PersistentPreRunE(s.RootCmd, []string{}))
	assert.Equal(t, zerolog.NoLevel, zerolog.GlobalLevel())

	// Info is below NoLevel, so nothing is emitted
	log.Info().Msg("swallowed")
	assert.Empty(t, out.String())
}

// TestRootCmdExecuteReportsFatalError covers the error path Server.Execute
// delegates to. Execute itself calls os.Exit so can't be invoked directly, but
// it is a straight composition of the two calls asserted here
func TestRootCmdExecuteReportsFatalError(t *testing.T) {
	resetViper(t)
	out := captureLog(t)
	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	s := helpersgrpc.New("app", "description", nil)

	cmdOut := new(bytes.Buffer)
	s.RootCmd.SetOut(cmdOut)
	s.RootCmd.SetErr(cmdOut)
	s.RootCmd.SetArgs([]string{"--" + logLevelFlag, invalidLevel})

	err := s.RootCmd.Execute()

	var fatal golanghelpers.FatalError
	assert.ErrorAs(t, err, &fatal)

	// SilenceErrors and SilenceUsage stop cobra reporting it...
	assert.Empty(t, cmdOut.String())

	// ...instead Server.Execute passes it to HandleFatalError, which logs it
	// and returns the exit code
	assert.Equal(t, 1, golanghelpers.HandleFatalError(err))

	logged := decodeLog(t, out)
	assert.Equal(t, levelError, logged["level"])
	assert.Equal(t, "Error setting the log level", logged["message"])
	assert.Equal(t, invalidLevelErr, logged["error"])
}

// TestServerExecute covers Server.Execute end to end. It calls os.Exit, so the
// only way to observe both the exit code and the logging is to re-run this
// test in a subprocess
func TestServerExecute(t *testing.T) {
	if os.Getenv(executeSubprocessEnv) == "1" {
		s := helpersgrpc.New("app", "description", nil)
		s.RootCmd.SetArgs([]string{"--" + logLevelFlag, invalidLevel})
		s.Execute()

		return
	}

	// Belt and braces - if the command ever stopped failing it would start a
	// real server and block forever
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
	t.Cleanup(cancel)

	//nolint:gosec // Re-executes this test binary, not external input
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestServerExecute$")
	cmd.Env = append(os.Environ(), executeSubprocessEnv+"=1")

	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr

	var exitErr *exec.ExitError
	assert.ErrorAs(t, cmd.Run(), &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())

	// HandleFatalError logs through the default zerolog logger, so the error
	// reaches stderr rather than being swallowed
	logged := findLog(parseLogLines(stderr.Bytes()), "Error setting the log level")
	assert.NotNil(t, logged)
	assert.Equal(t, levelError, logged["level"])
	assert.Equal(t, invalidLevelErr, logged["error"])
}

func TestRootCmdRunE(t *testing.T) {
	timeout := 10 * time.Millisecond

	tests := []struct {
		Name   string
		Port   string
		Opts   []helpersgrpc.Options
		ErrMsg string
	}{
		{
			Name:   "Listener cannot be started",
			Port:   "-1",
			ErrMsg: "failed to start listener: listen tcp: address -1: invalid port",
		},
		{
			Name: "Duplicate health check is rejected",
			// Port 0 binds an ephemeral port so the test can't clash
			Port: "0",
			Opts: []helpersgrpc.Options{
				{HealthChecks: map[string]helpersgrpc.HealthCheck{
					healthService: {Timeout: &timeout},
				}},
				{HealthChecks: map[string]helpersgrpc.HealthCheck{
					healthService: {Timeout: &timeout},
				}},
			},
			ErrMsg: "health check already registered: svc",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			resetViper(t)
			captureLog(t)

			s := helpersgrpc.New("app", "description", nil, test.Opts...)
			assert.NoError(t, s.RootCmd.Flags().Set("port", test.Port))

			assert.EqualError(t, s.RootCmd.RunE(s.RootCmd, []string{}), test.ErrMsg)
		})
	}
}

func TestRootCmdHealthChecks(t *testing.T) {
	tests := []struct {
		Name    string
		Status  grpc_health_v1.HealthCheckResponse_ServingStatus
		Level   string
		Message string
	}{
		{
			Name:    "Serving is logged at debug",
			Status:  grpc_health_v1.HealthCheckResponse_SERVING,
			Level:   levelDebug,
			Message: "Running health check",
		},
		{
			Name:    "Not serving is logged at error",
			Status:  grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			Level:   levelError,
			Message: "Health check failed",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			resetViper(t)
			out := captureLog(t)
			zerolog.SetGlobalLevel(zerolog.TraceLevel)

			// A long timeout means the check runs exactly once and the
			// goroutine then parks, so it can't race the logger being restored
			timeout := time.Hour
			servers := make(chan *grpc.Server, 1)

			s := helpersgrpc.New(
				"app",
				"description",
				[]helpersgrpc.ServerFactory{func(srv *grpc.Server) { servers <- srv }},
				helpersgrpc.Options{
					HealthChecks: map[string]helpersgrpc.HealthCheck{
						healthService: {
							Timeout: &timeout,
							Check: func(*health.Server) grpc_health_v1.HealthCheckResponse_ServingStatus {
								return test.Status
							},
						},
					},
				},
			)
			// Port 0 binds an ephemeral port so parallel packages can't clash
			assert.NoError(t, s.RootCmd.Flags().Set("port", "0"))

			done := make(chan error, 1)
			go func() { done <- s.RootCmd.RunE(s.RootCmd, []string{}) }()

			// The factory runs just before the server starts serving
			srv := <-servers

			assert.Eventually(t, func() bool {
				return bytes.Contains(out.Bytes(), []byte(test.Message))
			}, time.Second*5, time.Millisecond*10)

			srv.Stop()

			// Serve returns nil once stopped, or ErrServerStopped if it was
			// stopped before it began
			if err := <-done; err != nil {
				assert.ErrorIs(t, err, grpc.ErrServerStopped)
			}

			lines := decodeLogLines(t, out)

			check := findLog(lines, test.Message)
			assert.NotNil(t, check)
			assert.Equal(t, test.Level, check["level"])
			// The status is the readable name, not the numeric enum value
			assert.Equal(t, test.Status.String(), check["status"])
			assert.Equal(t, healthService, check["service"])
			assert.Equal(t, float64(timeout.Milliseconds()), check["timeout"])

			listening := findLog(lines, "Server listening")
			assert.NotNil(t, listening)
			assert.Equal(t, levelInfo, listening["level"])
			assert.NotEmpty(t, listening["address"])
		})
	}
}

// TestNewReadsEnvironment covers the viper.AutomaticEnv binding - LOG_LEVEL and
// PORT set the flag defaults
func TestNewReadsEnvironment(t *testing.T) {
	tests := []struct {
		Name             string
		LogLevel         string
		Port             string
		ExpectedLogLevel string
		ExpectedPort     string
	}{
		{
			Name:             "LOG_LEVEL and PORT are honoured",
			LogLevel:         levelDebug,
			Port:             "9999",
			ExpectedLogLevel: levelDebug,
			ExpectedPort:     "9999",
		},
		{
			// viper coerces a non-numeric PORT to 0, which binds an ephemeral
			// port rather than failing
			Name:             "Unparseable PORT falls back to zero",
			LogLevel:         levelInfo,
			Port:             "not-a-number",
			ExpectedLogLevel: levelInfo,
			ExpectedPort:     "0",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", test.LogLevel)
			t.Setenv("PORT", test.Port)
			resetViper(t)

			s := helpersgrpc.New("app", "description", nil)

			assert.Equal(t, test.ExpectedLogLevel, s.RootCmd.PersistentFlags().Lookup(logLevelFlag).DefValue)
			assert.Equal(t, test.ExpectedPort, s.RootCmd.Flags().Lookup("port").DefValue)
		})
	}
}
