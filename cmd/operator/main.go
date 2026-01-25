package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// generateShortHash creates a short hash of a string for use in Kubernetes labels
// Kubernetes labels have a 63 character limit, so we use a hash for long names
func generateShortHash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 characters (32 hex chars = 16 bytes)
}

var frpAgentGVR = schema.GroupVersionResource{
	Group:    "kuberde.io",
	Version:  "v1beta1",
	Resource: "rdeagents",
}

type Controller struct {
	k8sClient kubernetes.Interface
	dynClient dynamic.Interface
	informer  cache.SharedIndexInformer
}

func main() {
	log.Println("Starting FRP Operator...")

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error getting in-cluster config: %v", err)
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating K8s client: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating dynamic client: %v", err)
	}

	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, time.Second*60)
	informer := factory.ForResource(frpAgentGVR).Informer()

	controller := &Controller{
		k8sClient: k8sClient,
		dynClient: dynClient,
		informer:  informer,
	}

	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.onAdd,
		UpdateFunc: func(old, new interface{}) { controller.onAdd(new) },
		DeleteFunc: controller.onDelete,
	})
	if err != nil {
		log.Fatalf("Error adding event handler: %v", err)
	}

	stop := make(chan struct{})
	go factory.Start(stop)
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		log.Fatalf("Failed to sync cache")
	}

	go controller.runServerSessionMonitor(context.Background())
	go controller.runTTLChecker(context.Background())
	go controller.runStatusUpdater(context.Background())

	// Start health check HTTP server
	go startHealthCheckServer(controller)

	// Wait for signals
	stop2 := make(chan os.Signal, 1)
	signal.Notify(stop2, syscall.SIGINT, syscall.SIGTERM)
	<-stop2
	log.Println("Shutting down...")
}

// startHealthCheckServer starts an HTTP server for health checks
func startHealthCheckServer(controller *Controller) {
	http.HandleFunc("/healthz", handleHealthz)
	http.HandleFunc("/livez", handleHealthz)
	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		handleReadyz(w, r, controller)
	})

	port := os.Getenv("HEALTH_CHECK_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting health check server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Printf("Health check server error: %v", err)
	}
}

// handleHealthz handles liveness probe (basic operator health)
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "kuberde-operator",
	}); err != nil {
		log.Printf("Failed to encode healthz response: %v", err)
	}
}

// handleReadyz handles readiness probe (checks if operator is ready to reconcile)
func handleReadyz(w http.ResponseWriter, r *http.Request, controller *Controller) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if informer has synced
	if controller.informer != nil && !controller.informer.HasSynced() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "not ready",
			"reason": "informer not synced",
		}); err != nil {
			log.Printf("Failed to encode readyz response: %v", err)
		}
		return
	}

	// Check Kubernetes client connectivity by listing a resource we have permission for
	if controller.k8sClient != nil {
		// Get operator's namespace from environment
		namespace := os.Getenv("OPERATOR_NAMESPACE")
		if namespace == "" {
			namespace = "kuberde"
		}

		// Try to list pods in operator's namespace (we have permission for this)
		_, err := controller.k8sClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{Limit: 1})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			if err := json.NewEncoder(w).Encode(map[string]string{
				"status": "not ready",
				"reason": "kubernetes client not accessible",
			}); err != nil {
				log.Printf("Failed to encode readyz response: %v", err)
			}
			return
		}
	}

	// All checks passed
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "ready",
		"service": "kuberde-operator",
	}); err != nil {
		log.Printf("Failed to encode readyz response: %v", err)
	}
}

func (c *Controller) onAdd(obj interface{}) {
	u := obj.(*unstructured.Unstructured)
	log.Printf("Reconciling RDEAgent: %s/%s", u.GetNamespace(), u.GetName())

	if err := c.reconcileDeployment(u); err != nil {
		log.Printf("Error reconciling Deployment: %v", err)
		return
	}

	// After reconciliation, check if we need to reset lastActivity
	// This happens when an agent is being scaled back up after TTL scaled it down
	// Strategy: Check if status phase was "ScaledDown" but we're now reconciling
	// (which means someone explicitly tried to activate it again)
	name := u.GetName()
	namespace := u.GetNamespace()

	// Use CR name directly as agentID
	agentID := name

	shouldResetActivity := false
	status, found := u.Object["status"].(map[string]interface{})
	if found && status != nil {
		if phase, ok := status["phase"].(string); ok {
			// If it was previously ScaledDown but someone triggered a reconciliation,
			// it means they want to activate it again - reset the timer
			if phase == "ScaledDown" {
				shouldResetActivity = true
				log.Printf("Agent %s was ScaledDown but is being reconciled again, resetting lastActivity", agentID)
			}
		}
	}

	if shouldResetActivity {
		agentCR, err := c.dynClient.Resource(frpAgentGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			if agentCR.Object["status"] == nil {
				agentCR.Object["status"] = make(map[string]interface{})
			}
			statusMap := agentCR.Object["status"].(map[string]interface{})
			statusMap["lastActivity"] = time.Now().Format(time.RFC3339)

			_, err = c.dynClient.Resource(frpAgentGVR).Namespace(namespace).UpdateStatus(context.TODO(), agentCR, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Warning: failed to reset lastActivity for agent %s/%s: %v", namespace, name, err)
			} else {
				log.Printf("Reset lastActivity for agent %s/%s due to scale-up from ScaledDown", namespace, name)
			}
		}
	}

	// Don't update full status here - let the periodic status updater handle it
	// This avoids race conditions with the API server cache during initial object creation
	log.Printf("onAdd: finished reconciliation for %s/%s", u.GetNamespace(), u.GetName())
}

func (c *Controller) onDelete(obj interface{}) {
	u := obj.(*unstructured.Unstructured)
	log.Printf("RDEAgent deleted: %s/%s", u.GetNamespace(), u.GetName())
	// K8s OwnerReferences will handle Deployment deletion automatically
}

