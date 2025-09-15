package v1

import (
	"context"
	"testing"

	v1 "github.com/brevdev/cloud/v1"
)

// TestNebiusCredentialCreation tests credential creation without requiring real credentials
func TestNebiusCredentialCreation(t *testing.T) {
	tests := []struct {
		name      string
		refID     string
		iamToken  string
		projectID string
	}{
		{
			name:      "valid parameters",
			refID:     "test-ref",
			iamToken:  "test-token",
			projectID: "test-project",
		},
		{
			name:      "empty token",
			refID:     "test-ref",
			iamToken:  "",
			projectID: "test-project",
		},
		{
			name:      "empty project",
			refID:     "test-ref",
			iamToken:  "test-token",
			projectID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := NewNebiusCredential(tt.refID, tt.iamToken, tt.projectID)

			if cred == nil {
				t.Fatal("Expected credential to be created")
			}

			if cred.GetReferenceID() != tt.refID {
				t.Errorf("Expected RefID %s, got %s", tt.refID, cred.GetReferenceID())
			}

			if cred.GetCloudProviderID() != CloudProviderID {
				t.Errorf("Expected provider ID %s, got %s", CloudProviderID, cred.GetCloudProviderID())
			}

			if cred.GetAPIType() != v1.APITypeLocational {
				t.Errorf("Expected API type %s, got %s", v1.APITypeLocational, cred.GetAPIType())
			}
		})
	}
}

// TestNebiusCredentialTenantID tests tenant ID generation
func TestNebiusCredentialTenantID(t *testing.T) {
	cred := NewNebiusCredential("test", "token", "project-123")

	tenantID, err := cred.GetTenantID()
	if err != nil {
		t.Fatalf("Failed to get tenant ID: %v", err)
	}

	expectedTenantID := "nebius-project-123"
	if tenantID != expectedTenantID {
		t.Errorf("Expected tenant ID %s, got %s", expectedTenantID, tenantID)
	}
}

// TestNebiusCredentialTenantIDFallback tests tenant ID fallback for empty project ID
func TestNebiusCredentialTenantIDFallback(t *testing.T) {
	cred := NewNebiusCredential("test", "test-token", "")

	tenantID, err := cred.GetTenantID()
	if err != nil {
		t.Fatalf("Failed to get tenant ID: %v", err)
	}

	// Should start with "nebius-" followed by token hash
	if len(tenantID) == 0 || tenantID[:7] != "nebius-" {
		t.Errorf("Expected tenant ID to start with 'nebius-', got %s", tenantID)
	}
}

// TestNebiusClientCreationValidation tests client creation validation without actual SDK connection
func TestNebiusClientCreationValidation(t *testing.T) {
	tests := []struct {
		name        string
		refID       string
		credential  *NebiusCredential
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing refID",
			refID:       "",
			credential:  NewNebiusCredential("test", "token", "project"),
			expectError: true,
			errorMsg:    "refID is required",
		},
		{
			name:        "nil credential",
			refID:       "test",
			credential:  nil,
			expectError: true,
			errorMsg:    "credential is required",
		},
		{
			name:        "empty IAM token",
			refID:       "test",
			credential:  NewNebiusCredential("test", "", "project"),
			expectError: true,
			errorMsg:    "IAM token is required",
		},
		{
			name:        "empty project ID",
			refID:       "test",
			credential:  NewNebiusCredential("test", "token", ""),
			expectError: true,
			errorMsg:    "project ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewNebiusClient(tt.refID, tt.credential, "us-east-1")

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestClusterInterfaceImplementation tests that the client implements the CloudClusterManager interface
func TestClusterInterfaceImplementation(t *testing.T) {
	// This test ensures the NebiusClient implements the CloudClusterManager interface
	// without requiring actual API calls

	cred := NewNebiusCredential("test", "fake-token", "fake-project")

	// This should fail due to SDK initialization, but that's expected
	// We're just testing that the interface is properly implemented
	client, err := NewNebiusClient("test", cred, "us-east-1")
	if err != nil {
		// Expected to fail without real SDK connection
		// But the type should still be correct for interface checking
		var _ v1.CloudClusterManager = &NebiusClient{}
		return
	}

	// If somehow it succeeds (mock or test environment), verify interface
	var _ v1.CloudClusterManager = client

	// Verify basic client properties
	if client.GetCloudProviderID() != CloudProviderID {
		t.Errorf("Expected provider ID %s, got %s", CloudProviderID, client.GetCloudProviderID())
	}

	if client.GetReferenceID() != "test" {
		t.Errorf("Expected reference ID 'test', got %s", client.GetReferenceID())
	}

	if client.GetAPIType() != v1.APITypeLocational {
		t.Errorf("Expected API type %s, got %s", v1.APITypeLocational, client.GetAPIType())
	}
}

// TestGetMaxCreateClusterRequestsPerMinute tests the rate limit method
func TestGetMaxCreateClusterRequestsPerMinute(t *testing.T) {
	client := &NebiusClient{}

	maxRequests := client.GetMaxCreateClusterRequestsPerMinute()
	if maxRequests <= 0 {
		t.Errorf("Expected positive number for max requests per minute, got %d", maxRequests)
	}

	// Should be a reasonable conservative value
	if maxRequests > 100 {
		t.Errorf("Expected conservative rate limit, got %d", maxRequests)
	}
}

// TestClusterOperationsWithoutConnection tests that cluster operations fail gracefully without connection
func TestClusterOperationsWithoutConnection(t *testing.T) {
	// Create client with nil SDK (simulating no connection)
	client := &NebiusClient{
		refID:     "test",
		region:    "us-east-1",
		projectID: "test-project",
		sdk:       nil, // No SDK connection
	}

	ctx := context.Background()

	// Test CreateCluster
	_, err := client.CreateCluster(ctx, v1.CreateClusterAttrs{
		Name:     "test-cluster",
		RefID:    "test-ref",
		Location: "us-east-1",
		Version:  "1.28",
	})
	if err == nil || err.Error() != "Nebius SDK not initialized" {
		t.Errorf("Expected 'Nebius SDK not initialized' error, got: %v", err)
	}

	// Test GetCluster
	_, err = client.GetCluster(ctx, "test-cluster-id")
	if err == nil || err.Error() != "Nebius SDK not initialized" {
		t.Errorf("Expected 'Nebius SDK not initialized' error, got: %v", err)
	}

	// Test ListClusters
	_, err = client.ListClusters(ctx, v1.ListClustersArgs{})
	if err == nil || err.Error() != "Nebius SDK not initialized" {
		t.Errorf("Expected 'Nebius SDK not initialized' error, got: %v", err)
	}

	// Test DeleteCluster
	err = client.DeleteCluster(ctx, "test-cluster-id")
	if err == nil || err.Error() != "Nebius SDK not initialized" {
		t.Errorf("Expected 'Nebius SDK not initialized' error, got: %v", err)
	}
}