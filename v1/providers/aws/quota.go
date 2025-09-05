package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/brevdev/cloud/v1"
)

// GetInstanceTypeQuotas retrieves quotas for specific instance types
func (c *AWSClient) GetInstanceTypeQuotas(ctx context.Context, args v1.GetInstanceTypeQuotasArgs) (v1.Quota, error) {
	// AWS doesn't have a direct API for instance type quotas like GCP/Azure
	// We need to infer limits based on running/launched instances and general service limits
	
	// Get current running instances of this type
	current, err := c.getCurrentInstanceCount(ctx, args.InstanceType)
	if err != nil {
		return v1.Quota{}, fmt.Errorf("failed to get current instance count: %w", err)
	}

	// Determine quota based on instance type family
	maximum := c.estimateInstanceTypeQuota(args.InstanceType)
	
	quota := v1.Quota{
		ID:      fmt.Sprintf("%s-%s-quota", c.region, args.InstanceType),
		Name:    fmt.Sprintf("%s instances in %s", args.InstanceType, c.region),
		Current: current,
		Maximum: maximum,
		Unit:    "instances",
	}

	// For GPU instances, also check GPU limits
	if c.isGPUInstanceType(args.InstanceType) {
		quota.Unit = "gpu"
		// GPU instances typically have more restrictive limits
		quota.Maximum = quota.Maximum / 2 // Conservative estimate
	}

	return quota, nil
}

func (c *AWSClient) getCurrentInstanceCount(ctx context.Context, instanceType string) (int, error) {
	result, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("instance-type"),
				Values: []string{instanceType},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []string{
					"pending", "running", "stopping", "stopped",
				},
			},
		},
	})
	if err != nil {
		return 0, err
	}

	count := 0
	for _, reservation := range result.Reservations {
		count += len(reservation.Instances)
	}

	return count, nil
}

func (c *AWSClient) estimateInstanceTypeQuota(instanceType string) int {
	// Default quotas based on AWS service limits
	// These are conservative estimates - actual limits may be higher
	
	switch {
	// GPU instances typically have low limits
	case c.isGPUInstanceType(instanceType):
		switch {
		case strings.HasPrefix(instanceType, "p4"):
			return 8  // P4 instances have very low limits
		case strings.HasPrefix(instanceType, "p3"):
			return 16 // P3 instances
		case strings.HasPrefix(instanceType, "g4"):
			return 32 // G4 instances
		case strings.HasPrefix(instanceType, "g3"):
			return 16 // G3 instances
		default:
			return 8  // Conservative for unknown GPU types
		}
	
	// High-memory instances
	case strings.HasPrefix(instanceType, "x1") || 
		 strings.HasPrefix(instanceType, "x2") ||
		 strings.HasPrefix(instanceType, "z1"):
		return 20
		
	// High-performance instances
	case strings.HasPrefix(instanceType, "c5n") ||
		 strings.HasPrefix(instanceType, "c6i") ||
		 strings.HasPrefix(instanceType, "m5n") ||
		 strings.HasPrefix(instanceType, "r5n"):
		return 50
		
	// Burstable instances
	case strings.HasPrefix(instanceType, "t2") ||
		 strings.HasPrefix(instanceType, "t3") ||
		 strings.HasPrefix(instanceType, "t4"):
		return 100
		
	// General purpose instances
	case strings.HasPrefix(instanceType, "m5") ||
		 strings.HasPrefix(instanceType, "m6"):
		return 100
		
	// Compute optimized instances
	case strings.HasPrefix(instanceType, "c5") ||
		 strings.HasPrefix(instanceType, "c6"):
		return 100
		
	// Memory optimized instances
	case strings.HasPrefix(instanceType, "r5") ||
		 strings.HasPrefix(instanceType, "r6"):
		return 100
		
	default:
		// Conservative default for unknown instance types
		return 20
	}
}

func (c *AWSClient) isGPUInstanceType(instanceType string) bool {
	gpuFamilies := []string{"p2", "p3", "p4", "g2", "g3", "g4", "g5"}
	
	for _, family := range gpuFamilies {
		if strings.HasPrefix(instanceType, family) {
			return true
		}
	}
	
	return false
}