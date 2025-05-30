#!/bin/bash

# Exit on error
set -e

echo "Building backend image..."
cd backend
minikube image build -t pod-error-monitor-backend:latest .
cd ..

echo "Building frontend image..."
cd frontend
minikube image build -t pod-error-monitor-frontend:latest .
cd ..

minikube cache reload

echo "Creating namespace and applying Kubernetes manifests..."
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/

echo "Waiting for deployments to be ready..."
kubectl -n pod-error-monitor rollout status deployment/pod-error-monitor-backend
kubectl -n pod-error-monitor rollout status deployment/pod-error-monitor-frontend

echo "Deployment complete!"
echo "You can access the application through your ingress controller"
echo "To get the service URL, run: minikube service -n pod-error-monitor pod-error-monitor-frontend --url" 