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

package golanghelpers_test

import (
	"fmt"
	"testing"

	golanghelpers "github.com/mrsimonemms/golang-helpers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestHandleFatalError(t *testing.T) {
	tests := []struct {
		Name     string
		Error    error
		ExitCode int
		Msg      string
		Level    zerolog.Level
	}{
		{
			Name:     "No error",
			Error:    nil,
			ExitCode: 0,
		},
		{
			Name:     "Standard error",
			Error:    fmt.Errorf("some error"),
			ExitCode: 1,
			Msg:      "A fatal error occurred",
			Level:    zerolog.ErrorLevel,
		},
		{
			Name: "Fatal error - complete",
			Error: golanghelpers.FatalError{
				Cause: fmt.Errorf("some error"),
				Msg:   "Some message",
				WithParams: func(l *zerolog.Event) *zerolog.Event {
					return l.Str("hello", "world")
				},
			},
			ExitCode: 1,
			Msg:      "Some message",
			Level:    zerolog.ErrorLevel,
		},
		{
			Name:     "Fatal error - empty",
			Error:    golanghelpers.FatalError{},
			ExitCode: 1,
			Msg:      "A fatal error occurred",
			Level:    zerolog.ErrorLevel,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			h := &msgHook{}
			prev := log.Logger
			t.Cleanup(func() { log.Logger = prev })
			log.Logger = log.Logger.Hook(h)

			code := golanghelpers.HandleFatalError(test.Error)

			assert.Equal(t, test.ExitCode, code)

			if test.ExitCode > 0 {
				// Check the error that's logged
				assert.Equal(t, test.Msg, h.msg)
				assert.Equal(t, test.Level, h.level)
			}
		})
	}
}

type msgHook struct {
	level zerolog.Level
	msg   string
}

func (h *msgHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	h.level = level
	h.msg = msg
}
