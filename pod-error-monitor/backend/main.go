package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"

	"pod-error-monitor/config"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type PodError struct {
	Namespace     string `json:"namespace"`
	PodName       string `json:"podName"`
	ErrorType     string `json:"errorType"`
	ErrorMessage  string `json:"errorMessage"`
	ContainerName string `json:"containerName"`
	RestartCount  int32  `json:"restartCount"`
}

type NamespaceStats struct {
	Name          string  `json:"name"`
	TotalErrors   int     `json:"totalErrors"`
	Score         float64 `json:"score"`
	UniquePods    int     `json:"uniquePods"`
	CrashLoop     int     `json:"crashLoop"`
	ImagePull     int     `json:"imagePull"`
	HighRestarts  int     `json:"highRestarts"`
	TotalRestarts int32   `json:"totalRestarts"`
}

type KubeConfig struct {
	CurrentContext string   `json:"currentContext"`
	Contexts       []string `json:"contexts"`
}

type Server struct {
	clientset *kubernetes.Clientset
	config    *clientcmd.ClientConfig
	appConfig *config.Config
}

func main() {
	// Load configuration
	configPath := flag.String("config", config.GetConfigPath(), "path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	var k8sConfig *rest.Config
	var clientConfig clientcmd.ClientConfig

	if cfg.Kubernetes.UseInCluster {
		// Get in-cluster config
		k8sConfig, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Error creating in-cluster config: %v", err)
		}
	} else {
		// Get kubeconfig
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.ExplicitPath = cfg.Kubernetes.KubeconfigPath

		overrides := &clientcmd.ConfigOverrides{}
		if cfg.Kubernetes.DefaultContext != "" {
			overrides.CurrentContext = cfg.Kubernetes.DefaultContext
		}

		clientConfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
		k8sConfig, err = clientConfig.ClientConfig()
		if err != nil {
			log.Fatalf("Error building kubeconfig: %v", err)
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}

	// Initialize server with clientset and config
	server := &Server{
		clientset: clientset,
		config:    &clientConfig,
		appConfig: cfg,
	}

	// Initialize router
	r := mux.NewRouter()

	// API routes
	r.HandleFunc("/api/namespaces", server.getNamespaceStats).Methods("GET")
	r.HandleFunc("/api/namespaces/{namespace}/pods", server.getNamespacePodErrors).Methods("GET")
	r.HandleFunc("/api/contexts", server.getContexts).Methods("GET")
	r.HandleFunc("/api/contexts/{context}", server.switchContext).Methods("POST")

	// Configure CORS
	c := cors.New(cors.Options{
		AllowedOrigins: cfg.Server.CORS.AllowedOrigins,
		AllowedMethods: cfg.Server.CORS.AllowedMethods,
	})

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, c.Handler(r)))
}