// workloadContainerConfig holds parsed workload container configuration
type workloadContainerConfig struct {
	image           string
	command         []string
	args            []string
	ports           []corev1.ContainerPort
	imagePullPolicy corev1.PullPolicy
	env             []corev1.EnvVar
	resources       corev1.ResourceRequirements
	securityContext *corev1.SecurityContext
}

// parseWorkloadContainer extracts and validates workload container configuration
func (c *Controller) parseWorkloadContainer(cr *unstructured.Unstructured, name string) (*workloadContainerConfig, error) {
	workloadContainer, found, _ := unstructured.NestedMap(cr.Object, "spec", "workloadContainer")
	if !found {
		return nil, fmt.Errorf("workloadContainer is required")
	}

	config := &workloadContainerConfig{}

	image, _ := workloadContainer["image"].(string)
	if image == "" {
		log.Printf("Error: workloadContainer.image is required for agent %s", name)
		return nil, fmt.Errorf("workloadContainer.image is required")
	}
	config.image = image

	config.command = c.extractStringSlice(workloadContainer, "command")
	config.args = c.extractStringSlice(workloadContainer, "args")
	config.ports = c.extractContainerPorts(workloadContainer, name)
	config.imagePullPolicy = c.extractImagePullPolicy(workloadContainer)
	config.env = c.extractEnvVars(workloadContainer, name)
	config.resources = c.extractResources(workloadContainer)
	config.securityContext = c.extractSecurityContext(workloadContainer, name)

	return config, nil
}

