package v1

import (
	"os"
	"testing"

	"github.com/brevdev/cloud/internal/validation"
	v1 "github.com/brevdev/cloud/v1"
)

func TestValidationFunctions(t *testing.T) {
	checkSkip(t)
	accessKeyID, secretAccessKey, region := getAWSCredentials()

	config := validation.ProviderConfig{
		Credential: NewAWSCredential("validation-test", accessKeyID, secretAccessKey,
			WithDefaultRegion(region)),
		StableIDs: []v1.InstanceTypeID{}, // AWS doesn't have predefined stable IDs
	}

	validation.RunValidationSuite(t, config)
}

func TestInstanceLifecycleValidation(t *testing.T) {
	checkSkip(t)
	accessKeyID, secretAccessKey, region := getAWSCredentials()

	config := validation.ProviderConfig{
		Credential: NewAWSCredential("validation-test", accessKeyID, secretAccessKey,
			WithDefaultRegion(region)),
	}

	validation.RunInstanceLifecycleValidation(t, config)
}

func checkSkip(t *testing.T) {
	accessKeyID, secretAccessKey, _ := getAWSCredentials()
	isValidationTest := os.Getenv("VALIDATION_TEST")
	if (accessKeyID == "" || secretAccessKey == "") && isValidationTest != "" {
		t.Fatal("AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY not set, but VALIDATION_TEST is set")
	} else if (accessKeyID == "" || secretAccessKey == "") && isValidationTest == "" {
		t.Skip("AWS credentials not set, skipping AWS validation tests")
	}
}

func getAWSCredentials() (string, string, string) {
	accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	region := os.Getenv("AWS_DEFAULT_REGION")
	
	if region == "" {
		region = "us-east-1"
	}
	
	return accessKeyID, secretAccessKey, region
}

// TestAWSCredentialValidation tests credential validation
func TestAWSCredentialValidation(t *testing.T) {
	tests := []struct {
		name            string
		refID           string
		accessKeyID     string
		secretAccessKey string
		expectError     bool
	}{
		{
			name:            "valid credentials",
			refID:           "test-ref",
			accessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			secretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			expectError:     false,
		},
		{
			name:            "empty access key",
			refID:           "test-ref",
			accessKeyID:     "",
			secretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			expectError:     false, // NewAWSCredential doesn't validate, NewAWSClient does
		},
		{
			name:            "empty secret key",
			refID:           "test-ref",
			accessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			secretAccessKey: "",
			expectError:     false, // NewAWSCredential doesn't validate, NewAWSClient does
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := NewAWSCredential(tt.refID, tt.accessKeyID, tt.secretAccessKey)
			
			if cred == nil {
				t.Fatal("Expected credential to be created")
			}
			
			if cred.GetReferenceID() != tt.refID {
				t.Errorf("Expected RefID %s, got %s", tt.refID, cred.GetReferenceID())
			}
		})
	}
}

// TestAWSClientCreation tests client creation
func TestAWSClientCreation(t *testing.T) {
	cred := NewAWSCredential("test", "test-key", "test-secret")
	
	client, err := NewAWSClient("test-ref", cred, "us-east-1")
	if err != nil {
		t.Fatalf("Failed to create AWS client: %v", err)
	}
	
	if client.GetCloudProviderID() != CloudProviderID {
		t.Errorf("Expected provider ID %s, got %s", CloudProviderID, client.GetCloudProviderID())
	}
	
	if client.GetReferenceID() != "test-ref" {
		t.Errorf("Expected reference ID 'test-ref', got %s", client.GetReferenceID())
	}
}