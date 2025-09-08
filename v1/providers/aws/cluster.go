package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	v1 "github.com/brevdev/cloud/v1"
)

// CreateCluster creates an EKS cluster using hybrid IAM role approach
func (c *AWSClient) CreateCluster(ctx context.Context, attrs v1.CreateClusterAttrs) (*v1.Cluster, error) {
	if c.eksClient == nil {
		return nil, fmt.Errorf("EKS client not initialized")
	}

	// Handle cluster service role - use provided or create new
	clusterRoleArn := attrs.ClusterServiceRoleArn
	if clusterRoleArn == "" {
		var err error
		clusterRoleArn, err = c.ensureClusterServiceRole(ctx, attrs.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure cluster service role: %w", err)
		}
	}

	// Build EKS create cluster input
	input := &eks.CreateClusterInput{
		Name:    aws.String(attrs.Name),
		Version: aws.String(attrs.Version),
		RoleArn: aws.String(clusterRoleArn),
		ResourcesVpcConfig: &types.VpcConfigRequest{
			SubnetIds: attrs.SubnetIDs,
		},
	}

	// Set security groups if provided
	if len(attrs.SecurityGroupIDs) > 0 {
		input.ResourcesVpcConfig.SecurityGroupIds = attrs.SecurityGroupIDs
	}

	// Set API access configuration
	if attrs.PublicAPIAccess || attrs.PrivateAPIAccess {
		input.ResourcesVpcConfig.EndpointPublicAccess = &attrs.PublicAPIAccess
		input.ResourcesVpcConfig.EndpointPrivateAccess = &attrs.PrivateAPIAccess

		if len(attrs.AuthorizedNetworks) > 0 {
			input.ResourcesVpcConfig.PublicAccessCidrs = attrs.AuthorizedNetworks
		}
	}

	// Set logging configuration
	if len(attrs.LogTypes) > 0 {
		logSetup := &types.Logging{
			ClusterLogging: []types.LogSetup{
				{
					Enabled: aws.Bool(true),
					Types:  convertToEKSLogTypes(attrs.LogTypes),
				},
			},
		}
		input.Logging = logSetup
	}

	// Add tags - always include Brev identification tag (matching EC2 pattern)
	if input.Tags == nil {
		input.Tags = make(map[string]string)
	}
	if attrs.Tags != nil {
		for k, v := range attrs.Tags {
			input.Tags[k] = v
		}
	}
	// Always add the Brev created tag for identification and filtering (same as EC2)
	input.Tags["CreatedBy"] = "brev-cloud-sdk"
	if attrs.RefID != "" {
		input.Tags["RefID"] = attrs.RefID
	}

	// Create the cluster
	result, err := c.eksClient.CreateCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create EKS cluster: %w", err)
	}

	// Install essential add-ons for cluster functionality (like eksctl does)
	err = c.installEssentialAddons(ctx, attrs.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to install essential add-ons: %w", err)
	}

	// Convert EKS cluster to generic cluster (extract RefID from tags like EC2 does)
	refID := ""
	if result.Cluster.Tags != nil {
		refID = result.Cluster.Tags["RefID"]
	}
	cluster, err := c.convertEKSClusterToGeneric(result.Cluster, refID, c.refID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert EKS cluster to generic cluster: %w", err)
	}

	return cluster, nil
}

// GetCluster retrieves an EKS cluster by ID
func (c *AWSClient) GetCluster(ctx context.Context, id v1.CloudProviderClusterID) (*v1.Cluster, error) {
	if c.eksClient == nil {
		return nil, fmt.Errorf("EKS client not initialized")
	}

	result, err := c.eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(string(id)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe EKS cluster: %w", err)
	}

	// Convert EKS cluster to generic cluster (extract RefID from tags like EC2 does)
	refID := ""
	if result.Cluster.Tags != nil {
		refID = result.Cluster.Tags["RefID"]
	}
	cluster, err := c.convertEKSClusterToGeneric(result.Cluster, refID, c.refID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert EKS cluster to generic cluster: %w", err)
	}

	return cluster, nil
}

