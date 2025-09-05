package v1

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/alecthomas/units"

	v1 "github.com/brevdev/cloud/v1"
)

// ResizeInstanceVolume resizes the root volume of an EC2 instance
func (c *AWSClient) ResizeInstanceVolume(ctx context.Context, args v1.ResizeInstanceVolumeArgs) error {
	// Get the instance's root volume ID
	volumeID, err := c.getRootVolumeID(ctx, args.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to get root volume ID: %w", err)
	}

	// Calculate new size in GB
	newSizeGB := int32(args.Size / units.GiB)

	// Modify the volume
	_, err = c.ec2Client.ModifyVolume(ctx, &ec2.ModifyVolumeInput{
		VolumeId: aws.String(volumeID),
		Size:     aws.Int32(newSizeGB),
	})
	if err != nil {
		return fmt.Errorf("failed to modify volume: %w", err)
	}

	// Wait for optimization to complete if requested
	if args.WaitForOptimizing {
		err = c.waitForVolumeOptimization(ctx, volumeID)
		if err != nil {
			return fmt.Errorf("failed waiting for volume optimization: %w", err)
		}
	}

	return nil
}

func (c *AWSClient) getRootVolumeID(ctx context.Context, instanceID v1.CloudProviderInstanceID) (string, error) {
	result, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{string(instanceID)},
	})
	if err != nil {
		return "", err
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			if aws.ToString(instance.InstanceId) == string(instanceID) {
				// Find the root volume (typically /dev/sda1 or /dev/xvda)
				for _, bdm := range instance.BlockDeviceMappings {
					if aws.ToString(bdm.DeviceName) == "/dev/sda1" || 
					   aws.ToString(bdm.DeviceName) == "/dev/xvda" {
						if bdm.Ebs != nil {
							return aws.ToString(bdm.Ebs.VolumeId), nil
						}
					}
				}
				// If no specific root device found, use the first EBS volume
				for _, bdm := range instance.BlockDeviceMappings {
					if bdm.Ebs != nil {
						return aws.ToString(bdm.Ebs.VolumeId), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no root volume found for instance %s", instanceID)
}

func (c *AWSClient) waitForVolumeOptimization(ctx context.Context, volumeID string) error {
	// Note: AWS SDK v2 doesn't have a built-in volume optimization waiter
	// In a production implementation, you would implement a custom waiter
	// or poll the volume state until optimization is complete
	
	// For now, just return nil as a placeholder
	// TODO: Implement proper volume optimization waiting
	return nil
}

// ChangeInstanceType changes the instance type of an EC2 instance
func (c *AWSClient) ChangeInstanceType(ctx context.Context, instanceID v1.CloudProviderInstanceID, instanceType string) error {
	// First, stop the instance if it's running
	instanceObj, err := c.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	wasRunning := instanceObj.Status.LifecycleStatus == v1.LifecycleStatusRunning

	if wasRunning {
		err = c.StopInstance(ctx, instanceID)
		if err != nil {
			return fmt.Errorf("failed to stop instance: %w", err)
		}

		// Wait for instance to be stopped
		waiter := ec2.NewInstanceStoppedWaiter(c.ec2Client)
		err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: []string{string(instanceID)},
		}, 5*60) // Wait up to 5 minutes
		if err != nil {
			return fmt.Errorf("timeout waiting for instance to stop: %w", err)
		}
	}

	// Modify the instance type
	_, err = c.ec2Client.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
		InstanceId: aws.String(string(instanceID)),
		InstanceType: &types.AttributeValue{
			Value: aws.String(instanceType),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to modify instance type: %w", err)
	}

	// Restart the instance if it was running
	if wasRunning {
		err = c.StartInstance(ctx, instanceID)
		if err != nil {
			return fmt.Errorf("failed to restart instance: %w", err)
		}
	}

	return nil
}