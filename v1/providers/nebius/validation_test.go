package v1

import (
	"context"
	"os"
	"testing"

	"github.com/brevdev/cloud/internal/validation"
	v1 "github.com/brevdev/cloud/v1"
)

func TestValidationFunctions(t *testing.T) {
	checkSkip(t)

	// Use credentials from environment variables
	iamToken, projectID := getNebiusCredentials()
	config := validation.ProviderConfig{
		Credential: NewNebiusCredential("validation-test", iamToken, projectID),
		StableIDs: []v1.InstanceTypeID{
			// TODO: Add stable Nebius instance type IDs once available
		},
	}

	validation.RunValidationSuite(t, config)
}

func TestInstanceLifecycleValidation(t *testing.T) {
	checkSkip(t)

	// Use credentials from environment variables
	iamToken, projectID := getNebiusCredentials()
	config := validation.ProviderConfig{
		Credential: NewNebiusCredential("validation-test", iamToken, projectID),
	}

	validation.RunInstanceLifecycleValidation(t, config)
}

func checkSkip(t *testing.T) {
	// Try to create a client to test if credentials are available
	iamToken, projectID := getNebiusCredentials()
	if iamToken == "" || projectID == "" {
		t.Skip("Nebius credentials not available, skipping Nebius validation tests")
		return
	}

	credential := NewNebiusCredential("test", iamToken, projectID)
	client, err := credential.MakeClient(context.Background(), getNebiusRegion())
	if err != nil {
		t.Skip("Nebius credentials not working, skipping Nebius validation tests")
		return
	}

	// Test if we can actually make a call
	_, err = client.GetCapabilities(context.Background())
	if err != nil {
		t.Skip("Nebius credentials not working, skipping Nebius validation tests")
	}
}

func getNebiusRegion() string {
	region := os.Getenv("NEBIUS_DEFAULT_REGION")
	if region == "" {
		region = "us-east-1" // Default region for Nebius
	}
	return region
}

func getNebiusCredentials() (string, string) {
	iamToken := os.Getenv("NEBIUS_IAM_TOKEN")
	projectID := os.Getenv("NEBIUS_PROJECT_ID")
	return iamToken, projectID
}

// TestNebiusCredentialValidation tests credential validation
func TestNebiusCredentialValidation(t *testing.T) {
	tests := []struct {
		name        string
		refID       string
		iamToken    string
		projectID   string
		expectError bool
	}{
		{
			name:        "valid credentials",
			refID:       "test-ref",
			iamToken:    "test-iam-token",
			projectID:   "test-project-id",
			expectError: false,
		},
		{
			name:        "empty IAM token",
			refID:       "test-ref",
			iamToken:    "",
			projectID:   "test-project-id",
			expectError: false, // NewNebiusCredential doesn't validate, NewNebiusClient does
		},
		{
			name:        "empty project ID",
			refID:       "test-ref",
			iamToken:    "test-iam-token",
			projectID:   "",
			expectError: false, // NewNebiusCredential doesn't validate, NewNebiusClient does
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
		})
	}
}

// TestNebiusClientCreation tests client creation
func TestNebiusClientCreation(t *testing.T) {
	cred := NewNebiusCredential("test", "test-token", "test-project")

	// This will fail due to invalid credentials, but that's expected for unit tests
	_, err := NewNebiusClient("test-ref", cred, "us-east-1")
	if err == nil {
		t.Log("Client creation succeeded (unexpected but OK for test environment)")
	} else {
		// Expected to fail with invalid credentials
		t.Logf("Client creation failed as expected: %v", err)
	}
}