# AWS Provider

This directory contains the AWS provider implementation for the Brev Cloud SDK (v1).

## Overview

The AWS provider implements the CloudClient interface defined in `pkg/v1` to provide access to Amazon Web Services cloud infrastructure, primarily through the EC2 service. This implementation is based on the AWS SDK for Go v2.

## Supported Features

Based on the AWS EC2 API documentation, the following features are **SUPPORTED**:

### Core Instance Management
- ✅ **Create Instance**: `ec2:RunInstances`
- ✅ **Get Instance**: `ec2:DescribeInstances`
- ✅ **List Instances**: `ec2:DescribeInstances` with filtering
- ✅ **Terminate Instance**: `ec2:TerminateInstances`

### Advanced Instance Operations
- ✅ **Stop Instance**: `ec2:StopInstances`
- ✅ **Start Instance**: `ec2:StartInstances`
- ✅ **Reboot Instance**: `ec2:RebootInstances`
- ✅ **Change Instance Type**: `ec2:ModifyInstanceAttribute`

### Instance Types
- ✅ **Get Instance Types**: `ec2:DescribeInstanceTypes`
- ✅ **Get Availability Zones**: `ec2:DescribeAvailabilityZones`

### Storage Management
- ✅ **Resize Instance Volume**: `ec2:ModifyVolume`
- ✅ **EBS Volume Support**: Multiple volume types (gp3, gp2, io1, io2, st1, sc1)
- ✅ **Instance Store Support**: NVMe SSD local storage

### Network Security
- ✅ **Create Security Groups**: `ec2:CreateSecurityGroup`
- ✅ **Modify Firewall Rules**: `ec2:AuthorizeSecurityGroupIngress/Egress`
- ✅ **Revoke Security Group Rules**: `ec2:RevokeSecurityGroupIngress/Egress`

### Resource Management
- ✅ **Instance Tags**: `ec2:CreateTags`, `ec2:DeleteTags`
- ✅ **AMI Management**: `ec2:DescribeImages`
- ✅ **Key Pair Management**: `ec2:ImportKeyPair`

### Location Management
- ✅ **Get Regions**: `ec2:DescribeRegions`
- ✅ **Multi-region Support**: Regional API clients

### Additional Features
- ✅ **Spot Instances**: Instance market options for cost optimization
- ✅ **User Data**: Instance initialization scripts
- ✅ **Metadata Service**: IMDSv2 support for security
- ✅ **Encrypted EBS Volumes**: Default encryption for data protection

## Implementation Approach

This implementation follows the repository's recommended pattern:

- **Embedding `NotImplCloudClient`**: Unsupported features return `ErrNotImplemented` gracefully
- **AWS SDK v2**: Uses the latest AWS SDK with context support and error wrapping
- **Regional Clients**: Implements `APITypeLocational` for region-specific operations
- **Comprehensive Error Handling**: Maps AWS errors to v1 standard errors
- **Security by Default**: Implements secure defaults (encrypted volumes, IMDSv2, etc.)

## AWS Services Integration

The provider integrates with the following AWS services:

- **EC2 (Elastic Compute Cloud)**: Primary service for instance management
- **EBS (Elastic Block Store)**: Volume management and storage
- **VPC (Virtual Private Cloud)**: Network isolation and security groups
- **IAM (Identity and Access Management)**: Credential and permission management

## Authentication

The AWS provider supports multiple authentication methods:

### Static Credentials
```go
credential := NewAWSCredential("ref-id", "access-key", "secret-key")
```

### Session Token (Temporary Credentials)
```go
credential := NewAWSCredential("ref-id", "access-key", "secret-key",
    WithSessionToken("session-token"))
```

### AWS Profile
```go
credential := NewAWSCredential("ref-id", "", "",
    WithProfile("my-profile"))
```

### Environment Variables
The AWS SDK automatically picks up credentials from:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY` 
- `AWS_SESSION_TOKEN`
- `AWS_PROFILE`

## Configuration

### Required Environment Variables for Testing
```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_DEFAULT_REGION="us-east-1"
export AWS_VPC_ID="vpc-xxxxxxxxx"      # For validation tests
export AWS_SUBNET_ID="subnet-xxxxxxxxx" # For validation tests
```

### Instance Creation Parameters
The provider supports comprehensive instance configuration:

- **Instance Types**: All current EC2 instance types (t3, m5, c5, r5, p3, g4, etc.)
- **AMIs**: Ubuntu, Amazon Linux, custom AMIs
- **Storage**: Root volume sizing, additional EBS volumes, instance store
- **Networking**: VPC, subnet, security groups, public IP allocation
- **Security**: SSH key injection, security group rules, encrypted volumes
- **Tagging**: Resource tagging for management and billing

## GPU Support

The AWS provider includes comprehensive GPU instance support:

- **Instance Types**: P3, P4, G4, G5 families
- **GPU Metadata**: Memory, count, manufacturer information
- **Pricing**: Specialized pricing for GPU instances
- **Quotas**: GPU-specific quota management

## Security Features

The implementation follows security best practices:

### Default Security Posture
- **Encrypted EBS volumes** by default
- **IMDSv2** required for instance metadata
- **Default security groups** with deny-all inbound rules
- **SSH key management** with automatic key pair creation

### Network Security
- **Security group isolation** per instance
- **VPC integration** for network isolation  
- **Firewall rule management** with ingress/egress control
- **Public IP association** configurable

## Pricing Integration

The provider includes pricing estimation:

- **Base pricing** for common instance types
- **EBS storage pricing** by volume type
- **Regional pricing** variations (placeholder)
- **Spot instance** pricing support

*Note: Current pricing is estimated. Production usage should integrate with AWS Pricing API.*

## Error Handling

AWS-specific errors are mapped to standard v1 errors:

- `InsufficientInstanceCapacity` → `ErrInsufficientResources`
- `InstanceLimitExceeded` → `ErrOutOfQuota`
- `InvalidInstanceID.NotFound` → `ErrInstanceNotFound`
- `InvalidPermission.Duplicate` → `ErrDuplicateFirewallRule`
- `ServiceUnavailable` → `ErrServiceUnavailable`

## Testing

### Unit Tests
```bash
go test ./v1/providers/aws/
```

### Validation Tests
```bash
# Requires real AWS credentials
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
go test -v ./v1/providers/aws/ -run TestAWSValidationSuite
```

### Integration with Validation Suite
```bash
make test-validation-aws  # If Makefile target exists
```

## Limitations

### Current Limitations
- **Pricing**: Uses estimated pricing, not real-time AWS Pricing API
- **Advanced Networking**: Limited VPC/subnet auto-creation
- **Instance Store**: Limited configuration options for NVMe storage
- **Reserved Instances**: No support for RI purchasing/management

### AWS Service Limits
- **API Rate Limits**: EC2 APIs have various rate limits
- **Regional Quotas**: Instance type availability varies by region
- **Spot Instance**: Subject to availability and interruption

## Contributing

When contributing to the AWS provider:

1. **Follow AWS SDK patterns**: Use proper context handling and error wrapping
2. **Security first**: Default to secure configurations
3. **Test thoroughly**: Include both unit and integration tests
4. **Document changes**: Update this README for new features
5. **Error handling**: Map AWS errors appropriately

## References

- **AWS SDK for Go v2**: https://github.com/aws/aws-sdk-go-v2
- **EC2 API Reference**: https://docs.aws.amazon.com/ec2/latest/api/
- **AWS Service Limits**: https://docs.aws.amazon.com/general/latest/gr/ec2-service.html
- **Brev Cloud SDK**: Core interfaces in `pkg/v1/`