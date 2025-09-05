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
	region := getAWSRegion()

	// Use default credential chain (will pick up from ~/.aws/credentials)
	config := validation.ProviderConfig{
		Credential: NewAWSCredential("validation-test", "", "", // Empty credentials use default chain
			WithDefaultRegion(region)),
		StableIDs: []v1.InstanceTypeID{
			v1.InstanceTypeID(region + "-default-t3.micro"),
			v1.InstanceTypeID(region + "-default-t3.small"), 
			v1.InstanceTypeID(region + "-default-t3.medium"),
		}, // Some common stable instance types
	}

	validation.RunValidationSuite(t, config)
}

func TestInstanceLifecycleValidation(t *testing.T) {
	checkSkip(t)
	region := getAWSRegion()

	// Use default credential chain (will pick up from ~/.aws/credentials)  
	config := validation.ProviderConfig{
		Credential: NewAWSCredential("validation-test", "", "", // Empty credentials use default chain
			WithDefaultRegion(region)),
	}

	validation.RunInstanceLifecycleValidation(t, config)
}

func checkSkip(t *testing.T) {
	// Try to create a client to test if credentials are available
	region := getAWSRegion()
	credential := NewAWSCredential("test", "", "", WithDefaultRegion(region))
	client, err := credential.MakeClient(context.Background(), region)
	if err != nil {
		t.Skip("AWS credentials not available, skipping AWS validation tests")
		return
	}
	
	// Test if we can actually make a call
	_, err = client.GetCapabilities(context.Background())
	if err != nil {
		t.Skip("AWS credentials not working, skipping AWS validation tests")
	}
}

func getAWSRegion() string {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = "us-west-2" // Use us-west-2 since that's what we detected
	}
	return region
}

func getAWSCredentials() (string, string, string) {
	accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	region := getAWSRegion()
	
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