package v1

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/nebius/gosdk"

	v1 "github.com/brevdev/cloud/v1"
)

const (
	CloudProviderID           = "nebius"
	defaultBackoffMaxInterval = 30 * time.Second
	defaultBackoffMaxTime     = 5 * time.Minute
)

// NebiusCredential implements the CloudCredential interface for Nebius
type NebiusCredential struct {
	RefID      string
	IAMToken   string  // IAM token for authentication
	ProjectID  string  // Nebius project ID
	Profile    *string // Profile name for SDK configuration
}

var _ v1.CloudCredential = &NebiusCredential{}

// NebiusCredentialOption allows customization of Nebius credentials
type NebiusCredentialOption func(*NebiusCredential)

// WithProfile sets the profile name
func WithProfile(profile string) NebiusCredentialOption {
	return func(c *NebiusCredential) {
		c.Profile = &profile
	}
}

// NewNebiusCredential creates a new Nebius credential
func NewNebiusCredential(refID, iamToken, projectID string, opts ...NebiusCredentialOption) *NebiusCredential {
	cred := &NebiusCredential{
		RefID:     refID,
		IAMToken:  iamToken,
		ProjectID: projectID,
	}

	for _, opt := range opts {
		opt(cred)
	}

	return cred
}

// GetReferenceID returns the reference ID for this credential
func (c *NebiusCredential) GetReferenceID() string {
	return c.RefID
}

// GetAPIType returns the API type for Nebius (hierarchical)
func (c *NebiusCredential) GetAPIType() v1.APIType {
	return v1.APITypeLocational
}

// GetCloudProviderID returns the cloud provider ID for Nebius
func (c *NebiusCredential) GetCloudProviderID() v1.CloudProviderID {
	return CloudProviderID
}

// GetTenantID returns the tenant ID for Nebius
func (c *NebiusCredential) GetTenantID() (string, error) {
	// Use project ID as tenant ID for Nebius
	if c.ProjectID != "" {
		return fmt.Sprintf("nebius-%s", c.ProjectID), nil
	}
	// Fallback to hash of IAM token
	hash := sha256.Sum256([]byte(c.IAMToken))
	return fmt.Sprintf("nebius-%x", hash[:8]), nil
}

// MakeClient creates a new Nebius client from this credential
func (c *NebiusCredential) MakeClient(ctx context.Context, region string) (v1.CloudClient, error) {
	return NewNebiusClient(c.RefID, c, region)
}

// NebiusClient implements the CloudClient interface for Nebius
// It embeds NotImplCloudClient to handle unsupported features
type NebiusClient struct {
	v1.NotImplCloudClient
	refID      string
	region     string
	projectID  string
	sdk        *gosdk.SDK
	credential *NebiusCredential
	backoff    backoff.BackOff
}

var _ v1.CloudClient = &NebiusClient{}

type nebiusClientOptions struct {
	backoff backoff.BackOff
}

type NebiusClientOption func(*nebiusClientOptions)

// WithBackoff sets the backoff policy for retry operations
func WithBackoff(bo backoff.BackOff) NebiusClientOption {
	return func(opts *nebiusClientOptions) {
		opts.backoff = bo
	}
}

// NewNebiusClient creates a new Nebius client
func NewNebiusClient(refID string, credential *NebiusCredential, region string, opts ...NebiusClientOption) (*NebiusClient, error) {
	if refID == "" {
		return nil, fmt.Errorf("refID is required")
	}
	if credential == nil {
		return nil, fmt.Errorf("credential is required")
	}
	if credential.IAMToken == "" {
		return nil, fmt.Errorf("IAM token is required")
	}
	if credential.ProjectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}

	// Process options
	var options nebiusClientOptions
	for _, opt := range opts {
		opt(&options)
	}

	// Set default backoff if not provided
	if options.backoff == nil {
		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = defaultBackoffMaxInterval
		bo.MaxElapsedTime = defaultBackoffMaxTime
		options.backoff = bo
	}

	// Create Nebius SDK client
	ctx := context.Background()
	sdk, err := gosdk.New(
		ctx,
		gosdk.WithCredentials(
			gosdk.IAMToken(credential.IAMToken),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nebius SDK: %w", err)
	}

	return &NebiusClient{
		refID:      refID,
		region:     region,
		projectID:  credential.ProjectID,
		sdk:        sdk,
		credential: credential,
		backoff:    options.backoff,
	}, nil
}

// GetAPIType returns the API type for Nebius
func (c *NebiusClient) GetAPIType() v1.APIType {
	return v1.APITypeLocational
}

// GetCloudProviderID returns the cloud provider ID for Nebius
func (c *NebiusClient) GetCloudProviderID() v1.CloudProviderID {
	return CloudProviderID
}

// GetReferenceID returns the reference ID for this client
func (c *NebiusClient) GetReferenceID() string {
	return c.refID
}

// GetTenantID returns the tenant ID for Nebius
func (c *NebiusClient) GetTenantID() (string, error) {
	return c.credential.GetTenantID()
}

// MakeClient creates a new client instance for the specified region
func (c *NebiusClient) MakeClient(ctx context.Context, region string) (v1.CloudClient, error) {
	if region == c.region {
		return c, nil
	}
	return NewNebiusClient(c.refID, c.credential, region, WithBackoff(c.backoff))
}

// GetMaxCreateRequestsPerMinute returns the maximum number of create requests per minute
func (c *NebiusClient) GetMaxCreateRequestsPerMinute() int {
	// Conservative estimate for Nebius API rate limits
	return 10
}
