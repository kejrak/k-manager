#!/bin/sh

# Check if a custom config file is mounted
if [ -f "/app/config/config.yaml" ]; then
    export POD_ERROR_MONITOR_CONFIG="/app/config/config.yaml"
fi

# Check if we're running in a Kubernetes cluster
if [ -f /var/run/secrets/kubernetes.io/serviceaccount/token ]; then
    echo "Running in Kubernetes cluster"
    # Update the config to use in-cluster mode
    sed -i 's/use_in_cluster: false/use_in_cluster: true/' $POD_ERROR_MONITOR_CONFIG
    exec ./main
else
    echo "Running outside cluster"
    if [ -f /root/.kube/config ]; then
        # Update the config to use kubeconfig mode
        sed -i 's/use_in_cluster: true/use_in_cluster: false/' $POD_ERROR_MONITOR_CONFIG
        exec ./main
    else
        echo "No kubeconfig found at /root/.kube/config"
        exit 1
    fi
fi 