// ListClusters lists EKS clusters
func (c *AWSClient) ListClusters(ctx context.Context, args v1.ListClustersArgs) ([]v1.Cluster, error) {
	if c.eksClient == nil {
		return nil, fmt.Errorf("EKS client not initialized")
	}

	var clusters []v1.Cluster

	// If specific cluster IDs are requested, get them individually (no filtering)
	if len(args.ClusterIDs) > 0 {
		for _, id := range args.ClusterIDs {
			cluster, err := c.GetCluster(ctx, id)
			if err != nil {
				// Continue with other clusters if one fails
				continue
			}
			clusters = append(clusters, *cluster)
		}
		return clusters, nil
	}

	// List all clusters
	input := &eks.ListClustersInput{}
	
	paginator := eks.NewListClustersPaginator(c.eksClient, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
		}

		// Get details for each cluster
		for _, clusterName := range page.Clusters {
			cluster, err := c.GetCluster(ctx, v1.CloudProviderClusterID(clusterName))
			if err != nil {
				// Continue with other clusters if one fails (same pattern as EC2)
				continue
			}

			// Only return clusters created by Brev (unless specific tag filters override this) - same as EC2 pattern
			if len(args.TagFilters) == 0 {
				if cluster.Tags["CreatedBy"] != "brev-cloud-sdk" {
					continue // Skip clusters not created by Brev
				}
			}

			// Apply location filter if specified
			if len(args.Locations) > 0 {
				match := false
				for _, location := range args.Locations {
					if cluster.Location == location {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}

			// Apply tag filters if specified
			if len(args.TagFilters) > 0 {
				if !matchesTagFilters(cluster.Tags, args.TagFilters) {
					continue
				}
			}

			clusters = append(clusters, *cluster)
		}
	}

	return clusters, nil
}

// DeleteCluster deletes an EKS cluster and cleans up associated resources
func (c *AWSClient) DeleteCluster(ctx context.Context, id v1.CloudProviderClusterID) error {
	if c.eksClient == nil {
		return fmt.Errorf("EKS client not initialized")
	}

	clusterName := string(id)

	// Delete the EKS cluster (add-ons will be automatically deleted)
	_, err := c.eksClient.DeleteCluster(ctx, &eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %w", err)
	}

	// Clean up the IAM cluster service role we created (if it exists)
	err = c.cleanupClusterServiceRole(ctx, clusterName)
	if err != nil {
		// Log the error but don't fail the entire deletion
		// The cluster is already being deleted
		fmt.Printf("Warning: failed to cleanup cluster service role: %v\n", err)
	}

	return nil
}

// GetMaxCreateClusterRequestsPerMinute returns the maximum number of cluster creation requests per minute
func (c *AWSClient) GetMaxCreateClusterRequestsPerMinute() int {
	// EKS has lower limits for cluster operations
	return 2
}

// ensureClusterServiceRole creates or retrieves the EKS cluster service role
func (c *AWSClient) ensureClusterServiceRole(ctx context.Context, clusterName string) (string, error) {
	roleName := fmt.Sprintf("BrevEKSClusterServiceRole-%s", clusterName)
	
	// Check if role already exists
	getRoleResult, err := c.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err == nil {
		return aws.ToString(getRoleResult.Role.Arn), nil
	}

	// Create the role if it doesn't exist
	trustPolicy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect": "Allow",
				"Principal": map[string]string{
					"Service": "eks.amazonaws.com",
				},
				"Action": "sts:AssumeRole",
			},
		},
	}

	trustPolicyJSON, err := json.Marshal(trustPolicy)
	if err != nil {
		return "", fmt.Errorf("failed to marshal trust policy: %w", err)
	}

	createRoleInput := &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(string(trustPolicyJSON)),
		Description:              aws.String("EKS cluster service role created by Brev SDK"),
		Tags: []iamTypes.Tag{
			{
				Key:   aws.String("CreatedBy"),
				Value: aws.String("brev-cloud-sdk"),
			},
			{
				Key:   aws.String("Cluster"),
				Value: aws.String(clusterName),
			},
		},
	}

	createRoleResult, err := c.iamClient.CreateRole(ctx, createRoleInput)
	if err != nil {
		return "", fmt.Errorf("failed to create cluster service role: %w", err)
	}

	// Attach the required policy
	_, err = c.iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to attach cluster policy: %w", err)
	}

	return aws.ToString(createRoleResult.Role.Arn), nil
}

