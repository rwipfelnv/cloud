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
    echo "📋 List instances:"
    echo "   $0 list"
    echo ""
    echo "🚀 Create instance:"
    echo "   $0 create"
    echo "   $0 create --type t3.small --name my-test-instance"
    echo ""
    echo "💥 Destroy instance:"
    echo "   $0 destroy --instance-id i-1234567890abcdef0"
    echo ""
    echo "Available options:"
    echo "   --type        Instance type (default: t3.micro)"
    echo "   --region      AWS region (default: us-west-2)"
    echo "   --name        Instance name (default: brev-test-instance)"
    echo "   --instance-id Instance ID for destroy action"
    exit 0
fi

# Parse action
ACTION="$1"
shift

case "$ACTION" in
    "create")
        echo "🚀 Creating instance using Brev API..."
        go run test-brev-api.go -action=create "$@"
        ;;
    "destroy")
        echo "💥 Destroying instance using Brev API..."
        go run test-brev-api.go -action=destroy "$@"
        ;;
    "list")
        echo "📋 Listing instances using Brev API..."
        go run test-brev-api.go -action=list "$@"
        ;;
    *)
        echo "❌ Error: Unknown action '$ACTION'"
        echo "   Valid actions: create, destroy, list"
        exit 1
        ;;
esac