package v1

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CloudClusterManager defines the interface for managing Kubernetes clusters
type CloudClusterManager interface {
	CreateCluster(ctx context.Context, attrs CreateClusterAttrs) (*Cluster, error)
	GetCluster(ctx context.Context, id CloudProviderClusterID) (*Cluster, error)
	ListClusters(ctx context.Context, args ListClustersArgs) ([]Cluster, error)
	DeleteCluster(ctx context.Context, id CloudProviderClusterID) error
	GetMaxCreateClusterRequestsPerMinute() int
}

// Cluster represents a managed Kubernetes cluster
type Cluster struct {
	Name               string
	RefID              string
	CloudCredRefID     string // cloudCred used to create the cluster
	CreatedAt          time.Time
	CloudID            CloudProviderClusterID
	Status             ClusterStatus
	Version            string // Kubernetes version
	Endpoint           string // API server endpoint
	Location           string
	SubLocation        string // availability zone or specific location within region
	Tags               Tags
	NodeGroups         []NodeGroup
	VPCID              string
	SubnetIDs          []string
	SecurityGroupIDs   []string
	ServiceIPRange     string // CIDR block for service IPs
	PodIPRange         string // CIDR block for pod IPs
	DNSClusterIP       string
	PublicAPIAccess    bool
	PrivateAPIAccess   bool
	AuthorizedNetworks []string // CIDR blocks allowed to access API server
	Addons             []ClusterAddon
}

// ClusterStatus represents the current state of a cluster
type ClusterStatus struct {
	LifecycleStatus ClusterLifecycleStatus
	Messages        []string
}

// ClusterLifecycleStatus represents the lifecycle state of a cluster
type ClusterLifecycleStatus string

const (
	ClusterLifecycleStatusCreating  ClusterLifecycleStatus = "creating"
	ClusterLifecycleStatusActive    ClusterLifecycleStatus = "active"
	ClusterLifecycleStatusUpdating  ClusterLifecycleStatus = "updating"
	ClusterLifecycleStatusDeleting  ClusterLifecycleStatus = "deleting"
	ClusterLifecycleStatusDeleted   ClusterLifecycleStatus = "deleted"
	ClusterLifecycleStatusFailed    ClusterLifecycleStatus = "failed"
	ClusterLifecycleStatusDegraded  ClusterLifecycleStatus = "degraded"
)

// NodeGroup represents a group of worker nodes in a cluster
type NodeGroup struct {
	Name         string
	InstanceType string
	MinSize      int
	MaxSize      int
	DesiredSize  int
	DiskSize     int64 // in GB
	VolumeType   string
	Labels       map[string]string
	Taints       []NodeTaint
	Tags         Tags
	SubnetIDs    []string
	UseSpot      bool
	ImageID      string // AMI ID or equivalent
	UserData     string // base64 encoded user data
}

// NodeTaint represents a Kubernetes node taint
type NodeTaint struct {
	Key    string
	Value  string
	Effect TaintEffect
}

// TaintEffect represents the effect of a node taint
type TaintEffect string

const (
	TaintEffectNoSchedule       TaintEffect = "NoSchedule"
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"
	TaintEffectNoExecute        TaintEffect = "NoExecute"
)

// ClusterAddon represents an add-on or extension installed on the cluster
type ClusterAddon struct {
	Name    string
	Version string
	Enabled bool
	Config  map[string]interface{}
}

// CloudProviderClusterID is the cloud provider's identifier for a cluster
type CloudProviderClusterID string

// CreateClusterAttrs contains attributes for creating a new cluster
type CreateClusterAttrs struct {
	Name               string
	RefID              string // required, can be used for idempotency
	Location           string // region
	SubLocation        string // availability zone preference
	Version            string // Kubernetes version
	VPCID              string // if empty, a new VPC will be created
	SubnetIDs          []string // if empty, new subnets will be created
	SecurityGroupIDs   []string // additional security groups
	Tags               Tags
	ServiceIPRange     string // CIDR block for service IPs (optional)
	PodIPRange         string // CIDR block for pod IPs (optional)  
	PublicAPIAccess    bool   // whether API server should be publicly accessible
	PrivateAPIAccess   bool   // whether API server should be privately accessible
	AuthorizedNetworks []string // CIDR blocks allowed to access API server
	NodeGroups         []CreateNodeGroupAttrs
	Addons             []CreateClusterAddonAttrs
	LogTypes           []string // types of logs to enable (e.g., "api", "audit", "authenticator")
	
	// IAM Role ARNs - if provided, will be used; if empty, will be created automatically
	ClusterServiceRoleArn string // IAM role for EKS cluster service
	NodeGroupRoleArn      string // IAM role for EKS worker nodes
}

// CreateNodeGroupAttrs contains attributes for creating a node group
type CreateNodeGroupAttrs struct {
	Name         string
	InstanceType string
	MinSize      int
	MaxSize      int
	DesiredSize  int
	DiskSize     int64 // in GB
	VolumeType   string
	Labels       map[string]string
	Taints       []NodeTaint
	Tags         Tags
	SubnetIDs    []string // if empty, will use cluster subnets
	UseSpot      bool
	ImageID      string // optional, will use default if empty
	UserData     string // base64 encoded user data
}

