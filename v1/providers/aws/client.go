package v1

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/cenkalti/backoff/v4"

	v1 "github.com/brevdev/cloud/v1"
)

const (
	CloudProviderID           = "aws"
	DefaultRegion             = "us-east-1"
	defaultBackoffMaxInterval = 30 * time.Second
	defaultBackoffMaxTime     = 5 * time.Minute
)

// AWSCredential implements the CloudCredential interface for AWS
type AWSCredential struct {
	RefID           string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    *string // For temporary credentials
	Region          *string // Optional default region
	Profile         *string // AWS profile name
}

var _ v1.CloudCredential = &AWSCredential{}

// AWSCredentialOption allows customization of AWS credentials
type AWSCredentialOption func(*AWSCredential)

// WithSessionToken sets the session token for temporary credentials
func WithSessionToken(token string) AWSCredentialOption {
	return func(c *AWSCredential) {
		c.SessionToken = &token
	}
}

// WithDefaultRegion sets the default region
func WithDefaultRegion(region string) AWSCredentialOption {
	return func(c *AWSCredential) {
		c.Region = &region
	}
}

// WithProfile sets the AWS profile name
func WithProfile(profile string) AWSCredentialOption {
	return func(c *AWSCredential) {
		c.Profile = &profile
	}
}

// NewAWSCredential creates a new AWS credential
func NewAWSCredential(refID, accessKeyID, secretAccessKey string, opts ...AWSCredentialOption) *AWSCredential {
	cred := &AWSCredential{
		RefID:           refID,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}

	for _, opt := range opts {
		opt(cred)
	}

	return cred
}

// GetReferenceID returns the reference ID for this credential
func (c *AWSCredential) GetReferenceID() string {
	return c.RefID
}

// GetAPIType returns the API type for AWS (locational)
func (c *AWSCredential) GetAPIType() v1.APIType {
	return v1.APITypeLocational
}

// GetCloudProviderID returns the cloud provider ID for AWS
func (c *AWSCredential) GetCloudProviderID() v1.CloudProviderID {
	return CloudProviderID
}

// GetTenantID returns the tenant ID for AWS
func (c *AWSCredential) GetTenantID() (string, error) {
	// Use access key ID hash for tenant ID
	hash := sha256.Sum256([]byte(c.AccessKeyID))
	return fmt.Sprintf("aws-%x", hash[:8]), nil
}

// MakeClient creates a new AWS client from this credential
func (c *AWSCredential) MakeClient(ctx context.Context, region string) (v1.CloudClient, error) {
	return NewAWSClient(c.RefID, c, region)
}

// AWSClient implements the CloudClient interface for AWS
// It embeds NotImplCloudClient to handle unsupported features
type AWSClient struct {
	v1.NotImplCloudClient
	refID      string
	region     string
	awsConfig  aws.Config
	ec2Client  *ec2.Client
	eksClient  *eks.Client
	iamClient  *iam.Client
	ssmClient  *ssm.Client
	credential *AWSCredential
	backoff    backoff.BackOff
}

var _ v1.CloudClient = &AWSClient{}

type awsClientOptions struct {
	backoff backoff.BackOff
}

type AWSClientOption func(*awsClientOptions)

// WithBackoff sets the backoff policy for retry operations
func WithBackoff(bo backoff.BackOff) AWSClientOption {
	return func(opts *awsClientOptions) {
		opts.backoff = bo
	}
}

// NewAWSClient creates a new AWS client
func NewAWSClient(refID string, credential *AWSCredential, region string, opts ...AWSClientOption) (*AWSClient, error) {
	if refID == "" {
		return nil, fmt.Errorf("refID is required")
	}
	if credential == nil {
		return nil, fmt.Errorf("credential is required")
	}

	// Set default region if not provided
	if region == "" {
		if credential.Region != nil {
			region = *credential.Region
		} else {
			region = DefaultRegion
		}
	}

	// Process options
	var options awsClientOptions
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

	// Configure AWS SDK
	var cfg aws.Config
	var err error

	if credential.Profile != nil {
		// Use AWS profile
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithSharedConfigProfile(*credential.Profile),
		)
	} else if credential.AccessKeyID != "" && credential.SecretAccessKey != "" {
		// Use explicit credentials only if both are provided
		creds := credentials.NewStaticCredentialsProvider(
			credential.AccessKeyID,
			credential.SecretAccessKey,
			aws.ToString(credential.SessionToken),
		)

		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithCredentialsProvider(creds),
		)
	} else {
		// Use default credential chain (environment variables, ~/.aws/credentials, IAM roles, etc.)
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AWSClient{
		refID:      refID,
		region:     region,
		awsConfig:  cfg,
		ec2Client:  ec2.NewFromConfig(cfg),
		eksClient:  eks.NewFromConfig(cfg),
		iamClient:  iam.NewFromConfig(cfg),
		ssmClient:  ssm.NewFromConfig(cfg),
		credential: credential,
		backoff:    options.backoff,
	}, nil
}

// GetAPIType returns the API type for AWS
func (c *AWSClient) GetAPIType() v1.APIType {
	return v1.APITypeLocational
}

// GetCloudProviderID returns the cloud provider ID for AWS
func (c *AWSClient) GetCloudProviderID() v1.CloudProviderID {
	return CloudProviderID
}

// GetReferenceID returns the reference ID for this client
func (c *AWSClient) GetReferenceID() string {
	return c.refID
}

// GetTenantID returns the tenant ID for AWS
func (c *AWSClient) GetTenantID() (string, error) {
	return c.credential.GetTenantID()
}

// MakeClient creates a new client instance for the specified region
func (c *AWSClient) MakeClient(ctx context.Context, region string) (v1.CloudClient, error) {
	if region == c.region {
		return c, nil
	}
	return NewAWSClient(c.refID, c.credential, region, WithBackoff(c.backoff))
}

// GetMaxCreateRequestsPerMinute returns the maximum number of create requests per minute
func (c *AWSClient) GetMaxCreateRequestsPerMinute() int {
	// AWS EC2 has rate limits but they're complex and vary by operation
	// Conservative estimate for RunInstances API
	return 20
}