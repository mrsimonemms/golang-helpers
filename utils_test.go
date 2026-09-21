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
	"testing"
	"time"

	golanghelpers "github.com/mrsimonemms/golang-helpers"
	"github.com/stretchr/testify/assert"
)

// Ptr is generic, so each case needs its own type rather than a table row
func TestPtr(t *testing.T) {
	t.Run("Dereferences back to the original value", func(t *testing.T) {
		assert.Equal(t, "hello", *golanghelpers.Ptr("hello"))
		assert.Equal(t, 42, *golanghelpers.Ptr(42))
		assert.InDelta(t, 1.5, *golanghelpers.Ptr(1.5), 0)
		assert.True(t, *golanghelpers.Ptr(true))
	})

	t.Run("Zero values are addressable", func(t *testing.T) {
		assert.Empty(t, *golanghelpers.Ptr(""))
		assert.Zero(t, *golanghelpers.Ptr(0))
		assert.False(t, *golanghelpers.Ptr(false))
	})

	t.Run("Nil-able types are wrapped rather than collapsed", func(t *testing.T) {
		slice := golanghelpers.Ptr[[]int](nil)
		assert.NotNil(t, slice)
		assert.Nil(t, *slice)

		m := golanghelpers.Ptr[map[string]string](nil)
		assert.NotNil(t, m)
		assert.Nil(t, *m)
	})

	t.Run("Structs are copied, not aliased", func(t *testing.T) {
		type thing struct{ Name string }

		original := thing{Name: "before"}
		p := golanghelpers.Ptr(original)

		p.Name = "after"

		assert.Equal(t, "after", p.Name)
		assert.Equal(t, "before", original.Name, "the source value must be untouched")
	})

	t.Run("Each call returns a distinct pointer", func(t *testing.T) {
		a := golanghelpers.Ptr(1)
		b := golanghelpers.Ptr(1)

		assert.NotSame(t, a, b)
		assert.Equal(t, *a, *b)
	})

	// The grpc package relies on this to default a *time.Duration
	t.Run("Durations round-trip", func(t *testing.T) {
		timeout := golanghelpers.Ptr(time.Second * 10)

		assert.NotNil(t, timeout)
		assert.Equal(t, time.Second*10, *timeout)
	})
}
