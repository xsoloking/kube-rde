package main

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// computeStatus analyzes the current state and returns desired status
// This is separated from the update logic for clarity and testability
func (c *Controller) computeStatus(ctx context.Context, cr *unstructured.Unstructured, logger *StatusUpdateLogger) (*StatusComputationResult, error) {
	startTime := time.Now()
	result := &StatusComputationResult{
		Details: make(map[string]interface{}),
	}

	namespace := cr.GetNamespace()
	name := cr.GetName()

	// Use CR name directly as agentID since server-side already follows
	// naming convention: kuberde-[userId]-[workspaceId]-[serviceName]
	agentID := name

	result.Details["agentID"] = agentID

	// Phase 1: Get Deployment
	logger.LogPhase("get_deployment", map[string]interface{}{
		"agentID": agentID,
	})

	deploymentStartTime := time.Now()
	deployment, err := c.k8sClient.AppsV1().Deployments(namespace).Get(ctx, agentID, metav1.GetOptions{})
	deploymentDuration := time.Since(deploymentStartTime)

	if err != nil {
		logger.LogPhaseWithDuration("get_deployment_failed", deploymentDuration, map[string]interface{}{
			"error": err.Error(),
		})
		result.Phase = "Error"
		result.Message = fmt.Sprintf("Deployment not found: %v", err)
		result.ComputationTime = time.Since(startTime)
		return result, nil
	}

	desiredReplicas := int32(0)
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}
	readyReplicas := deployment.Status.ReadyReplicas

	result.Details["deployment.replicas.desired"] = desiredReplicas
	result.Details["deployment.replicas.ready"] = readyReplicas
	result.ReplicasDesired = desiredReplicas
	result.ReplicasReady = readyReplicas

	logger.LogPhaseWithDuration("get_deployment_complete", deploymentDuration, map[string]interface{}{
		"desired": desiredReplicas,
		"ready":   readyReplicas,
	})

	// Phase 2: Check replica state
	if desiredReplicas == 0 {
		result.Phase = "ScaledDown"
		result.Message = "Agent scaled down (0 replicas)"
		result.Online = false
		result.ComputationTime = time.Since(startTime)
		return result, nil
	}

	// Phase 3: Get Pod status using deployment selector
	logger.LogPhase("get_pod", map[string]interface{}{
		"agentID": agentID,
	})

	podStartTime := time.Now()
	podsClient := c.k8sClient.CoreV1().Pods(namespace)

	// Use deployment's selector to find pods (handles hashed instance labels)
	labelSelector := ""
	for k, v := range deployment.Spec.Selector.MatchLabels {
		if labelSelector != "" {
			labelSelector += ","
		}
		labelSelector += fmt.Sprintf("%s=%s", k, v)
	}

	podList, err := podsClient.List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	podDuration := time.Since(podStartTime)

	if err != nil {
		logger.LogPhaseWithDuration("get_pod_failed", podDuration, map[string]interface{}{
			"error": err.Error(),
		})
		result.Phase = "Error"
		result.Message = fmt.Sprintf("Pod check failed: %v", err)
		result.ComputationTime = time.Since(startTime)
		return result, nil
	}

	if len(podList.Items) == 0 {
		logger.LogPhaseWithDuration("get_pod_complete", podDuration, map[string]interface{}{
			"pods_found": 0,
		})

		// Check Deployment conditions for ReplicaFailure (e.g., quota exceeded)
		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
				result.Phase = "Error"
				result.Message = fmt.Sprintf("Deployment error: %s", condition.Message)
				result.Online = false
				result.ComputationTime = time.Since(startTime)
				return result, nil
			}
		}

		result.Phase = "Pending"
		result.Message = "No pods running yet"
		result.Online = false
		result.ComputationTime = time.Since(startTime)
		return result, nil
	}

	pod := podList.Items[0]
	result.PodName = pod.GetName()
	result.Details["pod.name"] = pod.GetName()
	result.Details["pod.phase"] = pod.Status.Phase

	logger.LogPhaseWithDuration("get_pod_complete", podDuration, map[string]interface{}{
		"podName": pod.GetName(),
		"phase":   pod.Status.Phase,
	})

	// Phase 4: Check pod running status
	if pod.Status.Phase != corev1.PodRunning {
		logger.LogPhase("pod_not_running", map[string]interface{}{
			"phase": pod.Status.Phase,
		})

		// Distinguish between normal startup and error states
		switch pod.Status.Phase {
		case corev1.PodPending:
			// Check container statuses to determine if it's starting or stuck
			isStarting := false
			errorMessage := ""

			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting != nil {
					reason := containerStatus.State.Waiting.Reason
					// Normal startup reasons
					if reason == "ContainerCreating" || reason == "PodInitializing" {
						isStarting = true
					}
					// Error reasons
					if reason == "ImagePullBackOff" || reason == "ErrImagePull" ||
						reason == "CrashLoopBackOff" || reason == "CreateContainerConfigError" {
						errorMessage = fmt.Sprintf("Container error: %s - %s", reason, containerStatus.State.Waiting.Message)
					}
				}
			}

			switch {
			case errorMessage != "":
				result.Phase = "Error"
				result.Message = errorMessage
			case isStarting:
				result.Phase = "Starting"
				result.Message = "Pod is starting (creating containers)"
			default:
				result.Phase = "Pending"
				result.Message = "Pod is pending"
			}

		case corev1.PodFailed:
			result.Phase = "Error"
			result.Message = fmt.Sprintf("Pod failed: %s", pod.Status.Reason)

		case corev1.PodSucceeded:
			// This shouldn't happen for long-running services, but treat as stopped
			result.Phase = "Error"
			result.Message = "Pod completed unexpectedly"

		default:
			result.Phase = "Error"
			result.Message = fmt.Sprintf("Pod in unexpected state: %s", pod.Status.Phase)
		}

		result.Online = false
		result.ComputationTime = time.Since(startTime)
		return result, nil
	}

	// Phase 5: Check agent online status
	logger.LogPhase("check_agent_online_start", map[string]interface{}{
		"agentID": agentID,
	})

	agentOnlineStart := time.Now()
	online := c.checkAgentOnline(agentID)
	onlineDuration := time.Since(agentOnlineStart)

	result.Online = online
	result.Details["agent.online"] = online
	result.Details["agent.online_check_duration"] = onlineDuration.String()

	logger.LogPhaseWithDuration("check_agent_online_complete", onlineDuration, map[string]interface{}{
		"online": online,
	})

	// Determine final phase based on online status
	if !online {
		result.Phase = "Disconnected"
		result.Message = "Agent not connected to server"
	} else {
		result.Phase = "Running"
		result.Message = "Agent is running and connected"
	}

	result.ComputationTime = time.Since(startTime)
	logger.LogPhase("compute_status_complete", map[string]interface{}{
		"phase":            result.Phase,
		"online":           result.Online,
		"totalComputeTime": result.ComputationTime.String(),
	})

	return result, nil
}

