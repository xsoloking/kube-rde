package main

import (
	"context"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        2 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// updateStatusWithRetry attempts to update CR status with exponential backoff
func (c *Controller) updateStatusWithRetry(ctx context.Context, cr *unstructured.Unstructured,
	logger *StatusUpdateLogger, config RetryConfig) error {

	namespace := cr.GetNamespace()
	name := cr.GetName()

	var lastErr error

	for attempt := 1; attempt <= config.MaxRetries; attempt++ {
		logger.LogPhase("update_attempt_start", map[string]interface{}{
			"attempt":    attempt,
			"maxRetries": config.MaxRetries,
		})

		// Phase 1: Fetch fresh CR
		fetchStartTime := time.Now()
		freshCR, err := c.dynClient.Resource(frpAgentGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		fetchDuration := time.Since(fetchStartTime)

		if err != nil {
			logger.LogPhaseWithDuration("fetch_fresh_cr_failed", fetchDuration, map[string]interface{}{
				"attempt": attempt,
				"error":   err.Error(),
			})
			lastErr = err

			if attempt < config.MaxRetries {
				backoff := calculateBackoff(attempt-1, config)
				logger.LogPhase("backoff_before_retry", map[string]interface{}{
					"attempt": attempt,
					"backoff": backoff.String(),
				})
				time.Sleep(backoff)
			}
			continue
		}

		logger.LogPhaseWithDuration("fetch_fresh_cr_success", fetchDuration, map[string]interface{}{
			"attempt":         attempt,
			"resourceVersion": freshCR.GetResourceVersion(),
		})

		// Phase 2: Compute status based on fresh CR
		computeStartTime := time.Now()
		computeResult, err := c.computeStatus(ctx, freshCR, logger)
		computeDuration := time.Since(computeStartTime)

		if err != nil {
			logger.LogPhaseWithDuration("compute_status_failed", computeDuration, map[string]interface{}{
				"attempt": attempt,
				"error":   err.Error(),
			})
			lastErr = err

			if attempt < config.MaxRetries {
				backoff := calculateBackoff(attempt-1, config)
				time.Sleep(backoff)
			}
			continue
		}

		logger.LogPhaseWithDuration("compute_status_success", computeDuration, map[string]interface{}{
			"attempt": attempt,
			"phase":   computeResult.Phase,
			"online":  computeResult.Online,
		})

		// Phase 3: Prepare status update
		prepareStartTime := time.Now()
		oldStatusInterface := freshCR.Object["status"]
		newStatus := c.prepareStatusMap(computeResult, freshCR)
		prepareDuration := time.Since(prepareStartTime)

		// Log status diff
		statusDiff := c.diffStatus(oldStatusInterface, newStatus)
		logger.LogPhaseWithDuration("status_prepared", prepareDuration, map[string]interface{}{
			"attempt": attempt,
			"diff":    statusDiff,
		})

		freshCR.Object["status"] = newStatus

		// Phase 4: Update status subresource
		updateStartTime := time.Now()
		_, err = c.dynClient.Resource(frpAgentGVR).Namespace(namespace).UpdateStatus(ctx, freshCR, metav1.UpdateOptions{})
		updateDuration := time.Since(updateStartTime)

		if err == nil {
			logger.LogPhaseWithDuration("update_status_success", updateDuration, map[string]interface{}{
				"attempt":       attempt,
				"totalDuration": time.Since(logger.startTime).String(),
			})
			logger.LogSuccess(attempt)
			return nil
		}

		// Classify error
		errType := classifyError(err)
		logger.LogError(attempt, err, errType)
		logger.LogPhaseWithDuration("update_status_failed", updateDuration, map[string]interface{}{
			"attempt":   attempt,
			"errorType": errType,
			"error":     err.Error(),
		})

		lastErr = err

		// Don't retry non-conflict errors
		if errType != "conflict" {
			logger.LogPhase("permanent_error_no_retry", map[string]interface{}{
				"attempt":   attempt,
				"errorType": errType,
			})
			return err
		}

		// Conflict error - will retry with fresh copy
		if attempt < config.MaxRetries {
			backoff := calculateBackoff(attempt-1, config)
			logger.LogPhase("conflict_backoff_before_retry", map[string]interface{}{
				"attempt": attempt,
				"backoff": backoff.String(),
			})
			time.Sleep(backoff)
		}
	}

	logger.LogError(config.MaxRetries, lastErr, "max_retries_exceeded")
	return lastErr
}

// calculateBackoff calculates exponential backoff duration
func calculateBackoff(attemptIndex int, config RetryConfig) time.Duration {
	backoff := time.Duration(math.Pow(config.BackoffMultiplier, float64(attemptIndex))) * config.InitialBackoff
	if backoff > config.MaxBackoff {
		backoff = config.MaxBackoff
	}
	return backoff
}

// classifyError determines the type of error
func classifyError(err error) string {
	if errors.IsConflict(err) {
		return "conflict"
	}
	if errors.IsNotFound(err) {
		return "notfound"
	}
	if errors.IsServerTimeout(err) || errors.IsServiceUnavailable(err) {
		return "network"
	}
	return "other"
}