// cleanupClusterServiceRole removes the IAM cluster service role created during cluster setup
func (c *AWSClient) cleanupClusterServiceRole(ctx context.Context, clusterName string) error {
	roleName := fmt.Sprintf("BrevEKSClusterServiceRole-%s", clusterName)
	
	// Check if role exists
	_, err := c.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		// Role doesn't exist, nothing to clean up
		return nil
	}

	// Detach the policy before deleting the role
	_, err = c.iamClient.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"),
	})
	if err != nil {
		return fmt.Errorf("failed to detach cluster policy from role %s: %w", roleName, err)
	}

	// Delete the role
	_, err = c.iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete cluster service role %s: %w", roleName, err)
	}

	return nil
}

// convertEKSClusterToGeneric converts an EKS cluster to the generic cluster format
func (c *AWSClient) convertEKSClusterToGeneric(eksCluster *types.Cluster, refID, cloudCredRefID string) (*v1.Cluster, error) {
	if eksCluster == nil {
		return nil, fmt.Errorf("EKS cluster is nil")
	}

	// Convert EKS status to generic status
	status := v1.ClusterStatus{
		LifecycleStatus: convertEKSStatusToGeneric(eksCluster.Status),
		Messages:        []string{},
	}

	// Extract creation time
	var createdAt time.Time
	if eksCluster.CreatedAt != nil {
		createdAt = *eksCluster.CreatedAt
	}

	// Extract VPC information
	var vpcID string
	var subnetIDs []string
	var securityGroupIDs []string
	if eksCluster.ResourcesVpcConfig != nil {
		vpcID = aws.ToString(eksCluster.ResourcesVpcConfig.VpcId)
		subnetIDs = eksCluster.ResourcesVpcConfig.SubnetIds
		securityGroupIDs = eksCluster.ResourcesVpcConfig.SecurityGroupIds
	}

	// Extract API access configuration
	var publicAPIAccess, privateAPIAccess bool
	var authorizedNetworks []string
	if eksCluster.ResourcesVpcConfig != nil {
		publicAPIAccess = eksCluster.ResourcesVpcConfig.EndpointPublicAccess
		privateAPIAccess = eksCluster.ResourcesVpcConfig.EndpointPrivateAccess
		authorizedNetworks = eksCluster.ResourcesVpcConfig.PublicAccessCidrs
	}

	// Extract service CIDR
	var serviceIPRange string
	if eksCluster.KubernetesNetworkConfig != nil && eksCluster.KubernetesNetworkConfig.ServiceIpv4Cidr != nil {
		serviceIPRange = *eksCluster.KubernetesNetworkConfig.ServiceIpv4Cidr
	}

	// Extract tags (EKS tags are already map[string]string = v1.Tags)
	tags := eksCluster.Tags

	cluster := &v1.Cluster{
		Name:               aws.ToString(eksCluster.Name),
		RefID:              refID,
		CloudCredRefID:     cloudCredRefID,
		CreatedAt:          createdAt,
		CloudID:            v1.CloudProviderClusterID(aws.ToString(eksCluster.Name)),
		Status:             status,
		Version:            aws.ToString(eksCluster.Version),
		Endpoint:           aws.ToString(eksCluster.Endpoint),
		Location:           c.region,
		SubLocation:        "",
		Tags:               tags,
		NodeGroups:         []v1.NodeGroup{}, // Node groups are managed separately in EKS
		VPCID:              vpcID,
		SubnetIDs:          subnetIDs,
		SecurityGroupIDs:   securityGroupIDs,
		ServiceIPRange:     serviceIPRange,
		PodIPRange:         "", // EKS manages this automatically
		DNSClusterIP:       "", // EKS manages this automatically
		PublicAPIAccess:    publicAPIAccess,
		PrivateAPIAccess:   privateAPIAccess,
		AuthorizedNetworks: authorizedNetworks,
		Addons:             []v1.ClusterAddon{}, // Addons are managed separately in EKS
	}

	return cluster, nil
}

