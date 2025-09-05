# Contributing to AWS Provider

This document provides guidelines for contributing to the AWS provider implementation.

## Development Setup

### Prerequisites
- Go 1.22 or later
- AWS CLI configured (for testing)
- Valid AWS account with appropriate permissions

### Local Development
```bash
# Clone the repository
git clone https://github.com/brevdev/cloud.git
cd cloud

# Install dependencies
go mod tidy

# Run tests (unit tests only)
go test ./v1/providers/aws/

# Run validation tests (requires AWS credentials)
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_DEFAULT_REGION="us-east-1"
go test -v ./v1/providers/aws/ -run TestAWSValidationSuite
```

### Required AWS Permissions
For development and testing, you need an IAM user or role with these permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ec2:*",
                "iam:PassRole"
            ],
            "Resource": "*"
        }
    ]
}
```

**Note**: This is overly permissive for testing convenience. Production usage should follow least-privilege principles.

## Code Structure

### File Organization
```
v1/providers/aws/
├── client.go           # Credential and client implementation
├── capabilities.go     # AWS capability declarations
├── instance.go         # EC2 instance operations
├── instancetype.go     # EC2 instance type management
├── image.go            # AMI management
├── networking.go       # Security groups and firewall
├── storage.go          # EBS volume operations
├── tags.go             # Resource tagging
├── location.go         # Region and AZ management
├── quota.go            # Service limit management
├── validation_test.go  # Validation test suite
├── README.md           # Documentation
├── SECURITY.md         # Security requirements
└── CONTRIBUTE.md       # This file
```

### Code Style Guidelines

#### Error Handling
- Always wrap AWS SDK errors with context
- Map AWS-specific errors to v1 standard errors
- Use consistent error messages

```go
// Good
_, err := c.ec2Client.RunInstances(ctx, input)
if err != nil {
    return nil, fmt.Errorf("failed to launch instance: %w", err)
}

// Better - with AWS error mapping
_, err := c.ec2Client.RunInstances(ctx, input)
if err != nil {
    return nil, c.convertEC2Error(err)
}
```

#### AWS SDK Usage
- Always pass context to AWS SDK calls
- Use pointer helpers (`aws.String()`, `aws.Int32()`, etc.)
- Check for nil pointers when accessing AWS response fields

```go
// Good
result, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
    InstanceIds: []string{string(id)},
})

// Access fields safely
if result.Reservations[0].Instances[0].PublicIpAddress != nil {
    instance.PublicIP = aws.ToString(result.Reservations[0].Instances[0].PublicIpAddress)
}
```

#### Resource Management
- Always tag resources created by the provider
- Use consistent naming patterns
- Clean up resources on errors when possible

```go
// Tag all created resources
TagSpecifications: []types.TagSpecification{
    {
        ResourceType: types.ResourceTypeInstance,
        Tags: []types.Tag{
            {
                Key:   aws.String("CreatedBy"),
                Value: aws.String("brev-cloud-sdk"),
            },
        },
    },
}
```

## Testing Guidelines

### Unit Tests
- Test all public methods
- Mock AWS SDK calls using interfaces
- Test error conditions and edge cases
- Use table-driven tests for multiple scenarios

### Integration Tests
- Use the validation test suite framework
- Test against real AWS resources
- Clean up resources after tests
- Use unique resource names to avoid conflicts

### Test Environment Variables
```bash
# Required for validation tests
export AWS_ACCESS_KEY_ID="your-test-access-key"
export AWS_SECRET_ACCESS_KEY="your-test-secret-key"
export AWS_DEFAULT_REGION="us-east-1"

# Optional - specify existing resources
export AWS_VPC_ID="vpc-xxxxxxxxx"
export AWS_SUBNET_ID="subnet-xxxxxxxxx"

