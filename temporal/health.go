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

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

// Define a healthcheck listener
type healthcheck struct {
	client    client.Client
	taskQueue string
}

func (h *healthcheck) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	statusCode := http.StatusOK
	msg := "OK"

	_, err := h.client.DescribeTaskQueue(r.Context(), h.taskQueue, enums.TASK_QUEUE_TYPE_ACTIVITY)
	if err != nil {
		log.Error().Err(err).Msg("Temporal connection unhealthy")
		statusCode = http.StatusServiceUnavailable
		msg = "Down"
	} else {
		log.Debug().Msg("Temporal connection healthy")
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(msg))
}

func NewHealthCheck(ctx context.Context, taskQueue, address string, c client.Client) {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/health", &healthcheck{
			client:    c,
			taskQueue: taskQueue,
		})

		srv := &http.Server{
			Addr:         address,
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
			Handler:      mux,
		}

		log.Info().Str("address", address).Msg("Starting healthcheck service")
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("Error serving health check connection")
		}
	}()
}
