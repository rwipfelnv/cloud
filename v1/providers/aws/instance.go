package v1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/alecthomas/units"
	"github.com/cenkalti/backoff/v4"

	v1 "github.com/brevdev/cloud/v1"
)

// CreateInstance creates a new EC2 instance
func (c *AWSClient) CreateInstance(ctx context.Context, attrs v1.CreateInstanceAttrs) (*v1.Instance, error) {
	// 1. Prepare security group
	sgID, err := c.ensureSecurityGroup(ctx, attrs.FirewallRules, attrs.VPCID, attrs.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to setup security group: %w", err)
	}

	// 2. Prepare key pair
	keyName, err := c.ensureKeyPair(ctx, attrs.PublicKey, attrs.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to setup key pair: %w", err)
	}

	// 3. Prepare user data
	var userData *string
	if attrs.UserDataBase64 != "" {
		userData = &attrs.UserDataBase64
	}

	// 4. Calculate root volume size
	rootVolumeSize := int32(8) // Default 8GB
	if attrs.DiskSize > 0 {
		rootVolumeSize = int32(attrs.DiskSize / units.GiB)
	}

	// 5. Prepare block device mappings
	blockDeviceMappings := []types.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/sda1"), // Root volume for Ubuntu
			Ebs: &types.EbsBlockDevice{
				VolumeSize:          aws.Int32(rootVolumeSize),
				VolumeType:          types.VolumeTypeGp3,
				DeleteOnTermination: aws.Bool(true),
				Encrypted:           aws.Bool(true), // Encrypt by default for security
			},
		},
	}

	// Add additional disks
	for i, disk := range attrs.AdditionalDisks {
		deviceName := fmt.Sprintf("/dev/sd%c", 'f'+i) // Start from /dev/sdf
		mapping := types.BlockDeviceMapping{
			DeviceName: aws.String(deviceName),
			Ebs: &types.EbsBlockDevice{
				VolumeSize:          aws.Int32(int32(disk.Size / units.GiB)),
				VolumeType:          types.VolumeTypeGp3,
				DeleteOnTermination: aws.Bool(true),
				Encrypted:           aws.Bool(true),
			},
		}
		if disk.Type != "" {
			mapping.Ebs.VolumeType = types.VolumeType(disk.Type)
		}
		blockDeviceMappings = append(blockDeviceMappings, mapping)
	}

	// 6. Prepare network interfaces
	networkInterfaces := []types.InstanceNetworkInterfaceSpecification{
		{
			DeviceIndex:              aws.Int32(0),
			AssociatePublicIpAddress: aws.Bool(true),
			Groups:                   []string{sgID},
			DeleteOnTermination:      aws.Bool(true),
		},
	}

	if attrs.SubnetID != "" {
		networkInterfaces[0].SubnetId = aws.String(attrs.SubnetID)
	}

	// 7. Prepare tags
	tags := convertToEC2Tags(attrs.Tags)
	tags = append(tags, types.Tag{
		Key:   aws.String("Name"),
		Value: aws.String(attrs.Name),
	})
	if attrs.RefID != "" {
		tags = append(tags, types.Tag{
			Key:   aws.String("RefID"),
			Value: aws.String(attrs.RefID),
		})
	}

	// 8. Prepare instance request
	runInput := &ec2.RunInstancesInput{
		ImageId:             aws.String(attrs.ImageID),
		InstanceType:        types.InstanceType(attrs.InstanceType),
		MinCount:            aws.Int32(1),
		MaxCount:            aws.Int32(1),
		KeyName:             aws.String(keyName),
		UserData:            userData,
		BlockDeviceMappings: blockDeviceMappings,
		NetworkInterfaces:   networkInterfaces,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags:         tags,
			},
			{
				ResourceType: types.ResourceTypeVolume,
				Tags:         tags,
			},
		},
		MetadataOptions: &types.InstanceMetadataOptionsRequest{
			HttpEndpoint: types.InstanceMetadataEndpointStateEnabled,
			HttpTokens:   types.HttpTokensStateRequired, // Require IMDSv2 for security
		},
	}

	// Add client token for idempotency
	if attrs.RefID != "" {
		runInput.ClientToken = aws.String(attrs.RefID)
	}

	// Handle spot instances
	if attrs.UseSpot {
		runInput.InstanceMarketOptions = &types.InstanceMarketOptionsRequest{
			MarketType: types.MarketTypeSpot,
		}
	}

	// 9. Launch instance with retry
	var result *ec2.RunInstancesOutput
	operation := func() error {
		var err error
		result, err = c.ec2Client.RunInstances(ctx, runInput)
		return err
	}

	if err := backoff.Retry(operation, c.backoff); err != nil {
		return nil, fmt.Errorf("failed to launch instance after retries: %w", err)
	}

	if len(result.Instances) == 0 {
		return nil, fmt.Errorf("no instances returned from RunInstances")
	}

	ec2Instance := result.Instances[0]
	return c.convertToInstance(ec2Instance, attrs.RefID, c.refID)
}