# For testing specific AMIs
export AWS_TEST_AMI_ID="ami-xxxxxxxxx"  # Ubuntu 22.04 LTS
```

## Implementation Guidelines

### Adding New Features
1. **Check AWS SDK**: Ensure the AWS SDK supports the feature
2. **Add Capability**: Add the capability to `capabilities.go`
3. **Implement Interface**: Implement the required interface methods
4. **Error Handling**: Add proper error mapping
5. **Test**: Add unit and integration tests
6. **Document**: Update README and code comments

### Security Considerations
1. **Default Security**: Always default to secure configurations
2. **Encryption**: Enable encryption by default where possible
3. **Access Control**: Follow least-privilege principles
4. **Credential Handling**: Never log or expose credentials
5. **Network Security**: Implement secure networking defaults

### Performance Considerations
1. **Pagination**: Use SDK pagination for list operations
2. **Rate Limits**: Respect AWS API rate limits
3. **Retries**: Implement exponential backoff for retryable operations
4. **Caching**: Cache stable data like instance types and regions
5. **Batch Operations**: Use batch operations where available

## Common Development Tasks

### Adding a New Instance Operation
```go
// 1. Add to capabilities.go if it's a new capability
func getAWSCapabilities() v1.Capabilities {
    return v1.Capabilities{
        // ... existing capabilities
        v1.CapabilityNewFeature, // Add new capability
    }
}

// 2. Implement the interface method in the appropriate file
func (c *AWSClient) NewOperation(ctx context.Context, args NewOperationArgs) error {
    // Implementation with proper error handling
    _, err := c.ec2Client.NewAWSOperation(ctx, &ec2.NewAWSOperationInput{
        // ... parameters
    })
    if err != nil {
        return c.convertEC2Error(err)
    }
    return nil
}

// 3. Add tests
func TestNewOperation(t *testing.T) {
    // Test implementation
}
```

### Adding Support for a New AWS Service
```go
// 1. Add the service client to AWSClient struct in client.go
type AWSClient struct {
    // ... existing clients
    newServiceClient *newservice.Client
}

// 2. Initialize the client in NewAWSClient()
return &AWSClient{
    // ... existing clients
    newServiceClient: newservice.NewFromConfig(cfg),
}

// 3. Create new file for the service (e.g., newservice.go)
// 4. Implement required interface methods
// 5. Add appropriate capabilities
// 6. Add tests
```

### Error Mapping
Add new AWS error mappings to `convertEC2Error()` or create service-specific error converters:

```go
func (c *AWSClient) convertEC2Error(err error) error {
    if err == nil {
        return nil
    }

    errStr := err.Error()
    switch {
    case strings.Contains(errStr, "InvalidInstanceID.NotFound"):
        return v1.ErrInstanceNotFound
    case strings.Contains(errStr, "NewSpecificError"):
        return v1.ErrNewMappedError
    default:
        return err
    }
}
```

## Pull Request Guidelines

### Before Submitting
1. **Run Tests**: Ensure all tests pass
2. **Lint**: Run `go fmt` and `go vet`
3. **Documentation**: Update relevant documentation
4. **Security Review**: Verify security implications
5. **Breaking Changes**: Document any breaking changes

### PR Template
```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Security
- [ ] Security implications reviewed
- [ ] No credentials exposed
- [ ] Secure defaults maintained

## Documentation
- [ ] README updated
- [ ] Code comments added
- [ ] Security documentation updated
```

### Code Review Process
1. **Automated Checks**: Ensure CI passes
2. **Code Review**: At least one approval from maintainer
3. **Testing**: Verify tests are comprehensive
4. **Security**: Security review for sensitive changes
5. **Documentation**: Ensure documentation is updated

## Troubleshooting

### Common Issues

#### AWS Credential Problems
```bash
# Check credentials
aws sts get-caller-identity

# Test specific region
aws ec2 describe-regions --region us-east-1
```

#### Permission Errors
- Verify IAM permissions match requirements
- Check for policy conditions that might deny access
- Ensure resource-based policies allow access

#### Rate Limiting
- Implement exponential backoff
- Use batch operations where possible
- Consider regional distribution of requests

#### Resource Limits
- Check service quotas in AWS console
- Request limit increases if needed
- Implement quota-aware resource allocation

### Debug Logging
Enable AWS SDK logging for debugging:

```go
cfg, err := config.LoadDefaultConfig(context.TODO(),
    config.WithClientLogMode(aws.LogRetries|aws.LogRequest|aws.LogResponse),
)
```

## Resources

- [AWS SDK for Go v2 Documentation](https://aws.github.io/aws-sdk-go-v2/docs/)
- [AWS EC2 API Reference](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/)
- [AWS Service Quotas](https://docs.aws.amazon.com/general/latest/gr/ec2-service.html)
- [Brev Cloud SDK Documentation](../../README.md)

## Getting Help

- **Issues**: Create GitHub issues for bugs or feature requests
- **Discussions**: Use GitHub discussions for questions
- **Security**: Report security issues privately to maintainers
- **Documentation**: Improve documentation through pull requests