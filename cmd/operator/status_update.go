package main

import (
	"log"
	"sync"
	"time"
)

// StatusUpdateTask represents a single CR status update task
type StatusUpdateTask struct {
	CR            interface{} // *unstructured.Unstructured
	CorrelationID string
	CreatedAt     time.Time
	LastAttemptAt time.Time
	AttemptCount  int
}

// StatusUpdateResult represents the outcome of a status update
type StatusUpdateResult struct {
	CRName        string
	CRNamespace   string
	CorrelationID string
	Success       bool
	Error         error
	ErrorType     string // "conflict", "notfound", "network", "other"
	Duration      time.Duration
	Attempts      int
	StartTime     time.Time
	EndTime       time.Time
}

// StatusUpdateMetrics tracks statistics for status updates
type StatusUpdateMetrics struct {
	mu                sync.RWMutex
	totalAttempts     int64
	successfulUpdates int64
	failedUpdates     int64
	conflictErrors    int64
	networkErrors     int64
	totalDuration     time.Duration
	maxLatency        time.Duration
	minLatency        time.Duration
	// field removed as it is unused
	lastUpdateCycleDuration time.Duration
	successRate             float64
}

// StatusComputationResult contains computed status for a CR
type StatusComputationResult struct {
	Phase           string // Running, ScaledDown, Error, Disconnected, Pending
	Online          bool
	Message         string
	PodName         string
	ReplicasDesired int32
	ReplicasReady   int32
	ComputationTime time.Duration
	Details         map[string]interface{} // Additional info for logging
}

// StatusUpdateLogger provides structured logging with correlation IDs
type StatusUpdateLogger struct {
	correlationID string
	crNamespace   string
	crName        string
	startTime     time.Time
}

// NewStatusUpdateLogger creates a new logger for a status update operation
func NewStatusUpdateLogger(correlationID, namespace, name string) *StatusUpdateLogger {
	return &StatusUpdateLogger{
		correlationID: correlationID,
		crNamespace:   namespace,
		crName:        name,
		startTime:     time.Now(),
	}
}

// LogPhase logs a phase with current duration
func (l *StatusUpdateLogger) LogPhase(phase string, details map[string]interface{}) {
	duration := time.Since(l.startTime)
	logWithCorrelation(l.correlationID, l.crNamespace, l.crName, phase, duration, details)
}

// LogPhaseWithDuration logs a specific phase duration
func (l *StatusUpdateLogger) LogPhaseWithDuration(phase string, phaseDuration time.Duration, details map[string]interface{}) {
	elapsed := time.Since(l.startTime)
	if details == nil {
		details = make(map[string]interface{})
	}
	details["phaseDuration"] = phaseDuration.String()
	details["totalElapsed"] = elapsed.String()
	logWithCorrelation(l.correlationID, l.crNamespace, l.crName, phase, elapsed, details)
}

// LogError logs an error with attempt number
func (l *StatusUpdateLogger) LogError(attempt int, err error, errType string) {
	duration := time.Since(l.startTime)
	details := map[string]interface{}{
		"attempt":   attempt,
		"error":     err.Error(),
		"errorType": errType,
	}
	logWithCorrelation(l.correlationID, l.crNamespace, l.crName, "error", duration, details)
}

// LogSuccess logs a successful update
func (l *StatusUpdateLogger) LogSuccess(attempt int) {
	duration := time.Since(l.startTime)
	details := map[string]interface{}{
		"attempt": attempt,
	}
	logWithCorrelation(l.correlationID, l.crNamespace, l.crName, "SUCCESS", duration, details)
}

// logWithCorrelation is a helper function to log with correlation ID
func logWithCorrelation(correlationID, namespace, crName, phase string, duration time.Duration, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["phase"] = phase
	details["duration"] = duration.String()
	log.Printf("[%s] %s/%s %v", correlationID, namespace, crName, details)
}

// RecordMetric records a metric for the given result
func (m *StatusUpdateMetrics) RecordMetric(result *StatusUpdateResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalAttempts++

	if result.Success {
		m.successfulUpdates++
	} else {
		m.failedUpdates++

		switch result.ErrorType {
		case "conflict":
			m.conflictErrors++
		case "network":
			m.networkErrors++
		}
	}

	m.totalDuration += result.Duration

	if result.Duration > m.maxLatency || m.maxLatency == 0 {
		m.maxLatency = result.Duration
	}
	if result.Duration < m.minLatency || m.minLatency == 0 {
		m.minLatency = result.Duration
	}

	if m.totalAttempts > 0 {
		m.successRate = float64(m.successfulUpdates) / float64(m.totalAttempts)
	}
}

// GetMetricsSnapshot returns a snapshot of current metrics
func (m *StatusUpdateMetrics) GetMetricsSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgLatency := time.Duration(0)
	if m.totalAttempts > 0 {
		avgLatency = m.totalDuration / time.Duration(m.totalAttempts)
	}

	return map[string]interface{}{
		"totalAttempts":     m.totalAttempts,
		"successfulUpdates": m.successfulUpdates,
		"failedUpdates":     m.failedUpdates,
		"conflictErrors":    m.conflictErrors,
		"networkErrors":     m.networkErrors,
		"avgLatency":        avgLatency.String(),
		"maxLatency":        m.maxLatency.String(),
		"minLatency":        m.minLatency.String(),
		"successRate":       m.successRate,
		"lastCycleDuration": m.lastUpdateCycleDuration.String(),
	}
}