// extractStringSlice extracts a string slice from a map
func (c *Controller) extractStringSlice(m map[string]interface{}, key string) []string {
	var result []string
	if slice, found := m[key].([]interface{}); found {
		for _, item := range slice {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	}
	return result
}

// extractContainerPorts extracts container ports from workload container spec
func (c *Controller) extractContainerPorts(workloadContainer map[string]interface{}, name string) []corev1.ContainerPort {
	var ports []corev1.ContainerPort
	portsList, found := workloadContainer["ports"].([]interface{})
	if !found {
		log.Printf("No ports found in CR for %s, defaulting to 8080", name)
		return []corev1.ContainerPort{{ContainerPort: 8080}}
	}

	for _, p := range portsList {
		portMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		containerPort := c.extractInt32Port(portMap["containerPort"])
		if containerPort <= 0 {
			continue
		}

		portObj := corev1.ContainerPort{ContainerPort: containerPort}
		if name, ok := portMap["name"].(string); ok && name != "" {
			portObj.Name = name
		}
		if protocol, ok := portMap["protocol"].(string); ok && protocol != "" {
			portObj.Protocol = corev1.Protocol(protocol)
		}
		ports = append(ports, portObj)
		log.Printf("Extracted port %d for container from CR", containerPort)
	}

	if len(ports) == 0 {
		log.Printf("No valid ports found in CR for %s, defaulting to 8080", name)
		return []corev1.ContainerPort{{ContainerPort: 8080}}
	}

	log.Printf("Using ports from CR for %s: %+v", name, ports)
	return ports
}

// extractInt32Port converts various numeric types to int32
func (c *Controller) extractInt32Port(val interface{}) int32 {
	switch v := val.(type) {
	case float64:
		return int32(v)
	case int64:
		return int32(v)
	case int:
		return int32(v)
	default:
		return 0
	}
}

// extractImagePullPolicy extracts image pull policy from workload container
func (c *Controller) extractImagePullPolicy(workloadContainer map[string]interface{}) corev1.PullPolicy {
	if policy, found := workloadContainer["imagePullPolicy"].(string); found {
		return corev1.PullPolicy(policy)
	}
	return corev1.PullIfNotPresent
}

// extractEnvVars extracts environment variables from workload container
func (c *Controller) extractEnvVars(workloadContainer map[string]interface{}, name string) []corev1.EnvVar {
	var env []corev1.EnvVar
	wcEnvList, found := workloadContainer["env"].([]interface{})
	if !found {
		return env
	}

	for _, e := range wcEnvList {
		envMap, ok := e.(map[string]interface{})
		if !ok {
			continue
		}

		envName, _ := envMap["name"].(string)
		envValue, _ := envMap["value"].(string)
		if envName != "" {
			env = append(env, corev1.EnvVar{Name: envName, Value: envValue})
			log.Printf("Extracted env var from workloadContainer for %s: %s", name, envName)
		}
	}
	return env
}

// extractResources extracts resource requirements from workload container
func (c *Controller) extractResources(workloadContainer map[string]interface{}) corev1.ResourceRequirements {
	if resMap, found := workloadContainer["resources"].(map[string]interface{}); found {
		return c.parseResources(resMap)
	}
	return corev1.ResourceRequirements{}
}

// extractSecurityContext extracts security context from workload container
func (c *Controller) extractSecurityContext(workloadContainer map[string]interface{}, name string) *corev1.SecurityContext {
	if scMap, found := workloadContainer["securityContext"].(map[string]interface{}); found {
		log.Printf("Extracted securityContext from workloadContainer for %s: %+v", name, scMap)
		return c.parseSecurityContext(scMap)
	}
	return nil
}

func (c *Controller) reconcileDeployment(cr *unstructured.Unstructured) error {
	spec, _, _ := unstructured.NestedMap(cr.Object, "spec")
	name := cr.GetName()
	namespace := cr.GetNamespace()

	serverUrl, _ := spec["serverUrl"].(string)
	owner, _ := spec["owner"].(string)
	authSecret, _ := spec["authSecret"].(string)
	localTarget, _ := spec["localTarget"].(string)

	if localTarget == "" {
		localTarget = "localhost:80"
	}

	// Parse workload container configuration
	workloadConfig, err := c.parseWorkloadContainer(cr, name)
	if err != nil {
		return err
	}

	// Construct Agent ID and Deployment Name
	// Use CR name directly as agentID since server-side already follows
	// naming convention: kuberde-[userId]-[workspaceId]-[serviceName]
	agentID := name

	// Reconcile Secrets for SSH credentials
	if err := c.reconcileSecrets(cr, agentID); err != nil {
		log.Printf("Error reconciling Secrets for agent %s: %v", agentID, err)
		return err
	}

	// Parse Env

	var workloadEnv []corev1.EnvVar

	if envSlice, found, _ := unstructured.NestedSlice(cr.Object, "spec", "env"); found {

		for _, e := range envSlice {

			if envMap, ok := e.(map[string]interface{}); ok {

				name, _ := envMap["name"].(string)

				value, _ := envMap["value"].(string)

				if name != "" {

					workloadEnv = append(workloadEnv, corev1.EnvVar{Name: name, Value: value})

				}

			}

		}

	}

	// Merge workloadContainer.env into workloadEnv
	// workloadContainer.env values override spec.env values for the same variable name
	if len(workloadConfig.env) > 0 {
		// Create a map of existing env vars for quick lookup
		envMap := make(map[string]corev1.EnvVar)
		for _, ev := range workloadEnv {
			envMap[ev.Name] = ev
		}

		// Merge workloadConfig.env, overriding existing values
		for _, wcEnv := range workloadConfig.env {
			envMap[wcEnv.Name] = wcEnv
		}

		// Convert back to slice
		workloadEnv = []corev1.EnvVar{}
		for _, ev := range envMap {
			workloadEnv = append(workloadEnv, ev)
		}
		log.Printf("Merged workloadContainer.env into workloadEnv for %s", name)
	}

	// Enforce SSH Config
	log.Printf("SSH Config check for agent %s: owner='%s' (len=%d)", agentID, owner, len(owner))

	// Handle SSH Public Keys - import from secret as SSH_PUBLIC_KEY
	if owner != "" {
		if keysSlice, found, _ := unstructured.NestedSlice(cr.Object, "spec", "sshPublicKeys"); found && len(keysSlice) > 0 {
			// SSH public keys exist in CR, add environment variable from secret
			secretName := fmt.Sprintf("%s-credentials", agentID)
			workloadEnv = append(workloadEnv, corev1.EnvVar{
				Name: "SSH_PUBLIC_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: "PUBLIC_KEY",
					},
				},
			})
			log.Printf("Added SSH_PUBLIC_KEY from secret %s for agent %s", secretName, agentID)
		}
	}

	// Extract storage configuration
	// Prefer using pvcName from spec (workspace PVC) over storage array
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume

	// Check for pvcName field in spec (new approach - use existing workspace PVC)
	if pvcName, found, _ := unstructured.NestedString(cr.Object, "spec", "pvcName"); found && pvcName != "" {
		// Extract volumeMounts from workloadContainer spec
		if volumeMountsList, found, _ := unstructured.NestedSlice(cr.Object, "spec", "workloadContainer", "volumeMounts"); found {
			// Use volumeMounts from CR spec
			for _, vm := range volumeMountsList {
				if vmMap, ok := vm.(map[string]interface{}); ok {
					volumeMount := corev1.VolumeMount{}
					if name, ok := vmMap["name"].(string); ok {
						volumeMount.Name = name
					}
					if mountPath, ok := vmMap["mountPath"].(string); ok {
						volumeMount.MountPath = mountPath
					}
					if readOnly, ok := vmMap["readOnly"].(bool); ok {
						volumeMount.ReadOnly = readOnly
					}
					if subPath, ok := vmMap["subPath"].(string); ok {
						volumeMount.SubPath = subPath
					}
					volumeMounts = append(volumeMounts, volumeMount)
				}
			}
			log.Printf("Using volumeMounts from CR spec for agent %s: %v", agentID, volumeMounts)
		} else {
			// Fallback to default /root mount if volumeMounts not specified in CR
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      "workspace",
				MountPath: "/root",
			})
			log.Printf("Using default volumeMount /root for agent %s (no volumeMounts in CR)", agentID)
		}

		volumes = append(volumes, corev1.Volume{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
		log.Printf("Using existing workspace PVC %s for agent %s", pvcName, agentID)
	} else {
		// Legacy storage configuration (deprecated - for backward compatibility)
		var storageSpecs []map[string]interface{}
		if storageList, found, _ := unstructured.NestedSlice(cr.Object, "spec", "storage"); found {
			for _, s := range storageList {
				if storageMap, ok := s.(map[string]interface{}); ok {
					storageSpecs = append(storageSpecs, storageMap)

					// Extract storage fields
					storageName, _ := storageMap["name"].(string)
					mountPath, _ := storageMap["mountPath"].(string)

					if storageName != "" && mountPath != "" {
						pvcName := fmt.Sprintf("kuberde-agent-%s-%s", agentID, storageName)

						// Add volume mount for workload container
						volumeMounts = append(volumeMounts, corev1.VolumeMount{
							Name:      storageName,
							MountPath: mountPath,
						})

						// Add volume for PodSpec
						volumes = append(volumes, corev1.Volume{
							Name: storageName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						})
					}
				}
			}
		}
		// Only reconcile storage if using legacy storage array (not pvcName)
		if len(storageSpecs) > 0 {
			if err := c.reconcileStorage(cr, agentID); err != nil {
				return err
			}
		}
	}

	// Extract nodeSelector configuration
	nodeSelector := make(map[string]string)
	if nodeSelectorMap, found, _ := unstructured.NestedMap(cr.Object, "spec", "nodeSelector"); found {
		for k, v := range nodeSelectorMap {
			if strVal, ok := v.(string); ok {
				nodeSelector[k] = strVal
			}
		}
	}

	// Extract tolerations configuration
	var tolerations []corev1.Toleration
	if tolerationsList, found, _ := unstructured.NestedSlice(cr.Object, "spec", "tolerations"); found {
		for _, t := range tolerationsList {
			if tolMap, ok := t.(map[string]interface{}); ok {
				tol := corev1.Toleration{}
				if key, ok := tolMap["key"].(string); ok {
					tol.Key = key
				}
				if operator, ok := tolMap["operator"].(string); ok {
					tol.Operator = corev1.TolerationOperator(operator)
				}
				if value, ok := tolMap["value"].(string); ok {
					tol.Value = value
				}
				if effect, ok := tolMap["effect"].(string); ok {
					tol.Effect = corev1.TaintEffect(effect)
				}
				if tolerationSeconds, ok := tolMap["tolerationSeconds"].(int64); ok {
					tol.TolerationSeconds = &tolerationSeconds
				}
				tolerations = append(tolerations, tol)
			}
		}
	}

	// Check if Deployment exists (using agentID as name)
	deploymentsClient := c.k8sClient.AppsV1().Deployments(namespace)

	log.Printf("reconcileDeployment: attempting to get Deployment %s", agentID)
	existingDeployment, err := deploymentsClient.Get(context.TODO(), agentID, metav1.GetOptions{})
	log.Printf("reconcileDeployment: get result - err=%v, found=%v", err, err == nil)

	// Build the expected Deployment spec
	// Use hash for instance label to satisfy Kubernetes 63-character limit
	instanceHash := generateShortHash(agentID)
	labels := map[string]string{"app": "kuberde-agent", "instance": instanceHash}

	// Store full agentID in annotation for reference
	annotations := map[string]string{"kuberde.io/agent-id": agentID}

	// NOTE: Replicas handling:
	// - Default desired state: replicas=1
	// - If already scaled down by TTL (replicas=0), preserve that state in spec
	// - TTL enforcement (scale to 0) is handled by independent TTL enforcer loop
	// - Server session monitor handles auto scale-up when session resumes
	replicas := int32(1)

	// If deployment exists and is already scaled down by TTL, preserve replicas=0
	if existingDeployment != nil && existingDeployment.Spec.Replicas != nil && *existingDeployment.Spec.Replicas == 0 {
		log.Printf("Agent %s is scaled down by TTL, preserving replicas=0 in spec", agentID)
		replicas = 0 // Preserve the TTL-enforced scale-down
	}

	// Determine agent image and token URL
	agentImage := os.Getenv("KUBERDE_AGENT_IMAGE")
	if agentImage == "" {
		agentImage = "soloking/kuberde-agent:latest"
	}

	tokenURL := os.Getenv("KEYCLOAK_TOKEN_URL")
	if tokenURL == "" {
		baseURL := os.Getenv("KEYCLOAK_URL")
		if baseURL == "" {
			// Use FQDN for cross-namespace DNS resolution (agents may run in team namespaces)
			baseURL = "http://keycloak.kuberde.svc.cluster.local:8080"
		}
		tokenURL = fmt.Sprintf("%s/realms/kuberde/protocol/openid-connect/token", baseURL)
	}

	expectedDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        agentID, // Use agentID as Deployment Name
			Namespace:   namespace,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cr, frpAgentGVR.GroupVersion().WithKind("RDEAgent")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						// 1. FRP Agent Sidecar
						{
							Name:            "kuberde-agent",
							Image:           agentImage,
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								{Name: "SERVER_URL", Value: serverUrl},
								{Name: "AGENT_ID", Value: agentID},
								{Name: "LOCAL_TARGET", Value: localTarget},
								{
									Name: "AUTH_CLIENT_ID",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key:                  "CLIENT_ID",
											LocalObjectReference: corev1.LocalObjectReference{Name: authSecret},
										},
									},
								},
								{
									Name: "AUTH_CLIENT_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key:                  "CLIENT_SECRET",
											LocalObjectReference: corev1.LocalObjectReference{Name: authSecret},
										},
									},
								},
								{Name: "AUTH_TOKEN_URL", Value: tokenURL},
							},
							// Default resources for kuberde-agent sidecar (required when ResourceQuota is set)
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						// 2. User Workload
						{
							Name:            "workload",
							Image:           workloadConfig.image,
							ImagePullPolicy: workloadConfig.imagePullPolicy,
							Command:         workloadConfig.command,
							Args:            workloadConfig.args,
							Ports:           workloadConfig.ports,
							Env:             workloadEnv,
							VolumeMounts:    volumeMounts,
							Resources:       workloadConfig.resources,
							SecurityContext: workloadConfig.securityContext,
						},
					},
					NodeSelector: nodeSelector,
					Tolerations:  tolerations,
					Volumes:      volumes,
				},
			},
		},
	}

	// NOTE: lastActivity reset is now handled by server session monitor when it detects active session
	// This is part of the new TTL architecture where server state drives activity tracking

	if err == nil {
		// Deployment exists, check if spec changed
		log.Printf("Deployment exists, calling deploymentSpecChanged...")
		changed := c.deploymentSpecChanged(existingDeployment, expectedDeployment)
		log.Printf("deploymentSpecChanged returned: %v", changed)

		if changed {
			log.Printf("Deployment spec changed for %s, updating...", name)
			expectedDeployment.ResourceVersion = existingDeployment.ResourceVersion
			_, err = deploymentsClient.Update(context.TODO(), expectedDeployment, metav1.UpdateOptions{})
			return err
		}
		// Spec unchanged, no action needed
		log.Printf("Deployment spec unchanged, no update needed")
		return nil
	}

	if !errors.IsNotFound(err) {
		return err
	}

	// Create Deployment
	log.Printf("Creating Deployment %s for agent %s", agentID, name)
	_, err = deploymentsClient.Create(context.TODO(), expectedDeployment, metav1.CreateOptions{})
	return err
}

