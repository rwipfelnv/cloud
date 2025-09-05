package v1

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	v1 "github.com/brevdev/cloud/v1"
)

// GetLocations retrieves available AWS regions
func (c *AWSClient) GetLocations(ctx context.Context, args v1.GetLocationsArgs) ([]v1.Location, error) {
	input := &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(args.IncludeUnavailable),
	}

	result, err := c.ec2Client.DescribeRegions(ctx, input)
	if err != nil {
		return nil, err
	}

	var locations []v1.Location
	for i, region := range result.Regions {
		location := v1.Location{
			Name:        aws.ToString(region.RegionName),
			Description: aws.ToString(region.RegionName),
			Available:   true, // AWS API only returns available regions by default
			Endpoint:    aws.ToString(region.Endpoint),
			Priority:    i + 1, // Simple priority based on order
			Country:     determineCountry(aws.ToString(region.RegionName)),
		}

		// Mark as unavailable if the region is not opted-in
		if region.OptInStatus != nil {
			switch *region.OptInStatus {
			case "opt-in-not-required", "opted-in":
				location.Available = true
			case "not-opted-in":
				location.Available = args.IncludeUnavailable
			}
		}

		locations = append(locations, location)
	}

	return locations, nil
}

// determineCountry maps AWS regions to ISO 3166-1 alpha-3 country codes
func determineCountry(region string) string {
	// Map AWS regions to ISO 3166-1 alpha-3 country codes
	regionCountryMap := map[string]string{
		"us-east-1":      "USA", // US East (N. Virginia)
		"us-east-2":      "USA", // US East (Ohio)
		"us-west-1":      "USA", // US West (N. California)
		"us-west-2":      "USA", // US West (Oregon)
		"ca-central-1":   "CAN", // Canada (Central)
		"ca-west-1":      "CAN", // Canada (Calgary)
		"eu-west-1":      "IRL", // Europe (Ireland)
		"eu-west-2":      "GBR", // Europe (London)
		"eu-west-3":      "FRA", // Europe (Paris)
		"eu-central-1":   "DEU", // Europe (Frankfurt)
		"eu-central-2":   "CHE", // Europe (Zurich)
		"eu-north-1":     "SWE", // Europe (Stockholm)
		"eu-south-1":     "ITA", // Europe (Milan)
		"eu-south-2":     "ESP", // Europe (Spain)
		"ap-southeast-1": "SGP", // Asia Pacific (Singapore)
		"ap-southeast-2": "AUS", // Asia Pacific (Sydney)
		"ap-southeast-3": "IDN", // Asia Pacific (Jakarta)
		"ap-southeast-4": "AUS", // Asia Pacific (Melbourne)
		"ap-northeast-1": "JPN", // Asia Pacific (Tokyo)
		"ap-northeast-2": "KOR", // Asia Pacific (Seoul)
		"ap-northeast-3": "JPN", // Asia Pacific (Osaka)
		"ap-south-1":     "IND", // Asia Pacific (Mumbai)
		"ap-south-2":     "IND", // Asia Pacific (Hyderabad)
		"ap-east-1":      "HKG", // Asia Pacific (Hong Kong)
		"me-south-1":     "BHR", // Middle East (Bahrain)
		"me-central-1":   "ARE", // Middle East (UAE)
		"af-south-1":     "ZAF", // Africa (Cape Town)
		"sa-east-1":      "BRA", // South America (São Paulo)
	}

	if country, exists := regionCountryMap[region]; exists {
		return country
	}

	// Default to unknown country
	return "UNK"
}