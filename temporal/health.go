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
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

type healthcheck struct {
	client     client.Client
	taskQueues []string
	timeout    time.Duration
}

type liveResponse struct {
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
}

type taskQueueTypeHealth struct {
	Type    string `json:"type"`
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
}

type taskQueueHealth struct {
	TaskQueue string                `json:"taskQueue"`
	Healthy   bool                  `json:"healthy"`
	Checks    []taskQueueTypeHealth `json:"checks"`
}

type readyResponse struct {
	Healthy    bool              `json:"healthy"`
	TemporalOK bool              `json:"temporalOk"`
	Error      string            `json:"error,omitempty"`
	TaskQueues []taskQueueHealth `json:"taskQueues,omitempty"`
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

func taskQueueTypeName(taskQueueType enumspb.TaskQueueType) string {
	switch taskQueueType {
	case enumspb.TASK_QUEUE_TYPE_WORKFLOW:
		return "workflow"
	case enumspb.TASK_QUEUE_TYPE_ACTIVITY:
		return "activity"
	default:
		return "unknown"
	}
}

func (h *healthcheck) checkTemporal(ctx context.Context) error {
	_, err := h.client.CheckHealth(ctx, &client.CheckHealthRequest{})
	return err
}

func (h *healthcheck) checkTaskQueue(
	ctx context.Context,
	taskQueue string,
	taskQueueType enumspb.TaskQueueType,
) taskQueueTypeHealth {
	_, err := h.client.DescribeTaskQueue(ctx, taskQueue, taskQueueType)
	if err != nil {
		log.Error().
			Err(err).
			Str("taskQueue", taskQueue).
			Str("taskQueueType", taskQueueTypeName(taskQueueType)).
			Msg("Temporal task queue unhealthy")

		return taskQueueTypeHealth{
			Type:    taskQueueTypeName(taskQueueType),
			Healthy: false,
			Error:   err.Error(),
		}
	}

	log.Debug().
		Str("taskQueue", taskQueue).
		Str("taskQueueType", taskQueueTypeName(taskQueueType)).
		Msg("Temporal task queue healthy")

	return taskQueueTypeHealth{
		Type:    taskQueueTypeName(taskQueueType),
		Healthy: true,
	}
}

func (h *healthcheck) serveLiveness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if err := h.checkTemporal(ctx); err != nil {
		log.Error().Err(err).Msg("Temporal liveness check failed")
		writeJSON(w, http.StatusServiceUnavailable, liveResponse{
			Healthy: false,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, liveResponse{
		Healthy: true,
	})
}

func (h *healthcheck) serveReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	resp := readyResponse{
		Healthy:    true,
		TemporalOK: true,
		TaskQueues: make([]taskQueueHealth, 0, len(h.taskQueues)),
	}

	if err := h.checkTemporal(ctx); err != nil {
		log.Error().Err(err).Msg("Temporal readiness health check failed")
		resp.Healthy = false
		resp.TemporalOK = false
		resp.Error = err.Error()

		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}

	for _, tq := range h.taskQueues {
		checks := []taskQueueTypeHealth{
			h.checkTaskQueue(ctx, tq, enumspb.TASK_QUEUE_TYPE_WORKFLOW),
			h.checkTaskQueue(ctx, tq, enumspb.TASK_QUEUE_TYPE_ACTIVITY),
		}

		taskQueueHealthy := true
		for _, check := range checks {
			if !check.Healthy {
				taskQueueHealthy = false
				resp.Healthy = false
			}
		}

		resp.TaskQueues = append(resp.TaskQueues, taskQueueHealth{
			TaskQueue: tq,
			Healthy:   taskQueueHealthy,
			Checks:    checks,
		})
	}

	statusCode := http.StatusOK
	if !resp.Healthy {
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, resp)
}

func (h *healthcheck) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/livez":
		h.serveLiveness(w, r)
	case "/readyz", "/health":
		h.serveReadiness(w, r)
	default:
		http.NotFound(w, r)
	}
}

func NewHealthCheck(ctx context.Context, taskQueues []string, address string, c client.Client) {
	h := &healthcheck{
		client:     c,
		taskQueues: taskQueues,
		timeout:    2 * time.Second,
	}

	mux := http.NewServeMux()
	mux.Handle("/", h)

	srv := &http.Server{
		Addr:              address,
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		Handler:           mux,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("Error shutting down healthcheck service")
		}
	}()

	go func() {
		log.Info().
			Str("address", address).
			Int("taskQueueCount", len(taskQueues)).
			Msg("Starting healthcheck service")

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("Error serving health check connection")
		}
	}()
}
