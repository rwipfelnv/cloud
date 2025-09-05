# AWS Provider Security Requirements

This document outlines security requirements and implementation details specific to the AWS provider, complementing the main [Security Requirements](../../docs/SECURITY.md).

## Overview

The AWS provider implements security-by-default practices following AWS Well-Architected Framework principles and the main Brev Cloud SDK security model.

## Network Security

### Default Security Model
The AWS provider implements the required **"deny all inbound, allow all outbound"** model through:

#### Security Groups
- **Default Inbound**: All inbound traffic blocked by default
- **SSH Access**: Only port 22 inbound allowed from specified IP ranges
- **Outbound**: All outbound traffic allowed (0.0.0.0/0:0-65535)
- **Instance-Level**: Each instance gets a dedicated security group

#### Implementation Details
```go
// Default security group creation in instance.go
func (c *AWSClient) ensureSecurityGroup(ctx context.Context, rules v1.FirewallRules, vpcID, name string) (string, error) {
    // Creates security group with no default inbound rules
    // Only adds explicitly specified firewall rules
    // All outbound traffic allowed by default
}
```

### VPC Integration
- **VPC Isolation**: Instances launched in specified VPC for network isolation
- **Subnet Placement**: Instances placed in specified subnets
- **Public IP**: Configurable public IP assignment
- **Network ACLs**: Inherits VPC-level network ACL rules

## Instance Security

### SSH Key Management
**Requirement**: All instances must have SSH server accessible with key-based authentication.

#### Implementation
- **Automatic Key Pair Creation**: Creates unique key pairs per instance
- **Public Key Injection**: User-provided public keys imported into AWS
- **No Password Authentication**: Only key-based SSH access supported
- **Key Naming**: Keys named with timestamp for uniqueness

```go
func (c *AWSClient) ensureKeyPair(ctx context.Context, publicKey, name string) (string, error) {
    keyName := fmt.Sprintf("%s-%s-%d", name, c.region, time.Now().Unix())
    // Imports public key to AWS and associates with instance
}
```

### Instance Metadata Security
- **IMDSv2 Required**: Instance Metadata Service v2 enforced
- **Token Required**: Metadata access requires session tokens
- **Network Protection**: Prevents SSRF attacks via metadata service

```go
MetadataOptions: &types.InstanceMetadataOptionsRequest{
    HttpEndpoint: types.InstanceMetadataEndpointStateEnabled,
    HttpTokens:   types.HttpTokensStateRequired, // IMDSv2 only
}
```

## Data Protection

### Encryption at Rest
**Requirement**: All storage must be encrypted at rest.

#### EBS Volume Encryption
- **Default Encryption**: All EBS volumes encrypted by default
- **Root Volumes**: Root volume encryption enforced
- **Additional Volumes**: Additional disks encrypted by default
- **AWS KMS**: Uses AWS-managed keys (can be customized)

```go
Ebs: &types.EbsBlockDevice{
    VolumeSize:          aws.Int32(rootVolumeSize),
    VolumeType:          types.VolumeTypeGp3,
    DeleteOnTermination: aws.Bool(true),
    Encrypted:           aws.Bool(true), // Always encrypted
}
```

#### Instance Store Encryption
- **Hardware Encryption**: Instance store SSDs encrypted at hardware level
- **Ephemeral Nature**: Data automatically destroyed on instance termination

### Encryption in Transit
- **SSH Communication**: All management communication over SSH
- **API Calls**: AWS API calls use TLS 1.2+
- **Inter-Instance**: Application-level encryption recommended for inter-instance communication

## Access Control

### IAM Integration
**Requirement**: Credential management follows AWS IAM best practices.

#### Supported Authentication Methods
1. **IAM User Access Keys**: Traditional access key/secret pairs
2. **IAM Roles**: Instance roles and cross-account role assumption
3. **Temporary Credentials**: STS temporary credentials with session tokens
4. **AWS Profiles**: Local credential profiles

