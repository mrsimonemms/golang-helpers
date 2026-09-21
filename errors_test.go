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
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-playground/validator/v10"
	golanghelpers "github.com/mrsimonemms/golang-helpers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

const (
	validationFailedMsg = "Validation failed"
	someMessage         = "Some message"
	theMessage          = "the message"
	tagRequired         = "required"
)

// validationTarget is deliberately invalid in more than one way so the
// array/count handling in logCause is genuinely exercised
type validationTarget struct {
	Name  string `validate:"required"`
	Age   int    `validate:"gte=18"`
	Email string `validate:"required,email"`
}

// loggedFieldError is the shape logCause emits for a single validation failure
type loggedFieldError struct {
	field     string
	ns        string
	tag       string
	param     string
	value     any
	kind      string
	actualTag string
}

// asJSON converts the expectations to the decoded form of the logged output
func asJSON(errs ...loggedFieldError) []any {
	out := make([]any, 0, len(errs))
	for i := range errs {
		e := &errs[i]
		out = append(out, map[string]any{
			"field":      e.field,
			"ns":         e.ns,
			"tag":        e.tag,
			"param":      e.param,
			"value":      e.value,
			"kind":       e.kind,
			"actual_tag": e.actualTag,
		})
	}
	return out
}

func TestHandleFatalError(t *testing.T) {
	validate := validator.New()

	// Multiple failures - Name is empty, Age is too low and Email is empty
	validationErrs := validate.Struct(validationTarget{Age: 10})
	assert.IsType(t, validator.ValidationErrors{}, validationErrs)

	// Validating a non-struct is a misuse of the validator
	invalidValidationErr := validate.Struct("not a struct")
	assert.IsType(t, &validator.InvalidValidationError{}, invalidValidationErr)

	tests := []struct {
		Name     string
		Error    error
		ExitCode int
		Msg      string
		Level    zerolog.Level
		Fields   func(t *testing.T, logged map[string]any)
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
			Fields: func(t *testing.T, logged map[string]any) {
				assert.Equal(t, "some error", logged["error"])
				assert.NotContains(t, logged, "error_type")
			},
		},
		{
			Name: "Fatal error - complete",
			Error: golanghelpers.FatalError{
				Cause: fmt.Errorf("some error"),
				Msg:   someMessage,
				WithParams: func(l *zerolog.Event) *zerolog.Event {
					return l.Str("hello", "world")
				},
			},
			ExitCode: 1,
			Msg:      someMessage,
			Level:    zerolog.ErrorLevel,
			Fields: func(t *testing.T, logged map[string]any) {
				// An ordinary cause uses the normal zerolog error field
				assert.Equal(t, "some error", logged["error"])
				assert.Equal(t, "world", logged["hello"])
				assert.NotContains(t, logged, "error_type")
				assert.NotContains(t, logged, "validation_errors")
				assert.NotContains(t, logged, "error_count")
			},
		},
		{
			Name: "Fatal error - wrapped",
			Error: fmt.Errorf("outer: %w", golanghelpers.FatalError{
				Cause: fmt.Errorf("some error"),
				Msg:   someMessage,
			}),
			ExitCode: 1,
			Msg:      someMessage,
			Level:    zerolog.ErrorLevel,
			Fields: func(t *testing.T, logged map[string]any) {
				// HandleFatalError uses errors.As, so the wrapper is unwrapped
				// and the FatalError still drives the output
				assert.Equal(t, "some error", logged["error"])
				assert.NotContains(t, logged, "error_type")
			},
		},
		{
			Name:     "Fatal error - empty",
			Error:    golanghelpers.FatalError{},
			ExitCode: 1,
			Msg:      "A fatal error occurred",
			Level:    zerolog.ErrorLevel,
			Fields: func(t *testing.T, logged map[string]any) {
				assert.NotContains(t, logged, "error")
				assert.NotContains(t, logged, "error_type")
			},
		},
		{
			Name: "Fatal error - invalid validation",
			Error: golanghelpers.FatalError{
				Cause: invalidValidationErr,
				Msg:   "Invalid validation",
			},
			ExitCode: 1,
			Msg:      "Invalid validation",
			Level:    zerolog.ErrorLevel,
			Fields: func(t *testing.T, logged map[string]any) {
				assert.Equal(t, "invalid validation", logged["error_type"])
				// The underlying error is still logged
				assert.Equal(t, "validator: (nil string)", logged["error"])
				assert.NotContains(t, logged, "validation_errors")
				assert.NotContains(t, logged, "error_count")
			},
		},
		{
			Name: "Fatal error - validation errors",
			Error: golanghelpers.FatalError{
				Cause: validationErrs,
				Msg:   validationFailedMsg,
				WithParams: func(l *zerolog.Event) *zerolog.Event {
					return l.Str("hello", "world")
				},
			},
			ExitCode: 1,
			Msg:      validationFailedMsg,
			Level:    zerolog.ErrorLevel,
			Fields: func(t *testing.T, logged map[string]any) {
				assert.Equal(t, "validation error", logged["error_type"])
				assert.Equal(t, float64(3), logged["error_count"])
				assert.Equal(t, "world", logged["hello"])

				// Every failure is logged, with the fields emitted by logCause
				assert.Equal(t, asJSON(
					loggedFieldError{
						field:     "Name",
						ns:        "validationTarget.Name",
						tag:       tagRequired,
						param:     "",
						value:     "",
						kind:      "string",
						actualTag: tagRequired,
					},
					loggedFieldError{
						field:     "Age",
						ns:        "validationTarget.Age",
						tag:       "gte",
						param:     "18",
						value:     float64(10),
						kind:      "int",
						actualTag: "gte",
					},
					loggedFieldError{
						field:     "Email",
						ns:        "validationTarget.Email",
						tag:       tagRequired,
						param:     "",
						value:     "",
						kind:      "string",
						actualTag: tagRequired,
					},
				), logged["validation_errors"])
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			h := &msgHook{}
			out := new(bytes.Buffer)
			prev := log.Logger
			t.Cleanup(func() { log.Logger = prev })
			log.Logger = zerolog.New(out).Hook(h)

			code := golanghelpers.HandleFatalError(test.Error)

			assert.Equal(t, test.ExitCode, code)

			if test.ExitCode > 0 {
				// Check the error that's logged
				assert.Equal(t, test.Msg, h.msg)
				assert.Equal(t, test.Level, h.level)

				// Field ordering isn't part of the contract, so decode the
				// structured output rather than compare the raw JSON
				logged := map[string]any{}
				assert.NoError(t, json.Unmarshal(out.Bytes(), &logged))
				assert.Equal(t, test.Msg, logged["message"])

				if test.Fields != nil {
					test.Fields(t, logged)
				}
			} else {
				assert.Empty(t, out.String())
			}
		})
	}
}

