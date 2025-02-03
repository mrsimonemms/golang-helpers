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

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/mrsimonemms/golang-helpers/examples/grpc/basic/v1"
	"github.com/sirupsen/logrus"
)

func (c *Commands) Command2(ctx context.Context, request *basic.Command2Request) (*basic.Command2Response, error) {
	res1, err := c.client.Run(request.Input1)
	if err != nil {
		return nil, fmt.Errorf("error running db: %w", err)
	}

	sleep := time.Second * 10
	logrus.WithField("timeout", sleep).WithContext(ctx).Info("Sleeping for effect")

	time.Sleep(sleep)

	res2, err := c.client.Run(request.Input2)
	if err != nil {
		return nil, fmt.Errorf("error running db: %w", err)
	}

	return &basic.Command2Response{
		Output1: res1,
		Output2: res2,
	}, nil
}
