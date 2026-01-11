package main

import (
	"context"
	"log"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// StatusUpdateQueue manages parallel status updates
type StatusUpdateQueue struct {
	queue       chan *StatusUpdateTask
	workers     int
	results     chan *StatusUpdateResult
	wg          sync.WaitGroup
	stopChan    chan struct{}
	metrics     *StatusUpdateMetrics
	retryConfig RetryConfig
}

// NewStatusUpdateQueue creates a new queue with worker pool
func NewStatusUpdateQueue(workers int) *StatusUpdateQueue {
	return &StatusUpdateQueue{
		queue:       make(chan *StatusUpdateTask, workers*2),
		results:     make(chan *StatusUpdateResult, workers*2),
		workers:     workers,
		stopChan:    make(chan struct{}),
		metrics:     &StatusUpdateMetrics{},
		retryConfig: DefaultRetryConfig(),
	}
}

// Start begins processing queued tasks
func (q *StatusUpdateQueue) Start(controller *Controller) {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go func(workerID int) {
			defer q.wg.Done()
			q.worker(workerID, controller)
		}(i)
	}
}

// Stop gracefully shuts down the queue
func (q *StatusUpdateQueue) Stop() {
	close(q.stopChan)
	q.wg.Wait()
	close(q.queue)
	close(q.results)
}

// Enqueue adds a task to the queue
func (q *StatusUpdateQueue) Enqueue(task *StatusUpdateTask) {
	select {
	case q.queue <- task:
	case <-q.stopChan:
	}
}

// worker processes tasks from the queue
func (q *StatusUpdateQueue) worker(workerID int, controller *Controller) {
	for {
		select {
		case task, ok := <-q.queue:
			if !ok {
				return
			}
			q.processTask(workerID, task, controller)
		case <-q.stopChan:
			return
		}
	}
}

// processTask handles a single status update task
func (q *StatusUpdateQueue) processTask(workerID int, task *StatusUpdateTask, controller *Controller) {
	startTime := time.Now()

	cr := task.CR.(*unstructured.Unstructured)
	logger := NewStatusUpdateLogger(
		task.CorrelationID,
		cr.GetNamespace(),
		cr.GetName(),
	)

	logger.LogPhase("worker_processing_start", map[string]interface{}{
		"workerID": workerID,
	})

	result := &StatusUpdateResult{
		CRName:        cr.GetName(),
		CRNamespace:   cr.GetNamespace(),
		CorrelationID: task.CorrelationID,
		Attempts:      task.AttemptCount,
		StartTime:     startTime,
	}

	// Execute update with retries
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := controller.updateStatusWithRetry(ctx, cr, logger, q.retryConfig)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)

	if err != nil {
		result.Success = false
		result.Error = err
		result.ErrorType = classifyError(err)
		logger.LogPhase("worker_processing_failed", map[string]interface{}{
			"workerID":  workerID,
			"duration":  result.Duration.String(),
			"errorType": result.ErrorType,
		})
	} else {
		result.Success = true
		logger.LogPhase("worker_processing_success", map[string]interface{}{
			"workerID": workerID,
			"duration": result.Duration.String(),
		})
	}

	// Record metrics
	q.metrics.RecordMetric(result)

	// Send result to collector
	select {
	case q.results <- result:
	case <-q.stopChan:
	}
}

// CollectResults gathers all results from completed tasks
func (q *StatusUpdateQueue) CollectResults(expectedCount int) []StatusUpdateResult {
	var results []StatusUpdateResult
	timeout := time.NewTimer(60 * time.Second)
	defer timeout.Stop()

	for len(results) < expectedCount {
		select {
		case result := <-q.results:
			results = append(results, *result)
		case <-timeout.C:
			log.Printf("Timeout waiting for results: got %d/%d", len(results), expectedCount)
			return results
		case <-q.stopChan:
			return results
		}
	}

	return results
}

// SummarizeResults logs a summary of update results
func (q *StatusUpdateQueue) SummarizeResults(correlationID string, results []StatusUpdateResult) {
	if len(results) == 0 {
		log.Printf("[%s] No results to summarize", correlationID)
		return
	}

	var successful, failed int
	var totalDuration time.Duration
	var minDuration, maxDuration time.Duration
	errorCounts := make(map[string]int)

	minDuration = time.Duration(1<<63 - 1) // Max int64

	for _, result := range results {
		totalDuration += result.Duration
		if result.Duration < minDuration {
			minDuration = result.Duration
		}
		if result.Duration > maxDuration {
			maxDuration = result.Duration
		}

		if result.Success {
			successful++
		} else {
			failed++
			errorCounts[result.ErrorType]++
		}
	}

	avgDuration := totalDuration / time.Duration(len(results))

	log.Printf("[%s] STATUS UPDATE SUMMARY: total=%d successful=%d failed=%d avgDuration=%v minDuration=%v maxDuration=%v errorCounts=%v",
		correlationID, len(results), successful, failed, avgDuration, minDuration, maxDuration, errorCounts)

	for _, result := range results {
		if !result.Success {
			log.Printf("[%s] FAILED UPDATE: %s/%s errorType=%s error=%v attempts=%d duration=%v",
				correlationID, result.CRNamespace, result.CRName, result.ErrorType, result.Error, result.Attempts, result.Duration)
		}
	}
}
