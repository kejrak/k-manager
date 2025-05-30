package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

type Server struct {
	clientset *kubernetes.Clientset
}

func main() {
	// Get in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error creating in-cluster config: %v", err)
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}

	// Initialize server with clientset
	server := &Server{clientset: clientset}

	// Initialize router
	r := mux.NewRouter()

	// API routes
	r.HandleFunc("/api/namespaces", server.getNamespaceStats).Methods("GET")
	r.HandleFunc("/api/namespaces/{namespace}/pods", server.getNamespacePodErrors).Methods("GET")

	// Configure CORS
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
	})

	// Start server
	port := ":8080"
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(port, c.Handler(r)))
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
