#!/bin/bash

# Brev API Test Runner
# Simple script to test the Brev AWS provider API

set -e

echo "🚀 Brev AWS Provider API Test"
echo "============================="

# Check if we're in the right directory
if [ ! -f "test-brev-api.go" ]; then
    echo "❌ Error: Please run this script from the examples directory"
    exit 1
fi

# Check AWS credentials
if ! aws sts get-caller-identity > /dev/null 2>&1; then
    echo "❌ Error: AWS credentials not configured"
    echo "   Please configure AWS credentials first"
    exit 1
fi

echo "✅ AWS credentials verified"

# Show usage if no arguments
if [ $# -eq 0 ]; then
    echo ""
    echo "Usage examples:"
    echo ""
    echo "📋 List resources:"
    echo "   $0 list --resource instance    # List EC2 instances"
    echo "   $0 list --resource cluster     # List EKS clusters"
    echo ""
    echo "🚀 Create resources:"
    echo "   $0 create --resource instance  # Create EC2 instance"
    echo "   $0 create --resource cluster   # Create EKS cluster"
    echo "   $0 create --resource cluster --name my-test-cluster --version 1.31"
    echo ""
    echo "💥 Destroy resources:"
    echo "   $0 destroy --resource instance --instance-id i-1234567890abcdef0"
    echo "   $0 destroy --resource cluster --cluster-name my-test-cluster"
    echo ""
    echo "Available options:"
    echo "   --resource    Resource type: instance or cluster (default: instance)"
    echo "   --type        Instance type (default: t3.micro)"
    echo "   --region      AWS region (default: us-west-2)"
    echo "   --name        Resource name (default: brev-test-instance)"
    echo "   --version     Kubernetes version for cluster (default: 1.32)"
    echo "   --instance-id Instance ID for destroy action"
    echo "   --cluster-name  Cluster name for destroy action"
    exit 0
fi

# Parse action
ACTION="$1"
shift

case "$ACTION" in
    "create")
        echo "🚀 Creating resource using Brev API..."
        go run test-brev-api.go -action=create "$@"
        ;;
    "destroy")
        echo "💥 Destroying resource using Brev API..."
        go run test-brev-api.go -action=destroy "$@"
        ;;
    "list")
        echo "📋 Listing resources using Brev API..."
        go run test-brev-api.go -action=list "$@"
        ;;
    *)
        echo "❌ Error: Unknown action '$ACTION'"
        echo "   Valid actions: create, destroy, list"
        exit 1
        ;;
esac