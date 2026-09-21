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

package golanghelpers

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type FatalError struct {
	Cause      error
	Msg        string
	Logger     func() *zerolog.Event
	WithParams func(l *zerolog.Event) *zerolog.Event
}

func (e FatalError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Msg
}

// Unwrap exposes the cause to errors.Is and errors.As
func (e FatalError) Unwrap() error {
	return e.Cause
}

func HandleFatalError(err error) int {
	if err == nil {
		return 0
	}

	var f FatalError
	const defaultMsg = "A fatal error occurred"
	if errors.As(err, &f) {
		if f.Msg == "" {
			f.Msg = defaultMsg
		}

		var l *zerolog.Event
		if f.Logger != nil {
			l = f.Logger()
		} else {
			l = log.Error()
		}
		if f.Cause != nil {
			l = logCause(l, f.Cause)
		}
		if f.WithParams != nil {
			l = f.WithParams(l)
		}

		l.Msg(f.Msg)
	} else {
		log.Error().Err(err).Msg(defaultMsg)
	}
	return 1
}

func logCause(l *zerolog.Event, err error) *zerolog.Event {
	// Invalid validation configuration
	var invalidErr *validator.InvalidValidationError
	if errors.As(err, &invalidErr) {
		return l.Err(err).
			Str("error_type", "invalid validation")
	}

	// Validation failed
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		fields := l.CreateArray()
		for _, fe := range validationErrs {
			fields = fields.Interface(map[string]any{
				"field":      fe.Field(),
				"ns":         fe.Namespace(),
				"tag":        fe.Tag(),
				"param":      fe.Param(),
				"value":      fe.Value(),
				"kind":       fe.Kind().String(),
				"actual_tag": fe.ActualTag(),
			})
		}

		return l.Array("validation_errors", fields).
			Int("error_count", len(validationErrs)).
			Str("error_type", "validation error")
	}

	// Normal error
	return l.Err(err)
}