// GetInstance retrieves a specific EC2 instance
func (c *AWSClient) GetInstance(ctx context.Context, id v1.CloudProviderInstanceID) (*v1.Instance, error) {
	result, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{string(id)},
	})
	if err != nil {
		return nil, c.convertEC2Error(err)
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			if aws.ToString(instance.InstanceId) == string(id) {
				refID := extractTagValue(instance.Tags, "RefID")
				return c.convertToInstance(instance, refID, c.refID)
			}
		}
	}

	return nil, v1.ErrInstanceNotFound
}

// ListInstances lists EC2 instances with optional filtering
func (c *AWSClient) ListInstances(ctx context.Context, args v1.ListInstancesArgs) ([]v1.Instance, error) {
	input := &ec2.DescribeInstancesInput{}

	// Add instance ID filters
	if len(args.InstanceIDs) > 0 {
		var ids []string
		for _, id := range args.InstanceIDs {
			ids = append(ids, string(id))
		}
		input.InstanceIds = ids
	}

	// Add filters
	var filters []types.Filter

	// Location filter (region is already handled by client)
	if !args.Locations.IsAll() && len(args.Locations) > 0 {
		// Filter by availability zones if locations don't match current region
		var validZones []string
		for _, loc := range args.Locations {
			if loc == c.region || strings.HasPrefix(loc, c.region+"-") {
				if strings.HasPrefix(loc, c.region+"-") {
					validZones = append(validZones, loc)
				}
			}
		}
		if len(validZones) > 0 {
			filters = append(filters, types.Filter{
				Name:   aws.String("placement.availability-zone"),
				Values: validZones,
			})
		}
	}

	// Tag filters
	for key, values := range args.TagFilters {
		filters = append(filters, types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: values,
		})
	}

	// Exclude terminated instances by default
	filters = append(filters, types.Filter{
		Name:   aws.String("instance-state-name"),
		Values: []string{"pending", "running", "stopping", "stopped", "shutting-down"},
	})

	if len(filters) > 0 {
		input.Filters = filters
	}

	var instances []v1.Instance
	paginator := ec2.NewDescribeInstancesPaginator(c.ec2Client, input)

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list instances: %w", err)
		}

		for _, reservation := range result.Reservations {
			for _, ec2Instance := range reservation.Instances {
				refID := extractTagValue(ec2Instance.Tags, "RefID")
				instance, err := c.convertToInstance(ec2Instance, refID, c.refID)
				if err != nil {
					// Log error but continue processing other instances
					continue
				}
				instances = append(instances, *instance)
			}
		}
	}

	return instances, nil
}

// TerminateInstance terminates an EC2 instance
func (c *AWSClient) TerminateInstance(ctx context.Context, instanceID v1.CloudProviderInstanceID) error {
	_, err := c.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{string(instanceID)},
	})

	if err != nil {
		return c.convertEC2Error(err)
	}

	return nil
}

// StopInstance stops an EC2 instance
func (c *AWSClient) StopInstance(ctx context.Context, instanceID v1.CloudProviderInstanceID) error {
	_, err := c.ec2Client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{string(instanceID)},
	})

	return c.convertEC2Error(err)
}

// StartInstance starts an EC2 instance
func (c *AWSClient) StartInstance(ctx context.Context, instanceID v1.CloudProviderInstanceID) error {
	_, err := c.ec2Client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{string(instanceID)},
	})

	return c.convertEC2Error(err)
}

