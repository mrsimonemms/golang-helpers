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
	"net/http"

	"github.com/gofiber/fiber/v3"
)

func FiberV3ErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := http.StatusText(code)

	var e *fiber.Error
	matched := errors.As(err, &e)
	if matched && e != nil {
		code = e.Code
	}
	if err != nil && (!matched || e != nil) {
		msg = err.Error()
	}

	return c.Status(code).JSON(fiber.Error{
		Code:    code,
		Message: msg,
	})
}
