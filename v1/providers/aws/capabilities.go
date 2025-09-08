package v1

import (
	"context"

	v1 "github.com/brevdev/cloud/v1"
)

// getAWSCapabilities returns the unified capabilities for AWS
// Based on EC2 API documentation at https://docs.aws.amazon.com/ec2/
func getAWSCapabilities() v1.Capabilities {
	return v1.Capabilities{
		// SUPPORTED FEATURES (with AWS API evidence):

		// Core Instance Management
		v1.CapabilityCreateInstance,          // EC2 RunInstances
		v1.CapabilityCreateIdempotentInstance, // EC2 supports ClientToken for idempotency
		v1.CapabilityTerminateInstance,       // EC2 TerminateInstances
		v1.CapabilityCreateTerminateInstance, // Combined create/terminate capability

		// Advanced Instance Operations
		v1.CapabilityStopStartInstance, // EC2 StopInstances/StartInstances
		v1.CapabilityRebootInstance,    // EC2 RebootInstances

		// Storage Operations
		v1.CapabilityResizeInstanceVolume, // EBS ModifyVolume

		// Network Security
		v1.CapabilityModifyFirewall, // Security Groups AuthorizeSecurityGroupIngress/Egress

		// Resource Management
		v1.CapabilityTags,         // EC2 CreateTags/DeleteTags
		v1.CapabilityMachineImage, // EC2 DescribeImages

		// User Data Support
		v1.CapabilityInstanceUserData, // EC2 RunInstances UserData parameter

		// Kubernetes Cluster Management
		v1.CapabilityCreateCluster,    // EKS CreateCluster
		v1.CapabilityListClusters,     // EKS ListClusters/DescribeCluster  
		v1.CapabilityGetCluster,       // EKS DescribeCluster
		v1.CapabilityDeleteCluster,    // EKS DeleteCluster
		v1.CapabilityClusterManagement, // Combined cluster management capability
	}
}

// GetCapabilities returns the capabilities of AWS client
func (c *AWSClient) GetCapabilities(_ context.Context) (v1.Capabilities, error) {
	return getAWSCapabilities(), nil
}

// GetCapabilities returns the capabilities for AWS credential
func (c *AWSCredential) GetCapabilities(_ context.Context) (v1.Capabilities, error) {
	return getAWSCapabilities(), nil
}