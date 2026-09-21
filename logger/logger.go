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

// Package logger contains helpers around Logrus, originally intended to make
// it behave more like Zerolog.
//
// Deprecated: this package is retained for backwards compatibility and
// should not be used in new code.
package logger

import (
	"strings"

	"github.com/sirupsen/logrus"
)

// Logger is default instance of logger used in all other packages
// instead of global scope logrus.Logger.
//
// Deprecated: this package is retained for backwards compatibility and
// should not be used in new code.
var Logger *logrus.Logger

func init() {
	Logger = logrus.New()
}

// Deprecated: this package is retained for backwards compatibility and
// should not be used in new code.
func GetAllLevels() string {
	l := []string{}
	for _, s := range logrus.AllLevels {
		l = append(l, s.String())
	}

	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}

	return strings.Join(l, ", ")
}

// Deprecated: this package is retained for backwards compatibility and
// should not be used in new code.
func SetLevel(logLevel string) error {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}

	Logger.SetLevel(level)
	return nil
}

// Log is used to return the default Logger.
//
// Deprecated: this package is retained for backwards compatibility and
// should not be used in new code.
func Log() *logrus.Logger {
	return Logger
}