// CreateClusterAddonAttrs contains attributes for creating a cluster add-on
type CreateClusterAddonAttrs struct {
	Name    string
	Version string // optional, will use default if empty
	Config  map[string]interface{}
}

// ListClustersArgs contains arguments for listing clusters
type ListClustersArgs struct {
	ClusterIDs []CloudProviderClusterID
	TagFilters map[string][]string
	Locations  LocationsFilter
}

// Cluster operation timeouts
const (
	CreatingToActiveTimeout  = 30 * time.Minute
	UpdatingToActiveTimeout  = 20 * time.Minute
	ActiveToDeletedTimeout   = 20 * time.Minute
	DeletingToDeletedTimeout = 20 * time.Minute
)

// ValidateCreateCluster validates cluster creation and returns the created cluster
func ValidateCreateCluster(ctx context.Context, client CloudClusterManager, attrs CreateClusterAttrs) (*Cluster, error) {
	t0 := time.Now().Add(-time.Minute)
	
	if attrs.RefID == "" {
		return nil, errors.New("RefID is required for cluster creation")
	}
	
	if attrs.Name == "" {
		return nil, errors.New("Name is required for cluster creation")
	}
	
	if attrs.Location == "" {
		return nil, errors.New("Location is required for cluster creation")
	}
	
	cluster, err := client.CreateCluster(ctx, attrs)
	if err != nil {
		return nil, err
	}
	
	var validationErr error
	t1 := time.Now().Add(1 * time.Minute)
	diff := t1.Sub(t0)
	
	if diff > 5*time.Minute {
		validationErr = errors.Join(validationErr, fmt.Errorf("create cluster took too long: %s", diff))
	}
	
	if cluster.CreatedAt.Before(t0) {
		validationErr = errors.Join(validationErr, fmt.Errorf("createdAt is before t0: %s", cluster.CreatedAt))
	}
	
	if cluster.CreatedAt.After(t1) {
		validationErr = errors.Join(validationErr, fmt.Errorf("createdAt is after t1: %s", cluster.CreatedAt))
	}
	
	if cluster.RefID != attrs.RefID {
		validationErr = errors.Join(validationErr, fmt.Errorf("refID mismatch: %s != %s", cluster.RefID, attrs.RefID))
	}
	
	if attrs.Location != "" && attrs.Location != cluster.Location {
		validationErr = errors.Join(validationErr, fmt.Errorf("location mismatch: %s != %s", attrs.Location, cluster.Location))
	}
	
	if attrs.Version != "" && attrs.Version != cluster.Version {
		validationErr = errors.Join(validationErr, fmt.Errorf("version mismatch: %s != %s", attrs.Version, cluster.Version))
	}
	
	return cluster, validationErr
}

// ValidateListCreatedCluster validates that a created cluster appears in the list
func ValidateListCreatedCluster(ctx context.Context, client CloudClusterManager, cluster *Cluster) error {
	clusters, err := client.ListClusters(ctx, ListClustersArgs{
		Locations: []string{cluster.Location},
	})
	if err != nil {
		return err
	}
	
	var validationErr error
	if len(clusters) == 0 {
		validationErr = errors.Join(validationErr, fmt.Errorf("no clusters found"))
	}
	
	var foundCluster *Cluster
	for i := range clusters {
		if clusters[i].CloudID == cluster.CloudID {
			foundCluster = &clusters[i]
			break
		}
	}
	
	if foundCluster == nil {
		validationErr = errors.Join(validationErr, fmt.Errorf("cluster not found: %s", cluster.CloudID))
	} else {
		if foundCluster.Location != cluster.Location {
			validationErr = errors.Join(validationErr, fmt.Errorf("location mismatch: %s != %s", foundCluster.Location, cluster.Location))
		}
		if foundCluster.RefID == "" {
			validationErr = errors.Join(validationErr, fmt.Errorf("refID is empty"))
		}
		if foundCluster.RefID != cluster.RefID {
			validationErr = errors.Join(validationErr, fmt.Errorf("refID mismatch: %s != %s", foundCluster.RefID, cluster.RefID))
		}
		if foundCluster.CloudCredRefID == "" {
			validationErr = errors.Join(validationErr, fmt.Errorf("cloudCredRefID is empty"))
		}
		if foundCluster.CloudCredRefID != cluster.CloudCredRefID {
			validationErr = errors.Join(validationErr, fmt.Errorf("cloudCredRefID mismatch: %s != %s", foundCluster.CloudCredRefID, cluster.CloudCredRefID))
		}
	}
	
	return validationErr
}

// ValidateDeleteCluster validates cluster deletion
func ValidateDeleteCluster(ctx context.Context, client CloudClusterManager, cluster *Cluster) error {
	err := client.DeleteCluster(ctx, cluster.CloudID)
	if err != nil {
		return err
	}
	// TODO: wait for cluster to go into deleting state
	return nil
}