// function removed as it is unused

func (c *Controller) checkTTL() {
	agentList, err := c.dynClient.Resource(frpAgentGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing RDEAgents: %v", err)
		return
	}

	for _, agent := range agentList.Items {
		spec, found, _ := unstructured.NestedMap(agent.Object, "spec")
		if !found {
			continue
		}

		ttlStr, ok := spec["ttl"].(string)
		if !ok {
			continue
		}

		// Parse TTL and scale down if exceeded
		statusObj, found := agent.Object["status"]
		if !found {
			continue
		}

		// Safely handle status object that might be nil or not a map
		if statusObj == nil {
			continue
		}

		statusMap, ok := statusObj.(map[string]interface{})
		if !ok {
			continue
		}

		lastActivityStr, ok := statusMap["lastActivity"].(string)
		if !ok {
			continue
		}

		lastActivity, err := time.Parse(time.RFC3339, lastActivityStr)
		if err != nil {
			continue
		}

		ttl, err := time.ParseDuration(ttlStr)
		if err != nil {
			continue
		}

		if time.Since(lastActivity) > ttl {
			name := agent.GetName()
			namespace := agent.GetNamespace()

			// Use CR name directly as agentID
			agentID := name

			log.Printf("TTL exceeded for agent %s, scaling down...", agentID)

			deploymentsClient := c.k8sClient.AppsV1().Deployments(namespace)
			deployment, err := deploymentsClient.Get(context.TODO(), agentID, metav1.GetOptions{})
			if err == nil && deployment.Spec.Replicas != nil && *deployment.Spec.Replicas != 0 {
				replicas := int32(0)
				deployment.Spec.Replicas = &replicas
				if _, err := deploymentsClient.Update(context.TODO(), deployment, metav1.UpdateOptions{}); err != nil {
					log.Printf("Failed to scale down deployment %s: %v", agentID, err)
				}
			}
		}
	}
}