// RebootInstance reboots an EC2 instance
func (c *AWSClient) RebootInstance(ctx context.Context, instanceID v1.CloudProviderInstanceID) error {
	_, err := c.ec2Client.RebootInstances(ctx, &ec2.RebootInstancesInput{
		InstanceIds: []string{string(instanceID)},
	})

	return c.convertEC2Error(err)
}

// MergeInstanceForUpdate merges instance updates
func (c *AWSClient) MergeInstanceForUpdate(currInst v1.Instance, newInst v1.Instance) v1.Instance {
	// For AWS, we generally prefer new instance data, but preserve some immutable fields
	merged := newInst
	merged.RefID = currInst.RefID           // RefID should not change
	merged.CloudCredRefID = currInst.CloudCredRefID // CloudCredRefID should not change
	merged.CreatedAt = currInst.CreatedAt   // CreatedAt should not change
	
	// Preserve empty fields from current instance
	if merged.Name == "" {
		merged.Name = currInst.Name
	}
	if merged.Location == "" {
		merged.Location = currInst.Location
	}
	if merged.SubLocation == "" {
		merged.SubLocation = currInst.SubLocation
	}
	
	return merged
}

// MergeInstanceTypeForUpdate merges instance type updates  
func (c *AWSClient) MergeInstanceTypeForUpdate(currIt v1.InstanceType, newIt v1.InstanceType) v1.InstanceType {
	// For instance types, generally prefer new data but preserve stable IDs
	merged := newIt
	if merged.ID == "" {
		merged.ID = currIt.ID
	}
	return merged
}

// Helper functions

func (c *AWSClient) ensureKeyPair(ctx context.Context, publicKey, name string) (string, error) {
	if publicKey == "" {
		return "", fmt.Errorf("public key is required")
	}

	// Generate a unique key name
	keyName := fmt.Sprintf("%s-%s-%d", name, c.region, time.Now().Unix())

	_, err := c.ec2Client.ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
		KeyName:           aws.String(keyName),
		PublicKeyMaterial: []byte(publicKey),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeKeyPair,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(keyName),
					},
					{
						Key:   aws.String("CreatedBy"),
						Value: aws.String("brev-cloud-sdk"),
					},
				},
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to import key pair: %w", err)
	}

	return keyName, nil
}

func (c *AWSClient) ensureSecurityGroup(ctx context.Context, rules v1.FirewallRules, vpcID, name string) (string, error) {
	// Generate a unique security group name
	sgName := fmt.Sprintf("%s-sg-%d", name, time.Now().Unix())
	sgDescription := fmt.Sprintf("Security group for %s", name)

	// Create security group
	createResult, err := c.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(sgName),
		Description: aws.String(sgDescription),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(sgName),
					},
					{
						Key:   aws.String("CreatedBy"),
						Value: aws.String("brev-cloud-sdk"),
					},
				},
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to create security group: %w", err)
	}

	sgID := aws.ToString(createResult.GroupId)

	// Add ingress rules
	if len(rules.IngressRules) > 0 {
		var permissions []types.IpPermission
		for _, rule := range rules.IngressRules {
			perm := types.IpPermission{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(rule.FromPort),
				ToPort:     aws.Int32(rule.ToPort),
			}

			for _, ipRange := range rule.IPRanges {
				perm.IpRanges = append(perm.IpRanges, types.IpRange{
					CidrIp: aws.String(ipRange),
				})
			}

			permissions = append(permissions, perm)
		}

		_, err = c.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       aws.String(sgID),
			IpPermissions: permissions,
		})

		if err != nil {
			return "", fmt.Errorf("failed to authorize ingress rules: %w", err)
		}
	}

	return sgID, nil
}

