# AWS Provider Testing Guide

This guide provides comprehensive instructions for testing the AWS provider implementation at various levels.

## Quick Start

Run basic tests to verify the AWS provider implementation.

## Testing Levels

### 1. Unit Tests
```bash
# Run basic unit tests
go test ./v1/providers/aws/

# Run with verbose output
go test -v ./v1/providers/aws/
```

### 2. Integration Tests
```bash
# Run focused integration tests
go test -v ./v1/providers/aws/ -run TestAWSIntegration

# Tests capabilities, regions, and instance types
```

## Advanced Testing Options

### 3. Instance Lifecycle Testing (Requires Setup)

**⚠️ Warning: This will create/destroy real AWS resources and incur costs!**

#### Prerequisites:
```bash
# Generate SSH key pair for testing
ssh-keygen -t rsa -b 2048 -f ~/.ssh/aws-test-key -N ""

# Set environment variables
export AWS_DEFAULT_REGION="us-west-2"
export AWS_VPC_ID="vpc-xxxxxxxxx"        # Your default VPC ID
export AWS_SUBNET_ID="subnet-xxxxxxxxx"    # Your subnet ID
export AWS_TEST_SSH_PUBLIC_KEY="$(cat ~/.ssh/aws-test-key.pub)"
export AWS_TEST_SSH_PRIVATE_KEY="$(cat ~/.ssh/aws-test-key)"
```

#### Run Instance Lifecycle Tests:
```bash
# Test complete instance creation/termination cycle
VALIDATION_TEST=true go test -v ./v1/providers/aws/ -run TestInstanceLifecycleValidation -timeout=10m

# What this tests:
# - Creates t3.micro instance with Ubuntu 22.04
# - Verifies instance reaches running state  
# - Tests SSH connectivity
# - Terminates instance
# - Cleans up resources
```

### 4. Full Validation Suite (Most Comprehensive)

```bash
# Run the complete validation suite
VALIDATION_TEST=true go test -v ./v1/providers/aws/ -run TestValidationFunctions -timeout=15m

# This tests:
# - All capabilities
# - Location discovery 
# - Instance type validation
# - Regional functionality
# - Error handling
# - Performance benchmarks
```

### 5. Specific Feature Testing

#### Test Instance Types (Fast)
```bash
# Test specific instance types
go run -c "
package main
import (
    \"context\"
    \"fmt\" 
    awsprovider \"github.com/brevdev/cloud/v1/providers/aws\"
    v1 \"github.com/brevdev/cloud/v1\"
)
func main() {
    ctx := context.Background()
    cred := awsprovider.NewAWSCredential(\"test\", \"\", \"\", awsprovider.WithDefaultRegion(\"us-west-2\"))
    client, _ := cred.MakeClient(ctx, \"us-west-2\")
    types, _ := client.GetInstanceTypes(ctx, v1.GetInstanceTypeArgs{
        InstanceTypes: []string{\"p3.2xlarge\", \"g4dn.xlarge\"}, // Test GPU instances
    })
    for _, it := range types {
        fmt.Printf(\"%s: %d vCPU, %.1f GiB RAM, %d GPUs\\n\", 
            it.Type, it.VCPU, float64(it.Memory)/(1024*1024*1024), len(it.SupportedGPUs))
    }
}
"
```

#### Test AMI Discovery
```bash
# Create a simple AMI test
cat > test_ami.go << 'EOF'
package main
import (
    "context"
    "fmt"
    awsprovider "github.com/brevdev/cloud/v1/providers/aws"
    v1 "github.com/brevdev/cloud/v1"
)
func main() {
    ctx := context.Background()
    cred := awsprovider.NewAWSCredential("test", "", "", awsprovider.WithDefaultRegion("us-west-2"))
    client, _ := cred.MakeClient(ctx, "us-west-2")
    
    images, _ := client.GetImages(ctx, v1.GetImageArgs{
        Owners: []string{"099720109477"}, // Canonical
        NameFilters: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"},
        Architectures: []string{"x86_64"},
    })
    
    fmt.Printf("Found %d Ubuntu 22.04 AMIs\n", len(images))
    if len(images) > 0 {
        latest := images[0]
        fmt.Printf("Latest: %s (%s)\n", latest.Name, latest.ID)
    }
}
EOF

go run test_ami.go
rm test_ami.go
```

### 6. Performance Testing

