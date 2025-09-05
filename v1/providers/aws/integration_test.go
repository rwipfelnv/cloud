package v1

import (
	"context"
	"testing"

	v1 "github.com/brevdev/cloud/v1"
)

// TestAWSIntegration tests basic AWS provider functionality with real credentials
func TestAWSIntegration(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	region := "us-west-2"

	// Create credential using default chain
	credential := NewAWSCredential("integration-test", "", "", WithDefaultRegion(region))

	// Create client
	client, err := credential.MakeClient(ctx, region)
	if err != nil {
		t.Skipf("Failed to create AWS client (credentials may not be available): %v", err)
	}

	// Test 1: Get capabilities
	t.Run("GetCapabilities", func(t *testing.T) {
		capabilities, err := client.GetCapabilities(ctx)
		if err != nil {
			t.Fatalf("Failed to get capabilities: %v", err)
		}
		if len(capabilities) == 0 {
			t.Error("Expected non-zero capabilities")
		}
		t.Logf("AWS Provider supports %d capabilities", len(capabilities))
	})

	// Test 2: Get locations (should be fast)
	t.Run("GetLocations", func(t *testing.T) {
		locations, err := client.GetLocations(ctx, v1.GetLocationsArgs{})
		if err != nil {
			t.Fatalf("Failed to get locations: %v", err)
		}
		if len(locations) == 0 {
			t.Error("Expected non-zero locations")
		}
		t.Logf("Found %d AWS regions", len(locations))

		// Verify our current region is in the list
		foundRegion := false
		for _, loc := range locations {
			if loc.Name == region {
				foundRegion = true
				break
			}
		}
		if !foundRegion {
			t.Errorf("Expected to find region %s in location list", region)
		}
	})

	// Test 3: Get a few specific instance types (should be faster than all types)
	t.Run("GetInstanceTypes", func(t *testing.T) {
		instanceTypes, err := client.GetInstanceTypes(ctx, v1.GetInstanceTypeArgs{
			InstanceTypes: []string{"t3.micro", "t3.small"}, // Test with just 2 types
		})
		if err != nil {
			t.Fatalf("Failed to get instance types: %v", err)
		}
		if len(instanceTypes) == 0 {
			t.Error("Expected non-zero instance types")
		}
		t.Logf("Found %d instance types", len(instanceTypes))

		// Verify the instance types we requested are present
		for _, it := range instanceTypes {
			if it.Type != "t3.micro" && it.Type != "t3.small" {
				t.Errorf("Unexpected instance type: %s", it.Type)
			}
			if it.VCPU == 0 {
				t.Errorf("Instance type %s should have non-zero VCPU count", it.Type)
			}
			if it.Memory == 0 {
				t.Errorf("Instance type %s should have non-zero memory", it.Type)
			}
		}
	})

	// Test 4: Test tenant ID and other client properties
	t.Run("ClientProperties", func(t *testing.T) {
		if client.GetCloudProviderID() != CloudProviderID {
			t.Errorf("Expected provider ID %s, got %s", CloudProviderID, client.GetCloudProviderID())
		}

		if client.GetAPIType() != v1.APITypeLocational {
			t.Errorf("Expected API type %s, got %s", v1.APITypeLocational, client.GetAPIType())
		}

		tenantID, err := client.GetTenantID()
		if err != nil {
			t.Errorf("Failed to get tenant ID: %v", err)
		}
		if tenantID == "" {
			t.Error("Expected non-empty tenant ID")
		}
		t.Logf("Tenant ID: %s", tenantID)
	})
}