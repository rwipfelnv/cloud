package v1

import (
	"context"
	"fmt"
	"time"

	mk8sproto "github.com/nebius/gosdk/proto/nebius/mk8s/v1"
	mk8s "github.com/nebius/gosdk/services/nebius/mk8s/v1"

	v1_cloud "github.com/brevdev/cloud/v1"
)

// CreateCluster creates a Kubernetes cluster using Nebius mk8s
func (c *NebiusClient) CreateCluster(ctx context.Context, attrs v1_cloud.CreateClusterAttrs) (*v1_cloud.Cluster, error) {
	if c.sdk == nil {
		return nil, fmt.Errorf("Nebius SDK not initialized")
	}

	// Get mk8s cluster service
	mk8sServices := mk8s.New(c.sdk)
	clusterService := mk8sServices.Cluster()

	// Build create cluster request
	request, err := c.buildCreateClusterRequest(attrs)
	if err != nil {
		return nil, fmt.Errorf("failed to build create cluster request: %w", err)
	}

	// Create the cluster
	response, err := clusterService.Create(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	// Wait for creation to complete
	_, err = response.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for cluster creation: %w", err)
	}

	// TODO: Get the created cluster by polling the Get API once operation structure is confirmed
	// For now, return a placeholder cluster
	createdCluster := &mk8sproto.Cluster{}
	// createdCluster, err := clusterService.Get(ctx, &mk8sproto.GetClusterRequest{
	//     Id: "cluster-id", // TODO: extract from operation response
	// })
	// if err != nil {
	//     return nil, fmt.Errorf("failed to get created cluster: %w", err)
	// }

	// Convert to generic cluster format
	return c.convertNebiusClusterToGeneric(createdCluster, attrs.RefID, c.refID)
}

