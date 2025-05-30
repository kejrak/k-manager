package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type podError struct {
	namespace     string
	podName       string
	errorType     string
	errorMessage  string
	containerName string
	restartCount  int32
}

type namespaceStats struct {
	name             string
	totalErrors      int
	uniquePods       map[string]bool
	errorTypes       map[string]int
	totalRestarts    int32
	crashLoopCount   int
	imagePullCount   int
	highRestartCount int
	score            float64
}

func main() {
	app := &cli.App{
		Name:    "pod-error-monitor",
		Usage:   "Monitor Kubernetes pod errors",
		Version: "1.0.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "kubeconfig",
				Aliases: []string{"k"},
				Value:   filepath.Join(homedir.HomeDir(), ".kube", "config"),
				Usage:   "Path to kubeconfig file",
			},
			&cli.StringFlag{
				Name:    "namespace",
				Aliases: []string{"n"},
				Usage:   "Filter by namespace (default: all namespaces)",
			},
			&cli.StringFlag{
				Name:    "context",
				Aliases: []string{"c"},
				Usage:   "Use specific Kubernetes context",
			},
			&cli.BoolFlag{
				Name:    "watch",
				Aliases: []string{"w"},
				Usage:   "Watch for changes (updates every 5 seconds)",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"V"},
				Usage:   "Show additional information about errors",
			},
		},
		Action:          runCLI,
		HideHelpCommand: true,
		CustomAppHelpTemplate: `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{.HelpName}} [options]

VERSION:
   {{.Version}}

DESCRIPTION:
   A command-line tool to monitor Kubernetes pod errors across namespaces.
   It detects various error conditions including:
   - CrashLoopBackOff
   - ImagePullBackOff
   - High restart counts (> 5)
   - Failed pods
   - Container creation errors

   The tool provides a clear overview of problematic pods and their error states,
   helping you quickly identify and troubleshoot issues in your cluster.

ERROR TYPES:
   CrashLoopBackOff    Container repeatedly crashes after starting
   ImagePullBackOff    Unable to pull the container image
   ErrImagePull        Error occurred while pulling the image
   PodFailed           Pod is in Failed phase
   HighRestartCount    Container has restarted more than 5 times
   CreateContainerError Unable to create the container
   InvalidImageName    Container image name is invalid
   ImageInspectError   Error inspecting the container image
   ErrImageNeverPull   Image pull policy prevents pulling

EXAMPLES:
   # Monitor all namespaces
   {{.HelpName}}

   # Monitor specific namespace
   {{.HelpName}} -n kube-system

   # Use specific context
   {{.HelpName}} -c minikube

   # Use custom kubeconfig
   {{.HelpName}} -k /path/to/kubeconfig

   # Watch for changes (updates every 5 seconds)
   {{.HelpName}} -w

   # Show verbose error information
   {{.HelpName}} --verbose, -V

   # Combine multiple options
   {{.HelpName}} -n kube-system -c minikube -w -V

OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}
`,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func runCLI(c *cli.Context) error {
	// Load kubeconfig
	config, err := clientcmd.LoadFromFile(c.String("kubeconfig"))
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	// Switch context if specified
	if ctx := c.String("context"); ctx != "" {
		if err := switchContext(ctx, c.String("kubeconfig")); err != nil {
			return fmt.Errorf("failed to switch context: %v", err)
		}
	}

	// Build kubernetes client
	restConfig, err := clientcmd.BuildConfigFromFlags("", c.String("kubeconfig"))
	if err != nil {
		return fmt.Errorf("failed to build config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	// Print current context
	fmt.Printf("Context: %s\n", config.CurrentContext)
	if c.String("namespace") != "" {
		fmt.Printf("Namespace: %s\n", c.String("namespace"))
	}
	fmt.Println()

	// Get and display errors
	return displayErrors(clientset, c.String("namespace"))
}

func calculateNamespaceStats(errors []podError) []namespaceStats {
	// Group by namespace
	statsMap := make(map[string]*namespaceStats)

	// Initialize stats for each namespace
	for _, err := range errors {
		if _, exists := statsMap[err.namespace]; !exists {
			statsMap[err.namespace] = &namespaceStats{
				name:       err.namespace,
				uniquePods: make(map[string]bool),
				errorTypes: make(map[string]int),
			}
		}

		stats := statsMap[err.namespace]
		stats.totalErrors++
		stats.uniquePods[err.podName] = true
		stats.errorTypes[err.errorType]++
		stats.totalRestarts += err.restartCount

		// Count specific error types
		switch err.errorType {
		case "CrashLoopBackOff":
			stats.crashLoopCount++
		case "ImagePullBackOff", "ErrImagePull":
			stats.imagePullCount++
		case "HighRestartCount":
			stats.highRestartCount++
		}
	}

	// Calculate scores and convert to slice
	var results []namespaceStats
	for _, stats := range statsMap {
		// Scoring formula:
		// - CrashLoopBackOff: 3 points
		// - ImagePull issues: 2 points
		// - High restart count: 2 points
		// - Other errors: 1 point
		// - Each restart: 0.1 points
		stats.score = float64(stats.crashLoopCount*3+
			stats.imagePullCount*2+
			stats.highRestartCount*2+
			(stats.totalErrors-stats.crashLoopCount-stats.imagePullCount-stats.highRestartCount)) +
			float64(stats.totalRestarts)*0.1

		results = append(results, *stats)
	}

	// Sort by score in descending order
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	return results
}

func displayErrors(clientset *kubernetes.Clientset, namespace string) error {
	// Get pods
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	var allErrors []podError
	// Collect all errors
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodFailed {
			allErrors = append(allErrors, podError{
				namespace:    pod.Namespace,
				podName:      pod.Name,
				errorType:    "PodFailed",
				errorMessage: "Pod is in Failed phase",
			})
			continue
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.RestartCount > 5 {
				allErrors = append(allErrors, podError{
					namespace:     pod.Namespace,
					podName:       pod.Name,
					errorType:     "HighRestartCount",
					errorMessage:  "Container has restarted multiple times",
					containerName: containerStatus.Name,
					restartCount:  containerStatus.RestartCount,
				})
			}

			if containerStatus.State.Waiting != nil {
				reason := containerStatus.State.Waiting.Reason
				errorTypes := map[string]bool{
					"ImagePullBackOff":     true,
					"CrashLoopBackOff":     true,
					"ErrImagePull":         true,
					"CreateContainerError": true,
					"InvalidImageName":     true,
					"ImageInspectError":    true,
					"ErrImageNeverPull":    true,
				}

				if errorTypes[reason] {
					allErrors = append(allErrors, podError{
						namespace:     pod.Namespace,
						podName:       pod.Name,
						errorType:     reason,
						errorMessage:  containerStatus.State.Waiting.Message,
						containerName: containerStatus.Name,
						restartCount:  containerStatus.RestartCount,
					})
				}
			}
		}
	}

	// Calculate namespace statistics
	stats := calculateNamespaceStats(allErrors)

	// Display namespace statistics
	fmt.Println("\nNamespace Statistics (sorted by severity):")
	fmt.Println("----------------------------------------")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAMESPACE\tSCORE\tTOTAL ERRORS\tUNIQUE PODS\tCRASHLOOP\tIMAGE PULL\tHIGH RESTARTS\tTOTAL RESTARTS\n")
	fmt.Fprintf(w, "---------\t-----\t------------\t-----------\t---------\t----------\t-------------\t--------------\n")

	for _, ns := range stats {
		fmt.Fprintf(w, "%s\t%.1f\t%d\t%d\t%d\t%d\t%d\t%d\n",
			ns.name,
			ns.score,
			ns.totalErrors,
			len(ns.uniquePods),
			ns.crashLoopCount,
			ns.imagePullCount,
			ns.highRestartCount,
			ns.totalRestarts,
		)
	}
	w.Flush()
	fmt.Println("\nScoring formula:")
	fmt.Println("- CrashLoopBackOff: 3 points")
	fmt.Println("- ImagePull issues: 2 points")
	fmt.Println("- High restart count: 2 points")
	fmt.Println("- Other errors: 1 point")
	fmt.Println("- Each restart: 0.1 points")

	// Display detailed errors by namespace
	fmt.Println("\nDetailed Errors by Namespace:")
	fmt.Println("----------------------------")

	// Group errors by namespace
	namespaceErrors := make(map[string][]podError)
	for _, err := range allErrors {
		namespaceErrors[err.namespace] = append(namespaceErrors[err.namespace], err)
	}

	// Sort namespaces by score
	for _, ns := range stats {
		errors := namespaceErrors[ns.name]
		if len(errors) > 0 {
			fmt.Printf("\nNamespace: %s (Score: %.1f, %d errors)\n", ns.name, ns.score, len(errors))
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "POD\tCONTAINER\tTYPE\tRESTARTS\tMESSAGE\n")
			fmt.Fprintf(w, "---\t---------\t----\t--------\t-------\n")

			for _, err := range errors {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					err.podName,
					err.containerName,
					err.errorType,
					err.restartCount,
					err.errorMessage,
				)
			}
			w.Flush()
			fmt.Println()
		}
	}

	return nil
}

func switchContext(newContext, kubeconfig string) error {
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	// Clean the context name
	newContext = strings.TrimSpace(newContext)

	// Validate if the context exists
	context, exists := config.Contexts[newContext]
	if !exists {
		return fmt.Errorf("context %q not found in kubeconfig", newContext)
	}

	// Validate if the context has a cluster defined
	if context.Cluster == "" {
		return fmt.Errorf("context %q has no cluster defined", newContext)
	}

	// Validate if the cluster exists and has a server
	cluster, exists := config.Clusters[context.Cluster]
	if !exists {
		return fmt.Errorf("cluster %q not found for context %q", context.Cluster, newContext)
	}
	if cluster.Server == "" {
		return fmt.Errorf("cluster %q has no server defined for context %q", context.Cluster, newContext)
	}

	config.CurrentContext = newContext
	return clientcmd.WriteToFile(*config, kubeconfig)
}