func (s *Server) getContexts(w http.ResponseWriter, r *http.Request) {
	if s.config == nil {
		http.Error(w, "Not running with kubeconfig", http.StatusBadRequest)
		return
	}

	rawConfig, err := (*s.config).RawConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	contexts := make([]string, 0, len(rawConfig.Contexts))
	for name := range rawConfig.Contexts {
		contexts = append(contexts, name)
	}

	response := KubeConfig{
		CurrentContext: rawConfig.CurrentContext,
		Contexts:       contexts,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) switchContext(w http.ResponseWriter, r *http.Request) {
	if s.config == nil {
		http.Error(w, "Not running with kubeconfig", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	newContext := vars["context"]

	rawConfig, err := (*s.config).RawConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Validate context exists
	if _, exists := rawConfig.Contexts[newContext]; !exists {
		http.Error(w, "Context not found", http.StatusBadRequest)
		return
	}

	// Update current context
	rawConfig.CurrentContext = newContext

	// Create new config
	configPath := s.appConfig.Kubernetes.KubeconfigPath
	if err := clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), rawConfig, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create new client config
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = configPath
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: newContext,
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	// Get new rest config
	config, err := clientConfig.ClientConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create new clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update server's clientset and config
	s.clientset = clientset
	s.config = &clientConfig

	// Get list of contexts for response
	contexts := make([]string, 0, len(rawConfig.Contexts))
	for name := range rawConfig.Contexts {
		contexts = append(contexts, name)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(KubeConfig{
		CurrentContext: newContext,
		Contexts:       contexts,
	})
}

func (s *Server) getNamespaceStats(w http.ResponseWriter, r *http.Request) {
	pods, err := s.clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := calculateNamespaceStats(pods)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) getNamespacePodErrors(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]

	pods, err := s.clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	errors := getPodErrors(pods)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(errors)
}

func calculateNamespaceStats(pods *v1.PodList) []NamespaceStats {
	statsMap := make(map[string]*NamespaceStats)
	uniquePodsMap := make(map[string]map[string]bool)

	// Initialize stats for each namespace
	for _, pod := range pods.Items {
		if _, exists := statsMap[pod.Namespace]; !exists {
			statsMap[pod.Namespace] = &NamespaceStats{
				Name: pod.Namespace,
			}
			uniquePodsMap[pod.Namespace] = make(map[string]bool)
		}

		stats := statsMap[pod.Namespace]

		if pod.Status.Phase == v1.PodFailed {
			stats.TotalErrors++
			uniquePodsMap[pod.Namespace][pod.Name] = true
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.RestartCount > 5 {
				stats.TotalErrors++
				stats.HighRestarts++
				stats.TotalRestarts += containerStatus.RestartCount
				uniquePodsMap[pod.Namespace][pod.Name] = true
			}

			if containerStatus.State.Waiting != nil {
				reason := containerStatus.State.Waiting.Reason
				switch reason {
				case "CrashLoopBackOff":
					stats.CrashLoop++
					stats.TotalErrors++
					uniquePodsMap[pod.Namespace][pod.Name] = true
				case "ImagePullBackOff", "ErrImagePull":
					stats.ImagePull++
					stats.TotalErrors++
					uniquePodsMap[pod.Namespace][pod.Name] = true
				}
			}
		}
	}

	// Calculate final stats and convert to slice
	var results []NamespaceStats
	for namespace, stats := range statsMap {
		stats.UniquePods = len(uniquePodsMap[namespace])

		// Calculate score
		stats.Score = float64(stats.CrashLoop*3+
			stats.ImagePull*2+
			stats.HighRestarts*2+
			(stats.TotalErrors-stats.CrashLoop-stats.ImagePull-stats.HighRestarts)) +
			float64(stats.TotalRestarts)*0.1

		if stats.TotalErrors > 0 {
			results = append(results, *stats)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

func getPodErrors(pods *v1.PodList) []PodError {
	var errors []PodError

	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodFailed {
			errors = append(errors, PodError{
				Namespace:    pod.Namespace,
				PodName:      pod.Name,
				ErrorType:    "PodFailed",
				ErrorMessage: "Pod is in Failed phase",
			})
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.RestartCount > 5 {
				errors = append(errors, PodError{
					Namespace:     pod.Namespace,
					PodName:       pod.Name,
					ErrorType:     "HighRestartCount",
					ErrorMessage:  "Container has restarted multiple times",
					ContainerName: containerStatus.Name,
					RestartCount:  containerStatus.RestartCount,
				})
			}

			if containerStatus.State.Waiting != nil {
				reason := containerStatus.State.Waiting.Reason
				if isErrorState(reason) {
					errors = append(errors, PodError{
						Namespace:     pod.Namespace,
						PodName:       pod.Name,
						ErrorType:     reason,
						ErrorMessage:  containerStatus.State.Waiting.Message,
						ContainerName: containerStatus.Name,
						RestartCount:  containerStatus.RestartCount,
					})
				}
			}
		}
	}

	return errors
}

func isErrorState(state string) bool {
	errorStates := map[string]bool{
		"ImagePullBackOff":     true,
		"CrashLoopBackOff":     true,
		"ErrImagePull":         true,
		"CreateContainerError": true,
		"InvalidImageName":     true,
		"ImageInspectError":    true,
		"ErrImageNeverPull":    true,
	}
	return errorStates[state]
}
