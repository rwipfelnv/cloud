package v1

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/brevdev/cloud/v1"
)

// GetImages retrieves available AMIs
func (c *AWSClient) GetImages(ctx context.Context, args v1.GetImageArgs) ([]v1.Image, error) {
	input := &ec2.DescribeImagesInput{}

	// Set owners if specified
	if len(args.Owners) > 0 {
		input.Owners = args.Owners
	} else {
		// Default to self and Amazon if no owners specified
		input.Owners = []string{"self", "amazon"}
	}

	// Set image IDs if specified
	if len(args.ImageIDs) > 0 {
		input.ImageIds = args.ImageIDs
	}

	// Build filters
	var filters []types.Filter

	// Architecture filters
	if len(args.Architectures) > 0 {
		filters = append(filters, types.Filter{
			Name:   aws.String("architecture"),
			Values: args.Architectures,
		})
	}

	// Name filters (support wildcards)
	if len(args.NameFilters) > 0 {
		filters = append(filters, types.Filter{
			Name:   aws.String("name"),
			Values: args.NameFilters,
		})
	}

	// Only include available images
	filters = append(filters, types.Filter{
		Name:   aws.String("state"),
		Values: []string{"available"},
	})

	// Only include EBS-backed images (most common for EC2)
	filters = append(filters, types.Filter{
		Name:   aws.String("root-device-type"),
		Values: []string{"ebs"},
	})

	// Only include images with public launch permissions or owned by us
	if contains(args.Owners, "amazon") || contains(args.Owners, "aws-marketplace") {
		filters = append(filters, types.Filter{
			Name:   aws.String("is-public"),
			Values: []string{"true"},
		})
	}

	if len(filters) > 0 {
		input.Filters = filters
	}

	result, err := c.ec2Client.DescribeImages(ctx, input)
	if err != nil {
		return nil, err
	}

	var images []v1.Image
	for _, ami := range result.Images {
		image := v1.Image{
			ID:           aws.ToString(ami.ImageId),
			Architecture: string(ami.Architecture),
			Description:  aws.ToString(ami.Description),
			Name:         aws.ToString(ami.Name),
			CreatedAt:    parseAMIDate(aws.ToString(ami.CreationDate)),
		}

		images = append(images, image)
	}

	return images, nil
}

// Helper function to check if slice contains a value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// parseAMIDate parses AWS AMI creation date format
func parseAMIDate(dateStr string) time.Time {
	// AWS AMI creation date format: "2023-10-12T14:30:00.000Z"
	if dateStr == "" {
		return time.Time{}
	}

	// Try parsing ISO format first
	if t, err := time.Parse("2006-01-02T15:04:05.000Z", dateStr); err == nil {
		return t
	}

	// Try alternative format without milliseconds
	if t, err := time.Parse("2006-01-02T15:04:05Z", dateStr); err == nil {
		return t
	}

	// If parsing fails, return zero time
	return time.Time{}
}