#### Permission Requirements
Minimum required IAM permissions for the AWS provider:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ec2:RunInstances",
                "ec2:DescribeInstances",
                "ec2:TerminateInstances",
                "ec2:StartInstances",
                "ec2:StopInstances",
                "ec2:RebootInstances",
                "ec2:DescribeInstanceTypes",
                "ec2:DescribeImages",
                "ec2:DescribeRegions",
                "ec2:DescribeAvailabilityZones",
                "ec2:CreateSecurityGroup",
                "ec2:AuthorizeSecurityGroupIngress",
                "ec2:AuthorizeSecurityGroupEgress",
                "ec2:RevokeSecurityGroupIngress",
                "ec2:RevokeSecurityGroupEgress",
                "ec2:ImportKeyPair",
                "ec2:CreateTags",
                "ec2:DeleteTags",
                "ec2:ModifyVolume",
                "ec2:ModifyInstanceAttribute"
            ],
            "Resource": "*"
        }
    ]
}
```

### Credential Security
- **No Hardcoded Credentials**: Credentials loaded from environment/profiles
- **Credential Rotation**: Supports credential rotation via environment updates
- **Least Privilege**: Implements minimal required permissions
- **Audit Trail**: All operations logged in CloudTrail (AWS-level)

## Firewall Rules

### Security Group Management
**Requirement**: Firewall rules must be explicitly configured and modifiable.

#### Rule Implementation
- **Explicit Rules**: Only explicitly specified rules allowed
- **Protocol Support**: TCP, UDP, ICMP rule support
- **CIDR Blocks**: Support for IP range specifications
- **Port Ranges**: Single ports and port ranges supported

#### Default Behavior
```go
// No default inbound rules - must be explicitly specified
// SSH access only added if specified in firewall rules
// All outbound traffic allowed by default (AWS security group behavior)
```

### Limitations and Considerations
- **Security Group Limits**: AWS limits per region (typically 2,500 security groups)
- **Rules per Security Group**: Typically 60 inbound + 60 outbound rules
- **Stateful**: Security groups are stateful (return traffic automatically allowed)

## Operating System Security

### Ubuntu 22.04 Requirements
**Requirement**: Support Ubuntu 22.04 with specific security configurations.

#### AMI Selection
- **Official AMIs**: Prefer official Ubuntu AMIs
- **Security Updates**: Use AMIs with latest security patches
- **Minimal Images**: Use minimal Ubuntu images when possible

#### SSH Configuration
- **Default User**: `ubuntu` user for Ubuntu AMIs
- **Root Access**: Root access typically disabled by default
- **SSH Keys**: Only SSH key authentication enabled
- **SSH Port**: Standard port 22 (configurable)

### System Requirements
- **Systemd**: Required for service management
- **SSH Server**: OpenSSH server must be running
- **Package Manager**: APT package manager available

## Monitoring and Compliance

### Security Monitoring
- **CloudTrail**: All API calls logged in AWS CloudTrail
- **VPC Flow Logs**: Network traffic monitoring available
- **Instance Monitoring**: CloudWatch integration for system metrics
- **Security Groups**: Changes tracked and logged

### Compliance Considerations
- **SOC 2**: AWS infrastructure SOC 2 compliant
- **ISO 27001**: AWS infrastructure ISO 27001 certified
- **Data Residency**: Instance placement in specified regions
- **Audit Logging**: Comprehensive audit trail via CloudTrail

## Security Limitations

### Current Limitations
1. **Network ACLs**: Limited support for VPC-level network ACLs
2. **WAF Integration**: No Web Application Firewall integration
3. **Secrets Management**: No AWS Secrets Manager integration for application secrets
4. **Certificate Management**: No automatic SSL/TLS certificate management

### AWS-Specific Considerations
1. **Shared Responsibility**: Security responsibilities shared between AWS and customer
2. **Region Isolation**: Instances isolated at AWS region level
3. **AZ Isolation**: Availability zone isolation for fault tolerance
4. **Hypervisor Security**: AWS Nitro system provides hardware-level isolation

## Security Best Practices

### Development
1. **Use IAM Roles**: Prefer IAM roles over access keys when possible
2. **Rotate Credentials**: Regularly rotate access keys
3. **Minimal Permissions**: Grant only necessary permissions
4. **Environment Variables**: Store credentials in environment variables, not code
5. **Encrypt Everything**: Default to encryption for all data

### Production
1. **VPC Isolation**: Use dedicated VPCs for production workloads
2. **Multi-AZ**: Deploy across multiple availability zones
3. **Security Groups**: Use specific, minimal security group rules
4. **Monitoring**: Enable comprehensive monitoring and alerting
5. **Backup**: Regular snapshots of critical volumes

### Incident Response
1. **CloudTrail**: Monitor CloudTrail logs for suspicious activity
2. **Isolation**: Ability to quickly isolate compromised instances
3. **Forensics**: EBS snapshots for forensic analysis
4. **Recovery**: Documented procedures for security incident recovery

For additional security guidance, refer to:
- [AWS Well-Architected Security Pillar](https://docs.aws.amazon.com/wellarchitected/latest/security-pillar/)
- [AWS Security Best Practices](https://aws.amazon.com/security/security-resources/)
- [Main Brev Cloud SDK Security Requirements](../../docs/SECURITY.md)