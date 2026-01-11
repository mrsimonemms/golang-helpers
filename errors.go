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
			l = l.Err(f.Cause)
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