// prepareStatusMap converts StatusComputationResult to status map
func (c *Controller) prepareStatusMap(result *StatusComputationResult, cr *unstructured.Unstructured) map[string]interface{} {
	status := make(map[string]interface{})

	// Preserve existing status fields
	if existing, found := cr.Object["status"]; found {
		if existingStatus, ok := existing.(map[string]interface{}); ok {
			for k, v := range existingStatus {
				status[k] = v
			}
		}
	}

	// Update with computed values
	status["phase"] = result.Phase
	status["online"] = result.Online
	status["message"] = result.Message

	// Preserve existing lastActivity across status updates for TTL tracking
	// lastActivity represents when the agent was last actively reconciled/updated
	// It should NOT be updated on every status check, only on actual reconciliation
	// (the reconciliation process will update it when needed)
	// If it doesn't exist yet, initialize it now
	if _, exists := status["lastActivity"]; !exists {
		status["lastActivity"] = time.Now().Format(time.RFC3339)
	}

	if result.PodName != "" {
		status["podName"] = result.PodName
	}

	// Store detailed debug info
	if len(result.Details) > 0 {
		status["_debug"] = result.Details
	}

	return status
}

// diffStatus returns a map of changed fields
func (c *Controller) diffStatus(oldStatusInterface interface{}, newStatus map[string]interface{}) map[string]interface{} {
	diff := make(map[string]interface{})

	oldMap := make(map[string]interface{})
	if oldStatusInterface != nil {
		if oldMapTyped, ok := oldStatusInterface.(map[string]interface{}); ok {
			oldMap = oldMapTyped
		}
	}

	for k, newVal := range newStatus {
		oldVal, exists := oldMap[k]
		if !exists || oldVal != newVal {
			diff[k] = map[string]interface{}{
				"old": oldVal,
				"new": newVal,
			}
		}
	}

	return diff
}

// function removed as it is unused
