package v1

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/brevdev/cloud/v1"
)

// UpdateInstanceTags updates the tags of an EC2 instance
func (c *AWSClient) UpdateInstanceTags(ctx context.Context, args v1.UpdateInstanceTagsArgs) error {
	if len(args.Tags) == 0 {
		return nil // Nothing to do
	}

	// Convert v1.Tags to EC2 tags
	var ec2Tags []types.Tag
	for key, value := range args.Tags {
		ec2Tags = append(ec2Tags, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	// Create or update tags
	_, err := c.ec2Client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{string(args.InstanceID)},
		Tags:      ec2Tags,
	})
	if err != nil {
		return fmt.Errorf("failed to update instance tags: %w", err)
	}

	return nil
}

// DeleteInstanceTags deletes specific tags from an EC2 instance
func (c *AWSClient) DeleteInstanceTags(ctx context.Context, instanceID v1.CloudProviderInstanceID, tagKeys []string) error {
	if len(tagKeys) == 0 {
		return nil // Nothing to do
	}

	// Convert tag keys to EC2 tag format
	var ec2Tags []types.Tag
	for _, key := range tagKeys {
		ec2Tags = append(ec2Tags, types.Tag{
			Key: aws.String(key),
			// Value is not specified for deletion
		})
	}

	// Delete tags
	_, err := c.ec2Client.DeleteTags(ctx, &ec2.DeleteTagsInput{
		Resources: []string{string(instanceID)},
		Tags:      ec2Tags,
	})
	if err != nil {
		return fmt.Errorf("failed to delete instance tags: %w", err)
	}

	return nil
}

// GetInstanceTags retrieves all tags for an EC2 instance
func (c *AWSClient) GetInstanceTags(ctx context.Context, instanceID v1.CloudProviderInstanceID) (v1.Tags, error) {
	result, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{string(instanceID)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance: %w", err)
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			if aws.ToString(instance.InstanceId) == string(instanceID) {
				return convertFromEC2Tags(instance.Tags), nil
			}
		}
	}

	return nil, v1.ErrInstanceNotFound
}