// GetCluster retrieves a Kubernetes cluster by ID
func (c *NebiusClient) GetCluster(ctx context.Context, id v1_cloud.CloudProviderClusterID) (*v1_cloud.Cluster, error) {
	if c.sdk == nil {
		return nil, fmt.Errorf("Nebius SDK not initialized")
	}

	// Get mk8s cluster service
	mk8sServices := mk8s.New(c.sdk)
	clusterService := mk8sServices.Cluster()

	// Get the cluster
	response, err := clusterService.Get(ctx, &mk8sproto.GetClusterRequest{
		Id: string(id),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Extract RefID from metadata tags if present
	refID := ""
	if response.GetMetadata().GetLabels() != nil {
		refID = response.GetMetadata().GetLabels()["RefID"]
	}

	// Convert to generic cluster format
	return c.convertNebiusClusterToGeneric(response, refID, c.refID)
}

// ListClusters lists Kubernetes clusters
func (c *NebiusClient) ListClusters(ctx context.Context, args v1_cloud.ListClustersArgs) ([]v1_cloud.Cluster, error) {
	if c.sdk == nil {
		return nil, fmt.Errorf("Nebius SDK not initialized")
	}

	// Get mk8s cluster service
	// mk8sServices := mk8s.New(c.sdk)
	// clusterService := mk8sServices.Cluster()

	var clusters []v1_cloud.Cluster

	// If specific cluster IDs are requested, get them individually
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

	// List all clusters in the project - TODO: implement once API structure is confirmed
	// response, err := clusterService.List(ctx, &mk8sproto.ListClustersRequest{
	//     ParentId: c.projectID,
	// })
	// if err != nil {
	//     return nil, fmt.Errorf("failed to list clusters: %w", err)
	// }

	// Convert each cluster - TODO: fix field name once structure is confirmed
	// for _, nebiusCluster := range response.GetClusters() {
	clusters = append(clusters, v1_cloud.Cluster{Name: "placeholder"}) // TODO: implement properly
	/*
		// Extract RefID from metadata tags if present
		refID := ""
		if nebiusCluster.GetMetadata().GetLabels() != nil {
			refID = nebiusCluster.GetMetadata().GetLabels()["RefID"]
		}

		// Convert to generic cluster format
		cluster, err := c.convertNebiusClusterToGeneric(nebiusCluster, refID, c.refID)
		if err != nil {
			// Continue with other clusters if one fails
			continue
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
	*/

	return clusters, nil
}

// DeleteCluster deletes a Kubernetes cluster
func (c *NebiusClient) DeleteCluster(ctx context.Context, id v1_cloud.CloudProviderClusterID) error {
	if c.sdk == nil {
		return fmt.Errorf("Nebius SDK not initialized")
	}

	// Get mk8s cluster service
	mk8sServices := mk8s.New(c.sdk)
	clusterService := mk8sServices.Cluster()

	// Delete the cluster
	response, err := clusterService.Delete(ctx, &mk8sproto.DeleteClusterRequest{
		Id: string(id),
	})
	if err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	// Wait for deletion to complete
	_, err = response.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for cluster deletion: %w", err)
	}

	return nil
}

// GetMaxCreateClusterRequestsPerMinute returns the maximum number of cluster creation requests per minute
func (c *NebiusClient) GetMaxCreateClusterRequestsPerMinute() int {
	// Conservative estimate for Nebius mk8s API rate limits
	return 5
}

// Helper functions that will be needed once the protobuf client is available

// convertNebiusClusterToGeneric converts a Nebius cluster to the generic cluster format
func (c *NebiusClient) convertNebiusClusterToGeneric(nebiusCluster *mk8sproto.Cluster, refID, cloudCredRefID string) (*v1_cloud.Cluster, error) {
	if nebiusCluster == nil {
		return nil, fmt.Errorf("Nebius cluster is nil")
	}

	// Convert Nebius status to generic status
	status := v1_cloud.ClusterStatus{
		LifecycleStatus: convertNebiusStatusToGeneric(nebiusCluster.GetStatus().GetState()),
		Messages:        []string{},
	}

	// Extract creation time from metadata
	var createdAt time.Time
	if nebiusCluster.GetMetadata().GetCreatedAt() != nil {
		createdAt = nebiusCluster.GetMetadata().GetCreatedAt().AsTime()
	}

	// Extract network configuration
	var serviceIPRange string
	// TODO: Fix field access once protobuf structure is confirmed
	// if nebiusCluster.GetSpec().GetNetworkConfig() != nil {
	//     serviceIPRange = nebiusCluster.GetSpec().GetNetworkConfig().GetServiceCidrs()[0]
	// }

	// Extract control plane endpoint
	var endpoint string
	// TODO: Fix endpoint field access once protobuf structure is confirmed
	// if nebiusCluster.GetStatus().GetControlPlane() != nil {
	//     endpoint = nebiusCluster.GetStatus().GetControlPlane().GetEndpoint()
	// }

	// Extract Kubernetes version
	var version string
	// TODO: Fix version field access once protobuf structure is confirmed
	// if nebiusCluster.GetSpec().GetControlPlane() != nil {
	//     version = nebiusCluster.GetSpec().GetControlPlane().GetVersion()
	// }

	// Convert labels to tags
	tags := make(v1_cloud.Tags)
	if nebiusCluster.GetMetadata().GetLabels() != nil {
		for k, v := range nebiusCluster.GetMetadata().GetLabels() {
			tags[k] = v
		}
	}

	// Extract subnet information
	var subnetIDs []string
	// TODO: Fix subnet field access once protobuf structure is confirmed
	// if nebiusCluster.GetSpec().GetControlPlane() != nil {
	//     subnetID := nebiusCluster.GetSpec().GetControlPlane().GetSubnetId()
	//     if subnetID != "" {
	//         subnetIDs = []string{subnetID}
	//     }
	// }

	cluster := &v1_cloud.Cluster{
		Name:               nebiusCluster.GetMetadata().GetName(),
		RefID:              refID,
		CloudCredRefID:     cloudCredRefID,
		CreatedAt:          createdAt,
		CloudID:            v1_cloud.CloudProviderClusterID(nebiusCluster.GetMetadata().GetId()),
		Status:             status,
		Version:            version,
		Endpoint:           endpoint,
		Location:           c.region,
		SubLocation:        "",
		Tags:               tags,
		NodeGroups:         []v1_cloud.NodeGroup{}, // Node groups are managed separately in Nebius
		VPCID:              "",                      // Nebius uses different networking model
		SubnetIDs:          subnetIDs,
		SecurityGroupIDs:   []string{},    // Nebius uses different security model
		ServiceIPRange:     serviceIPRange,
		PodIPRange:         "",    // Managed automatically by Nebius
		DNSClusterIP:       "",    // Managed automatically by Nebius
		PublicAPIAccess:    true,  // Inferred from endpoint availability
		PrivateAPIAccess:   false, // TODO: Determine from endpoint configuration
		AuthorizedNetworks: []string{},
		Addons:             []v1_cloud.ClusterAddon{}, // Addons are managed separately
	}

	return cluster, nil
}

// convertNebiusStatusToGeneric converts Nebius cluster status to generic cluster status
func convertNebiusStatusToGeneric(status mk8sproto.ClusterStatus_State) v1_cloud.ClusterLifecycleStatus {
	switch status {
	case mk8sproto.ClusterStatus_PROVISIONING:
		return v1_cloud.ClusterLifecycleStatusCreating
	case mk8sproto.ClusterStatus_RUNNING:
		return v1_cloud.ClusterLifecycleStatusActive
	case mk8sproto.ClusterStatus_DELETING:
		return v1_cloud.ClusterLifecycleStatusDeleting
	case mk8sproto.ClusterStatus_STATE_UNSPECIFIED:
		fallthrough
	default:
		return v1_cloud.ClusterLifecycleStatusFailed
	}
}

// buildCreateClusterRequest builds a Nebius mk8s CreateClusterRequest from generic attributes
func (c *NebiusClient) buildCreateClusterRequest(attrs v1_cloud.CreateClusterAttrs) (*mk8sproto.CreateClusterRequest, error) {
	// Build metadata with name and labels
	labels := make(map[string]string)
	if attrs.Tags != nil {
		for k, v := range attrs.Tags {
			labels[k] = v
		}
	}
	// Always add the Brev identification tag and RefID
	labels["CreatedBy"] = "brev-cloud-sdk"
	if attrs.RefID != "" {
		labels["RefID"] = attrs.RefID
	}

	// TODO: Build metadata once structure is confirmed
	// metadata := &mk8sproto.Metadata{
	//     Name:   attrs.Name,
	//     Labels: labels,
	// }

	// Build cluster spec - simplified for now until protobuf structure is confirmed
	clusterSpec := &mk8sproto.ClusterSpec{
		// TODO: Add proper field configuration once protobuf structure is confirmed
		// ControlPlane: &mk8sproto.ClusterSpec_ControlPlane{
		//     Version: attrs.Version,
		//     SubnetId: attrs.SubnetIDs[0], // if available
		//     EtcdClusterSize: 1,
		// },
		// NetworkConfig: &mk8sproto.ClusterSpec_NetworkConfig{
		//     ServiceCidrs: []string{attrs.ServiceIPRange},
		// },
	}

	// TODO: Build request once CreateClusterRequest structure is confirmed
	request := &mk8sproto.CreateClusterRequest{
		// ParentId: c.projectID, // TODO: confirm field name
		// Metadata: metadata,
		Spec: clusterSpec,
	}

	return request, nil
}

// Helper function to check if tags match filters (similar to AWS implementation)
func matchesTagFilters(tags v1_cloud.Tags, filters map[string][]string) bool {
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