func (c *AWSClient) convertToInstance(ec2Instance types.Instance, refID, cloudCredRefID string) (*v1.Instance, error) {
	instance := &v1.Instance{
		CloudID:         v1.CloudProviderInstanceID(aws.ToString(ec2Instance.InstanceId)),
		RefID:           refID,
		CloudCredRefID:  cloudCredRefID,
		Name:            extractNameTag(ec2Instance.Tags),
		CreatedAt:       aws.ToTime(ec2Instance.LaunchTime),
		ImageID:         aws.ToString(ec2Instance.ImageId),
		InstanceType:    string(ec2Instance.InstanceType),
		Location:        c.region,
		Status:          convertEC2State(ec2Instance.State),
		VPCID:           aws.ToString(ec2Instance.VpcId),
		SubnetID:        aws.ToString(ec2Instance.SubnetId),
		Spot:            ec2Instance.SpotInstanceRequestId != nil,
		Tags:            convertFromEC2Tags(ec2Instance.Tags),
		SSHUser:         determineSSHUser(aws.ToString(ec2Instance.ImageId)),
		SSHPort:         22,
		Stoppable:       true,
		Rebootable:      true,
	}

	// Set availability zone
	if ec2Instance.Placement != nil {
		instance.SubLocation = aws.ToString(ec2Instance.Placement.AvailabilityZone)
	}

	// Set IP addresses
	if ec2Instance.PublicIpAddress != nil {
		instance.PublicIP = aws.ToString(ec2Instance.PublicIpAddress)
	}
	if ec2Instance.PrivateIpAddress != nil {
		instance.PrivateIP = aws.ToString(ec2Instance.PrivateIpAddress)
	}
	if ec2Instance.PublicDnsName != nil {
		instance.PublicDNS = aws.ToString(ec2Instance.PublicDnsName)
	}

	// Set hostname
	if instance.PublicDNS != "" {
		instance.Hostname = instance.PublicDNS
	} else if instance.PublicIP != "" {
		instance.Hostname = instance.PublicIP
	}

	// Calculate total disk size (would need additional API calls for exact sizes)
	if len(ec2Instance.BlockDeviceMappings) > 0 {
		// For now, set a placeholder - in a full implementation, you'd call DescribeVolumes
		instance.DiskSize = 8 * units.GiB // Default assumption
	}

	return instance, nil
}

func convertEC2State(state *types.InstanceState) v1.Status {
	if state == nil {
		return v1.Status{LifecycleStatus: v1.LifecycleStatusPending}
	}

	var lifecycle v1.LifecycleStatus
	switch state.Name {
	case types.InstanceStateNamePending:
		lifecycle = v1.LifecycleStatusPending
	case types.InstanceStateNameRunning:
		lifecycle = v1.LifecycleStatusRunning
	case types.InstanceStateNameStopping:
		lifecycle = v1.LifecycleStatusStopping
	case types.InstanceStateNameStopped:
		lifecycle = v1.LifecycleStatusStopped
	case types.InstanceStateNameShuttingDown:
		lifecycle = v1.LifecycleStatusTerminating
	case types.InstanceStateNameTerminated:
		lifecycle = v1.LifecycleStatusTerminated
	default:
		lifecycle = v1.LifecycleStatusFailed
	}

	return v1.Status{
		LifecycleStatus: lifecycle,
		Messages:        []string{string(state.Name)},
	}
}

func extractNameTag(tags []types.Tag) string {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == "Name" {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}

func extractTagValue(tags []types.Tag, key string) string {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == key {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}

func convertToEC2Tags(tags v1.Tags) []types.Tag {
	var ec2Tags []types.Tag
	for key, value := range tags {
		ec2Tags = append(ec2Tags, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	return ec2Tags
}

func convertFromEC2Tags(ec2Tags []types.Tag) v1.Tags {
	tags := make(v1.Tags)
	for _, tag := range ec2Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tags
}

func determineSSHUser(imageID string) string {
	// This is a simple heuristic - in practice, you might query the AMI details
	// or maintain a mapping of known AMI patterns to SSH users
	if strings.Contains(imageID, "ubuntu") {
		return "ubuntu"
	}
	if strings.Contains(imageID, "amzn") {
		return "ec2-user"
	}
	// Default to ubuntu for most cases
	return "ubuntu"
}

func (c *AWSClient) convertEC2Error(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()
	if strings.Contains(errStr, "InvalidInstanceID.NotFound") {
		return v1.ErrInstanceNotFound
	}
	if strings.Contains(errStr, "InsufficientInstanceCapacity") {
		return v1.ErrInsufficientResources
	}
	if strings.Contains(errStr, "InstanceLimitExceeded") {
		return v1.ErrOutOfQuota
	}
	if strings.Contains(errStr, "ServiceUnavailable") {
		return v1.ErrServiceUnavailable
	}

	return err
}