#### Benchmark Instance Type Discovery
```bash
# Test performance of instance type discovery
go test -v ./v1/providers/aws/ -run TestAWSIntegration/GetInstanceTypes -count=5

# Time different approaches
time go test -v ./v1/providers/aws/ -run TestAWSIntegration/GetLocations
```

## Cost-Safe Testing

### Free Tier Compatible Tests
```bash
# These tests should stay within AWS free tier limits:

# 1. Discovery operations (no cost)
go test -v ./v1/providers/aws/ -run TestAWSIntegration

# 2. Small instance lifecycle (minimal cost)
# Uses t3.micro (free tier eligible) for < 5 minutes
VALIDATION_TEST=true AWS_TEST_INSTANCE_TYPE="t3.micro" go test -v ./v1/providers/aws/ -run TestInstanceLifecycleValidation -timeout=10m
```

### Cost Estimation
- **Discovery tests**: $0 (read-only operations)
- **t3.micro lifecycle test**: ~$0.01 (< 5 minutes runtime)  
- **Full validation suite**: ~$0.05-0.10 (multiple small instances)

## Environment Setup Options

### Option 1: AWS Credentials File (Recommended)
Use AWS credentials file (`~/.aws/credentials`):
```bash
# No additional setup needed if credentials file exists
go test -v ./v1/providers/aws/ -run TestAWSIntegration
```

### Option 2: Environment Variables
```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"  
export AWS_DEFAULT_REGION="us-west-2"
```

### Option 3: Temporary Credentials
```bash
# Get temporary credentials (1 hour)
aws sts get-session-token --duration-seconds 3600

# Use the returned credentials
export AWS_ACCESS_KEY_ID="temp-key-from-sts"
export AWS_SECRET_ACCESS_KEY="temp-secret-from-sts"
export AWS_SESSION_TOKEN="temp-token-from-sts"
```

## Troubleshooting

### Common Issues

1. **"credentials not available"**
   ```bash
   # Check credentials
   aws sts get-caller-identity
   ```

2. **"access denied" errors**
   ```bash
   # Verify permissions
   aws ec2 describe-regions
   aws ec2 describe-instance-types --instance-types t3.micro
   ```

3. **VPC/Subnet issues**
   ```bash
   # Find your default VPC and subnet
   aws ec2 describe-vpcs --filters "Name=is-default,Values=true"
   aws ec2 describe-subnets --filters "Name=vpc-id,Values=YOUR-VPC-ID"
   ```

### Debug Mode
```bash
# Enable AWS SDK debug logging
AWS_SDK_LOAD_CONFIG=1 go test -v ./v1/providers/aws/ -run TestAWSIntegration
```

## Testing Priority

### High Priority (Safe & Fast)
1. **Integration tests** - Basic functionality verification
2. **AMI discovery test** - Verify Ubuntu image detection  
3. **GPU instance types** - Test P3/G4 instance metadata
4. **Multi-region testing** - Test different AWS regions

### Medium Priority (Small Cost)
5. **Instance lifecycle** - Create/terminate t3.micro (~$0.01)
6. **Security group creation** - Test firewall rules
7. **Tag management** - Test resource tagging

### Lower Priority (More Setup)
8. **Full validation suite** - Comprehensive testing
9. **Load testing** - Performance benchmarks
10. **Multi-VPC testing** - Advanced networking

## Quick Test Commands

```bash
# 1. Basic functionality verification
go test -v ./v1/providers/aws/ -run TestAWSIntegration

# 2. Test AMI discovery (fast, free)
go test -v ./v1/providers/aws/ -run TestAWSIntegration/GetInstanceTypes  

# 3. Instance creation test (costs ~$0.01)
# ssh-keygen -t rsa -b 2048 -f ~/.ssh/aws-test-key -N ""
# export AWS_TEST_SSH_PUBLIC_KEY="$(cat ~/.ssh/aws-test-key.pub)"
# VALIDATION_TEST=true go test -v ./v1/providers/aws/ -run TestInstanceLifecycleValidation -timeout=10m
```

## Success Metrics

A successful AWS provider implementation should demonstrate:

- ✅ **All capabilities** working (11 total)
- ✅ **All regions** discovered (17 total)  
- ✅ **Instance types** loading correctly
- ✅ **Real AWS API** integration working
- ✅ **Credential chain** handling functional
- ✅ **Error mapping** working
- ✅ **SSH connectivity** to instances
- ✅ **Instance lifecycle** management