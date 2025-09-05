package v1

import (
	"context"

	v1 "github.com/brevdev/cloud/v1"
)

// GetLocations returns the current AWS region as a locational API
func (c *AWSClient) GetLocations(ctx context.Context, args v1.GetLocationsArgs) ([]v1.Location, error) {
	// For AWS as a locational API, we only return the current region
	location := v1.Location{
		Name:        c.region,
		Description: c.region,
		Available:   true,
		Priority:    1,
		Country:     determineCountry(c.region),
	}

	return []v1.Location{location}, nil
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