// convertEKSStatusToGeneric converts EKS cluster status to generic cluster status
func convertEKSStatusToGeneric(status types.ClusterStatus) v1.ClusterLifecycleStatus {
	switch status {
	case types.ClusterStatusCreating:
		return v1.ClusterLifecycleStatusCreating
	case types.ClusterStatusActive:
		return v1.ClusterLifecycleStatusActive
	case types.ClusterStatusUpdating:
		return v1.ClusterLifecycleStatusUpdating
	case types.ClusterStatusDeleting:
		return v1.ClusterLifecycleStatusDeleting
	case types.ClusterStatusFailed:
		return v1.ClusterLifecycleStatusFailed
	default:
		return v1.ClusterLifecycleStatusFailed
	}
}

// convertToEKSLogTypes converts generic log types to EKS log types
func convertToEKSLogTypes(logTypes []string) []types.LogType {
	var eksLogTypes []types.LogType
	for _, logType := range logTypes {
		switch strings.ToLower(logType) {
		case "api":
			eksLogTypes = append(eksLogTypes, types.LogTypeApi)
		case "audit":
			eksLogTypes = append(eksLogTypes, types.LogTypeAudit)
		case "authenticator":
			eksLogTypes = append(eksLogTypes, types.LogTypeAuthenticator)
		case "controllermanager":
			eksLogTypes = append(eksLogTypes, types.LogTypeControllerManager)
		case "scheduler":
			eksLogTypes = append(eksLogTypes, types.LogTypeScheduler)
		}
	}
	return eksLogTypes
}

// Helper function to check if tags match filters
func matchesTagFilters(tags v1.Tags, filters map[string][]string) bool {
	for key, allowedValues := range filters {
		tagValue, exists := tags[key]
		if !exists {
			return false
		}

		valueMatch := false
		for _, allowedValue := range allowedValues {
			if tagValue == allowedValue {
				valueMatch = true
				break
			}
		}
		if !valueMatch {
			return false
		}
	}
	return true
}

// installEssentialAddons installs the essential EKS add-ons for cluster functionality
func (c *AWSClient) installEssentialAddons(ctx context.Context, clusterName string) error {
	// Essential add-ons required for basic EKS functionality (same as eksctl)
	addons := []struct {
		name        string
		description string
	}{
		{"vpc-cni", "VPC CNI for pod networking"},
		{"coredns", "CoreDNS for DNS resolution"},
		{"kube-proxy", "kube-proxy for service networking"},
	}

	for _, addon := range addons {
		// Check if addon already exists
		_, err := c.eksClient.DescribeAddon(ctx, &eks.DescribeAddonInput{
			ClusterName: aws.String(clusterName),
			AddonName:   aws.String(addon.name),
		})
		if err == nil {
			// Addon already exists, skip
			continue
		}

		// Create the addon
		_, err = c.eksClient.CreateAddon(ctx, &eks.CreateAddonInput{
			ClusterName: aws.String(clusterName),
			AddonName:   aws.String(addon.name),
			Tags: map[string]string{
				"CreatedBy": "brev-cloud-sdk",
			},
		})
		if err != nil {
			return fmt.Errorf("failed to install %s addon (%s): %w", addon.name, addon.description, err)
		}
	}

	return nil
}