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

import "go.temporal.io/sdk/workflow"

// Compensator is a LIFO stack of compensation functions for the mortgage saga.
//
// Usage pattern:
//  1. Declare a Compensator at the top of the workflow function.
//  2. Defer a block that calls compensate when the workflow is failing.
//  3. After each forward step succeeds, call add to register its undo.
//  4. If the workflow returns an error, the deferred block calls compensate,
//     which runs the registered functions in reverse order from a disconnected
//     context so they are not cancelled along with the failing workflow.
type Compensator struct {
	fns []func(workflow.Context) error
}

// Add registers a compensation function. Functions are called in LIFO order.
func (c *Compensator) Add(fn func(workflow.Context) error) {
	c.fns = append(c.fns, fn)
}

// Compensate runs all registered compensations in reverse order using ctx.
// Individual compensation failures are logged and do not mask the original
// workflow error.
func (c *Compensator) Compensate(ctx workflow.Context) {
	for i := len(c.fns) - 1; i >= 0; i-- {
		if err := c.fns[i](ctx); err != nil {
			workflow.GetLogger(ctx).Error("compensation step failed", "error", err)
		}
	}
}