func TestHandleFatalErrorCustomLogger(t *testing.T) {
	out := new(bytes.Buffer)
	prev := log.Logger
	t.Cleanup(func() { log.Logger = prev })
	log.Logger = zerolog.New(out)

	validationErrs := validator.New().Struct(validationTarget{Age: 10})

	code := golanghelpers.HandleFatalError(golanghelpers.FatalError{
		Cause:  validationErrs,
		Msg:    validationFailedMsg,
		Logger: log.Warn,
	})

	assert.Equal(t, 1, code)

	logged := map[string]any{}
	assert.NoError(t, json.Unmarshal(out.Bytes(), &logged))
	assert.Equal(t, "warn", logged["level"])
	assert.Equal(t, validationFailedMsg, logged["message"])
	assert.Equal(t, "validation error", logged["error_type"])
	assert.Equal(t, float64(3), logged["error_count"])
	assert.Len(t, logged["validation_errors"], 3)
}

func TestFatalErrorError(t *testing.T) {
	tests := []struct {
		Name     string
		Error    golanghelpers.FatalError
		Expected string
	}{
		{
			Name: "The cause is preferred over the message",
			Error: golanghelpers.FatalError{
				Cause: fmt.Errorf("the cause"),
				Msg:   theMessage,
			},
			Expected: "the cause",
		},
		{
			Name:     "The message is used when there is no cause",
			Error:    golanghelpers.FatalError{Msg: theMessage},
			Expected: theMessage,
		},
		{
			Name:     "An empty error renders as empty",
			Error:    golanghelpers.FatalError{},
			Expected: "",
		},
		{
			Name: "A validation cause renders the validator message",
			Error: golanghelpers.FatalError{
				Cause: validator.New().Struct(validationTarget{Age: 10}),
				Msg:   validationFailedMsg,
			},
			Expected: validator.New().Struct(validationTarget{Age: 10}).Error(),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Expected, test.Error.Error())

			// The same string is what fmt and wrapping see
			assert.Equal(t, test.Expected, fmt.Sprintf("%v", test.Error))
			assert.Equal(t, "outer: "+test.Expected, fmt.Errorf("outer: %w", test.Error).Error())
		})
	}
}

func TestFatalErrorUnwrapping(t *testing.T) {
	cause := fmt.Errorf("the cause")
	fatal := golanghelpers.FatalError{Cause: cause, Msg: theMessage}

	t.Run("errors.As finds it through a wrapper", func(t *testing.T) {
		var found golanghelpers.FatalError
		assert.ErrorAs(t, fmt.Errorf("outer: %w", fatal), &found)
		assert.Equal(t, theMessage, found.Msg)
		assert.Equal(t, cause, found.Cause)
	})

	t.Run("The cause is not reachable via errors.Is", func(t *testing.T) {
		// FatalError has no Unwrap method, so the cause is only available
		// through the exported field
		assert.NotErrorIs(t, fatal, cause)
	})

	t.Run("errors.Is never matches a FatalError", func(t *testing.T) {
		// The Logger and WithParams func fields make FatalError
		// non-comparable, so errors.Is can't match it even against itself.
		// errors.As is the only way to recover one. Reported separately
		assert.NotErrorIs(t, fmt.Errorf("outer: %w", fatal), fatal)
		assert.NotErrorIs(t, fatal, fatal)
	})
}

type msgHook struct {
	level zerolog.Level
	msg   string
}

func (h *msgHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	h.level = level
	h.msg = msg
}