// runServerSessionMonitor checks server API every 30 seconds for active agent sessions
// When session is detected, it refreshes lastActivity and scales up if needed
// This is independent of TTL enforcement and reconciliation
func (c *Controller) runServerSessionMonitor(ctx context.Context) {
	log.Println("Server session monitor started (30s interval)")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Server session monitor stopped")
			return
		case <-ticker.C:
			c.monitorServerSessions()
		}
	}
}

// monitorServerSessions queries server API for each agent to check session status
func (c *Controller) monitorServerSessions() {
	agentList, err := c.dynClient.Resource(frpAgentGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing RDEAgents for session monitoring: %v", err)
		return
	}

	for _, agent := range agentList.Items {
		c.checkAgentSession(&agent)
	}
}

// checkAgentSession queries server for a specific agent's session status
// If session exists: refresh lastActivity and scale up if needed
// If no session: preserve lastActivity for TTL calculation
func (c *Controller) checkAgentSession(agent *unstructured.Unstructured) {
	namespace := agent.GetNamespace()
	name := agent.GetName()

	// Extract agentID (same logic as reconciliation)
	// Use CR name directly as agentID
	agentID := name

	// Query server API for session status
	serverURL := os.Getenv("KUBERDE_SERVER_API")
	if serverURL == "" {
		serverURL = "http://kuberde-server.kuberde.svc:80"
	}

	resp, err := http.Get(serverURL + "/mgmt/agents/" + agentID)
	if err != nil {
		log.Printf("ServerMonitor: Error querying server for agent %s: %v", agentID, err)
		return // Don't modify lastActivity on error (conservative)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ServerMonitor: Server returned %d for agent %s", resp.StatusCode, agentID)
		return
	}

	var stats struct {
		Online           bool   `json:"online"`
		LastActivity     string `json:"last_activity"`
		HasActiveSession bool   `json:"hasActiveSession"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		log.Printf("ServerMonitor: Error decoding server response for agent %s: %v", agentID, err)
		return
	}

	// FIX: Use the actual LastActivity from Server instead of checking HasActiveSession
	// HasActiveSession only tells us if Agent is connected, not if users are accessing it
	// LastActivity from Server tells us when the last user activity occurred
	if !stats.Online {
		log.Printf("ServerMonitor: Agent %s is offline", agentID)
		return
	}

	log.Printf("ServerMonitor: Agent %s is online, server LastActivity: %s", agentID, stats.LastActivity)

	// Get fresh CR to avoid conflicts with concurrent updates
	freshAgent, err := c.dynClient.Resource(frpAgentGVR).Namespace(namespace).
		Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("ServerMonitor: Error getting fresh agent %s: %v", agentID, err)
		return
	}

	// 1. Sync LastActivity from Server (this is the actual user activity time)
	if freshAgent.Object["status"] == nil {
		freshAgent.Object["status"] = make(map[string]interface{})
	}
	statusMap := freshAgent.Object["status"].(map[string]interface{})

	// Use the Server's LastActivity time instead of current time
	// This ensures TTL is based on actual user activity, not just Agent being online
	if stats.LastActivity != "" {
		statusMap["lastActivity"] = stats.LastActivity
		log.Printf("ServerMonitor: Agent %s - synced lastActivity from server: %s", agentID, stats.LastActivity)
	} else {
		// Fallback to current time if Server doesn't provide it
		statusMap["lastActivity"] = time.Now().Format(time.RFC3339)
	}

	// 2. Scale-up is now handled by the Server on demand (when user connects)
	// We do NOT scale up here just because we see an active session, as that
	// creates a race condition during scale-down (agent is still online but terminating).
	// The operator's job in this loop is only to sync LastActivity status.
	/*
		deploymentsClient := c.k8sClient.AppsV1().Deployments(namespace)
		deployment, err := deploymentsClient.Get(context.TODO(), agentID, metav1.GetOptions{})
		if err != nil {
			log.Printf("ServerMonitor: Error getting deployment for %s: %v", agentID, err)
			// Still update lastActivity even if deployment fetch fails
		} else if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
			log.Printf("ServerMonitor: Agent %s scaled to 0, scaling up due to active session", agentID)
			replicas := int32(1)
			deployment.Spec.Replicas = &replicas
			_, err = deploymentsClient.Update(context.TODO(), deployment, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("ServerMonitor: Error scaling up agent %s: %v", agentID, err)
				return // Don't update status if scale-up failed
			}
		}
	*/

	// 3. Update CR status with refreshed lastActivity
	_, err = c.dynClient.Resource(frpAgentGVR).Namespace(namespace).
		UpdateStatus(context.TODO(), freshAgent, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("ServerMonitor: Warning - failed to update lastActivity for %s: %v", agentID, err)
		// Continue anyway - will retry next cycle
	}
}

func (c *Controller) runTTLChecker(ctx context.Context) {
	log.Println("TTL enforcement checker started (60s interval)")
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("TTL enforcement checker stopped")
			return
		case <-ticker.C:
			c.checkTTL()
		}
	}
}

func (c *Controller) deploymentSpecChanged(existing, expected *appsv1.Deployment) bool {
	// Compare only the Spec and Labels that matter for reconciliation
	// Ignore fields that Kubernetes manages (e.g., status, metadata.uid, etc.)

	if existing == nil || expected == nil {
		return existing != expected
	}

	// Compare replicas
	existingReplicas := int32(0)
	if existing.Spec.Replicas != nil {
		existingReplicas = *existing.Spec.Replicas
	}
	expectedReplicas := int32(0)
	if expected.Spec.Replicas != nil {
		expectedReplicas = *expected.Spec.Replicas
	}

	if existingReplicas != expectedReplicas {
		log.Printf("Replicas changed: %d -> %d", existingReplicas, expectedReplicas)
		return true
	}

	// Compare selector labels
	if !reflect.DeepEqual(existing.Spec.Selector.MatchLabels, expected.Spec.Selector.MatchLabels) {
		log.Printf("Selector labels changed")
		return true
	}

	// Compare pod template spec
	changed := c.podSpecChanged(&existing.Spec.Template.Spec, &expected.Spec.Template.Spec)
	if changed {
		return true
	}

	// Compare pod template labels
	if !reflect.DeepEqual(existing.Spec.Template.Labels, expected.Spec.Template.Labels) {
		log.Printf("Pod template labels changed")
		return true
	}

	return false
}

func (c *Controller) podSpecChanged(existing, expected *corev1.PodSpec) bool {
	// Compare containers
	if len(existing.Containers) != len(expected.Containers) {
		log.Printf("Container count changed: %d -> %d", len(existing.Containers), len(expected.Containers))
		return true
	}

	for i := range existing.Containers {
		if c.containerChanged(&existing.Containers[i], &expected.Containers[i]) {
			return true
		}
	}

	// Compare NodeSelector (normalize nil vs empty map)
	existingNS := existing.NodeSelector
	if existingNS == nil {
		existingNS = make(map[string]string)
	}
	expectedNS := expected.NodeSelector
	if expectedNS == nil {
		expectedNS = make(map[string]string)
	}
	if !reflect.DeepEqual(existingNS, expectedNS) {
		log.Printf("NodeSelector changed")
		return true
	}

	// Compare Tolerations (normalize nil vs empty slice)
	existingTol := existing.Tolerations
	if existingTol == nil {
		existingTol = []corev1.Toleration{}
	}
	expectedTol := expected.Tolerations
	if expectedTol == nil {
		expectedTol = []corev1.Toleration{}
	}
	if !reflect.DeepEqual(existingTol, expectedTol) {
		log.Printf("Tolerations changed")
		return true
	}

	// Compare Volumes (normalize nil vs empty slice)
	existingVol := existing.Volumes
	if existingVol == nil {
		existingVol = []corev1.Volume{}
	}
	expectedVol := expected.Volumes
	if expectedVol == nil {
		expectedVol = []corev1.Volume{}
	}
	if !reflect.DeepEqual(existingVol, expectedVol) {
		log.Printf("Volumes changed")
		return true
	}

	log.Printf("podSpecChanged: all containers unchanged")
	return false
}

func (c *Controller) containerChanged(existing, expected *corev1.Container) bool {
	if existing == nil || expected == nil {
		return existing != expected
	}

	// Compare critical fields
	if existing.Name != expected.Name {
		log.Printf("Container name changed: %s -> %s", existing.Name, expected.Name)
		return true
	}

	if existing.Image != expected.Image {
		log.Printf("Container image changed for %s: %s -> %s", existing.Name, existing.Image, expected.Image)
		return true
	}

	if existing.ImagePullPolicy != expected.ImagePullPolicy {
		log.Printf("ImagePullPolicy changed for %s", existing.Name)
		return true
	}

	// Compare environment variables
	if !c.envVarsEqual(existing.Env, expected.Env) {
		log.Printf("Environment variables changed for container %s", existing.Name)
		return true
	}

	// Compare command and args
	if !reflect.DeepEqual(existing.Command, expected.Command) {
		log.Printf("Command changed for container %s", existing.Name)
		return true
	}

	if !reflect.DeepEqual(existing.Args, expected.Args) {
		log.Printf("Args changed for container %s", existing.Name)
		return true
	}

	// Compare ports
	if !reflect.DeepEqual(existing.Ports, expected.Ports) {
		log.Printf("Ports changed for container %s", existing.Name)
		return true
	}

	// Compare volume mounts
	if !reflect.DeepEqual(existing.VolumeMounts, expected.VolumeMounts) {
		log.Printf("VolumeMounts changed for container %s", existing.Name)
		return true
	}

	return false
}

func (c *Controller) envVarsEqual(existing, expected []corev1.EnvVar) bool {
	if len(existing) != len(expected) {
		return false
	}

	// Create maps for easier comparison (handles different ordering)
	existingMap := make(map[string]corev1.EnvVar)
	for _, ev := range existing {
		existingMap[ev.Name] = ev
	}

	for _, expectedEv := range expected {
		existingEv, found := existingMap[expectedEv.Name]
		if !found {
			log.Printf("Missing environment variable: %s", expectedEv.Name)
			return false
		}

		// Deep comparison of EnvVar (including ValueFrom references)
		if !reflect.DeepEqual(existingEv, expectedEv) {
			log.Printf("Environment variable %s changed", expectedEv.Name)
			return false
		}
	}

	return true
}

// parseResources converts a map[string]interface{} to corev1.ResourceRequirements
func (c *Controller) parseResources(resourcesMap map[string]interface{}) corev1.ResourceRequirements {
	reqs := corev1.ResourceRequirements{
		Requests: make(corev1.ResourceList),
		Limits:   make(corev1.ResourceList),
	}

	if requests, ok := resourcesMap["requests"].(map[string]interface{}); ok {
		for k, v := range requests {
			if val, ok := v.(string); ok {
				// Use ParseQuantity to handle suffixes like 'Mi', 'G'
				if q, err := resource.ParseQuantity(val); err == nil {
					reqs.Requests[corev1.ResourceName(k)] = q
				} else {
					log.Printf("Warning: failed to parse request quantity for %s: %s", k, val)
				}
			} else if val, ok := v.(int64); ok {
				// Handle raw integers (e.g. 1 GPU)
				q := *resource.NewQuantity(val, resource.DecimalSI)
				reqs.Requests[corev1.ResourceName(k)] = q
			}
		}
	}

	if limits, ok := resourcesMap["limits"].(map[string]interface{}); ok {
		for k, v := range limits {
			if val, ok := v.(string); ok {
				if q, err := resource.ParseQuantity(val); err == nil {
					reqs.Limits[corev1.ResourceName(k)] = q
				} else {
					log.Printf("Warning: failed to parse limit quantity for %s: %s", k, val)
				}
			} else if val, ok := v.(int64); ok {
				q := *resource.NewQuantity(val, resource.DecimalSI)
				reqs.Limits[corev1.ResourceName(k)] = q
			}
		}
	}
	return reqs
}

// parseSecurityContext converts securityContext map to Kubernetes SecurityContext
func (c *Controller) parseSecurityContext(scMap map[string]interface{}) *corev1.SecurityContext {
	sc := &corev1.SecurityContext{}

	// Parse runAsUser
	if runAsUser, ok := scMap["runAsUser"].(float64); ok {
		uid := int64(runAsUser)
		sc.RunAsUser = &uid
	} else if runAsUser, ok := scMap["runAsUser"].(int64); ok {
		sc.RunAsUser = &runAsUser
	}

	// Parse runAsGroup
	if runAsGroup, ok := scMap["runAsGroup"].(float64); ok {
		gid := int64(runAsGroup)
		sc.RunAsGroup = &gid
	} else if runAsGroup, ok := scMap["runAsGroup"].(int64); ok {
		sc.RunAsGroup = &runAsGroup
	}

	// Note: fsGroup is a pod-level setting (PodSecurityContext), not container-level
	// It should be handled separately if needed in the future

	// Parse runAsNonRoot
	if runAsNonRoot, ok := scMap["runAsNonRoot"].(bool); ok {
		sc.RunAsNonRoot = &runAsNonRoot
	}

	// Parse privileged
	if privileged, ok := scMap["privileged"].(bool); ok {
		sc.Privileged = &privileged
	}

	// Parse readOnlyRootFilesystem
	if readOnlyRootFilesystem, ok := scMap["readOnlyRootFilesystem"].(bool); ok {
		sc.ReadOnlyRootFilesystem = &readOnlyRootFilesystem
	}

	// Parse allowPrivilegeEscalation
	if allowPrivilegeEscalation, ok := scMap["allowPrivilegeEscalation"].(bool); ok {
		sc.AllowPrivilegeEscalation = &allowPrivilegeEscalation
	}

	return sc
}

// reconcileSecrets creates or updates Kubernetes Secrets for SSH public keys
func (c *Controller) reconcileSecrets(cr *unstructured.Unstructured, agentID string) error {
	namespace := cr.GetNamespace()

	// Parse SSH public keys from spec
	var publicKeys string
	if keysSlice, found, _ := unstructured.NestedSlice(cr.Object, "spec", "sshPublicKeys"); found {
		var keys []string
		for _, k := range keysSlice {
			if keyStr, ok := k.(string); ok && keyStr != "" {
				keys = append(keys, keyStr)
			}
		}
		if len(keys) > 0 {
			publicKeys = strings.Join(keys, "\n")
		}
	}

	// Only create secret if there are SSH public keys
	if publicKeys == "" {
		log.Printf("No SSH public keys for agent %s, skipping secret creation", agentID)
		return nil
	}

	// Create Secret name
	secretName := fmt.Sprintf("%s-credentials", agentID)

	// Build Secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cr, frpAgentGVR.GroupVersion().WithKind("RDEAgent")),
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"PUBLIC_KEY": publicKeys,
		},
	}

	// Try to create or update Secret
	secretsClient := c.k8sClient.CoreV1().Secrets(namespace)
	existing, err := secretsClient.Get(context.TODO(), secretName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		// Secret doesn't exist, create new one
		log.Printf("Creating Secret %s for agent %s with SSH public keys", secretName, agentID)
		_, err = secretsClient.Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Error creating Secret %s: %v", secretName, err)
			return err
		}
		return nil
	}

	if err != nil {
		log.Printf("Error getting Secret %s: %v", secretName, err)
		return err
	}

	// Secret exists, check if PUBLIC_KEY changed
	needsUpdate := false
	if existing.StringData == nil {
		existing.StringData = make(map[string]string)
	}

	if existing.StringData["PUBLIC_KEY"] != publicKeys {
		log.Printf("SSH public keys changed for agent %s, updating Secret", agentID)
		existing.StringData["PUBLIC_KEY"] = publicKeys
		needsUpdate = true
	}

	if needsUpdate {
		_, err = secretsClient.Update(context.TODO(), existing, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("Error updating Secret %s: %v", secretName, err)
			return err
		}
	}

	return nil
}

// reconcileStorage creates or updates Kubernetes PVCs for storage volumes
func (c *Controller) reconcileStorage(cr *unstructured.Unstructured, agentID string) error {
	namespace := cr.GetNamespace()

	storageList, found, _ := unstructured.NestedSlice(cr.Object, "spec", "storage")
	if !found {
		return nil
	}

	pvcsClient := c.k8sClient.CoreV1().PersistentVolumeClaims(namespace)

	for _, s := range storageList {
		storage := s.(map[string]interface{})
		name, _ := storage["name"].(string)
		size, _ := storage["size"].(string)
		storageClass, _ := storage["storageClass"].(string)

		pvcName := fmt.Sprintf("kuberde-agent-%s-%s", agentID, name)

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(cr, frpAgentGVR.GroupVersion().WithKind("RDEAgent")),
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				StorageClassName: &storageClass,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(size),
					},
				},
			},
		}

		existing, err := pvcsClient.Get(context.TODO(), pvcName, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			log.Printf("Creating PVC %s for agent %s", pvcName, agentID)
			_, err = pvcsClient.Create(context.TODO(), pvc, metav1.CreateOptions{})
			if err != nil {
				log.Printf("Error creating PVC %s: %v", pvcName, err)
				return err
			}
		case err == nil:
			// PVC exists - only update mutable fields (storage size)
			// Note: Most PVC fields are immutable after creation, so we must be careful
			log.Printf("PVC %s already exists for agent %s", pvcName, agentID)

			// Check if size needs to be updated
			oldSize := existing.Spec.Resources.Requests[corev1.ResourceStorage]
			newSize := resource.MustParse(size)

			if oldSize.Cmp(newSize) != 0 {
				log.Printf("Updating PVC %s size from %s to %s", pvcName, oldSize.String(), newSize.String())
				existing.Spec.Resources.Requests[corev1.ResourceStorage] = newSize
				_, err = pvcsClient.Update(context.TODO(), existing, metav1.UpdateOptions{})
				if err != nil {
					log.Printf("Warning: Failed to update PVC %s size: %v (immutable field issue)", pvcName, err)
					// Don't return error - PVC still exists and is usable
				}
			} else {
				log.Printf("PVC %s size unchanged, no update needed", pvcName)
			}
		default:
			log.Printf("Error getting PVC %s: %v", pvcName, err)
			return err
		}
	}

	return nil
}

// NOTE: updateStatus() has been refactored and is no longer used directly.
// The status update logic has been moved to separate functions for better separation of concerns:
// - computeStatus() - computes the desired status (in status_compute.go)
// - updateStatusWithRetry() - handles update with exponential backoff (in status_retry.go)
// - updateAllStatusesParallel() - orchestrates parallel updates (in this file)
// These functions are now called by the new parallel status update system.

// checkAgentOnline queries the FRP Server API to check if agent is online
func (c *Controller) checkAgentOnline(agentID string) bool {
	serverURL := os.Getenv("KUBERDE_SERVER_API")
	if serverURL == "" {
		serverURL = "http://kuberde-server.kuberde.svc:80"
	}

	url := fmt.Sprintf("%s/mgmt/agents/%s", serverURL, agentID)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Error checking agent %s online status: %v", agentID, err)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Parse response to check online status
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response for agent %s: %v", agentID, err)
		return false
	}

	var agentInfo map[string]interface{}
	err = json.Unmarshal(body, &agentInfo)
	if err != nil {
		log.Printf("Error parsing agent info for %s: %v", agentID, err)
		return false
	}

	// Check online status from response
	if online, ok := agentInfo["online"].(bool); ok {
		return online
	}

	return false
}

// generateCorrelationID creates a unique identifier for this status update cycle
func generateCorrelationID() string {
	return fmt.Sprintf("status-update-%d", time.Now().UnixNano())
}

// runStatusUpdater periodically updates CR status with parallel processing
func (c *Controller) runStatusUpdater(ctx context.Context) {
	log.Printf("Status updater goroutine started with parallel worker pool")
	_, _ = os.Stderr.WriteString("Status updater: goroutine initialized\n")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Status updater goroutine stopped")
			return
		case <-ticker.C:
			correlationID := generateCorrelationID()
			log.Printf("[%s] Status update cycle started", correlationID)
			c.updateAllStatusesParallel(correlationID)
		}
	}
}

// updateAllStatusesParallel updates status for all RDEAgents using parallel worker pool
func (c *Controller) updateAllStatusesParallel(correlationID string) {
	startTime := time.Now()

	log.Printf("[%s] updateAllStatusesParallel: listing RDEAgents", correlationID)

	agentList, err := c.dynClient.Resource(frpAgentGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("[%s] Error listing RDEAgents for status update: %v", correlationID, err)
		_, _ = fmt.Fprintf(os.Stderr, "[%s] Error listing agents: %v\n", correlationID, err)
		return
	}

	log.Printf("[%s] updateAllStatusesParallel: found %d agents", correlationID, len(agentList.Items))

	if len(agentList.Items) == 0 {
		log.Printf("[%s] No agents to update", correlationID)
		return
	}

	// Create queue with configurable worker count
	// Rule of thumb: 4-8 workers for I/O bound operations
	workers := 4
	if count := len(agentList.Items); count > 16 {
		workers = 8
	}

	queue := NewStatusUpdateQueue(workers)
	queue.Start(c)

	log.Printf("[%s] Created status update queue with %d workers", correlationID, workers)

	// Enqueue all agents
	for i := range agentList.Items {
		task := &StatusUpdateTask{
			CR:            &agentList.Items[i],
			CorrelationID: correlationID,
			CreatedAt:     time.Now(),
			AttemptCount:  1,
		}
		queue.Enqueue(task)
	}

	log.Printf("[%s] Queued %d agents for status update", correlationID, len(agentList.Items))

	// Collect results
	results := queue.CollectResults(len(agentList.Items))

	// Summarize results
	queue.SummarizeResults(correlationID, results)

	cycleDuration := time.Since(startTime)
	log.Printf("[%s] Status update cycle completed in %v", correlationID, cycleDuration)

	queue.Stop()
}
