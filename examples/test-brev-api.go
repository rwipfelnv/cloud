package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	awsprovider "github.com/brevdev/cloud/v1/providers/aws"
	"github.com/brevdev/cloud/v1"
)

func main() {
	// Command line flags
	var (
		action      = flag.String("action", "create", "Action to perform: create, destroy, or list")
		instanceID  = flag.String("instance-id", "", "Instance ID for destroy action")
		region      = flag.String("region", "us-west-2", "AWS region")
		instanceType = flag.String("type", "t3.micro", "EC2 instance type")
		name        = flag.String("name", "brev-test-instance", "Instance name")
	)
	flag.Parse()

	if *action == "destroy" && *instanceID == "" {
		log.Fatal("Error: --instance-id is required for destroy action")
	}

	ctx := context.Background()

	// Create AWS credential (uses default credential chain)
	fmt.Println("🔑 Creating AWS credential...")
	credential := awsprovider.NewAWSCredential("test-brev-api", "", "", 
		awsprovider.WithDefaultRegion(*region))

	// Create client
	fmt.Println("🌐 Creating AWS client...")
	client, err := credential.MakeClient(ctx, *region)
	if err != nil {
		log.Fatalf("Failed to create AWS client: %v", err)
	}

	fmt.Printf("✅ Connected to AWS provider: %s\n", client.GetCloudProviderID())

	switch *action {
	case "create":
		createInstance(ctx, client, *region, *instanceType, *name)
	case "destroy":
		destroyInstance(ctx, client, *instanceID)
	case "list":
		listInstances(ctx, client)
	default:
		log.Fatalf("Unknown action: %s. Use create, destroy, or list", *action)
	}
}

func createInstance(ctx context.Context, client v1.CloudClient, region, instanceType, name string) {
	fmt.Println("\n🚀 Creating instance...")

	// Get available instance types to validate our choice
	fmt.Println("📋 Checking available instance types...")
	types, err := client.GetInstanceTypes(ctx, v1.GetInstanceTypeArgs{
		InstanceTypes: []string{instanceType},
	})
	if err != nil {
		log.Fatalf("Failed to get instance types: %v", err)
	}

	if len(types) == 0 {
		log.Fatalf("Instance type %s not available in region %s", instanceType, region)
	}

	selectedType := types[0]
	if !selectedType.IsAvailable {
		log.Fatalf("Instance type %s is not available", instanceType)
	}

	fmt.Printf("✅ Instance type validated: %s (%d vCPU, %.1f GiB RAM)\n", 
		selectedType.Type, selectedType.VCPU, float64(selectedType.Memory)/(1024*1024*1024))

	// Use or create aws-test-key
	fmt.Println("🔑 Setting up SSH key...")
	homeDir := os.Getenv("HOME")
	keyPath := homeDir + "/.ssh/aws-test-key"
	pubKeyPath := keyPath + ".pub"
	
	var publicKey string
	if keyData, err := os.ReadFile(pubKeyPath); err == nil {
		publicKey = string(keyData)
		fmt.Println("   ✅ Using existing ~/.ssh/aws-test-key.pub")
	} else {
		fmt.Println("   Creating new SSH key pair...")
		// Generate SSH key pair
		if err := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", keyPath, "-N", "", "-C", "brev-test-key").Run(); err != nil {
			log.Fatalf("Failed to generate SSH key: %v", err)
		}
		
		keyData, err := os.ReadFile(pubKeyPath)
		if err != nil {
			log.Fatalf("Failed to read generated public key: %v", err)
		}
		publicKey = string(keyData)
		fmt.Printf("   ✅ Generated new SSH key at %s\n", keyPath)
	}

	// Create instance with EKS-optimized AMI in the EKS cluster VPC
	attrs := v1.CreateInstanceAttrs{
		Name:         name,
		InstanceType: selectedType.Type,
		Location:     selectedType.Location,
		PublicKey:    publicKey,
		ImageID:      "ami-0eb4348a120bdcf1e", // EKS-optimized AMI for x86_64 (amazon-eks-node-1.32)
		VPCID:        "vpc-0f4abb62c03df0c8b",   // EKS cluster VPC
		SubnetID:     "subnet-03e80a1f6e5d07717", // EKS cluster subnet (first one)
		DiskSize:     20 * 1024 * 1024 * 1024, // 20 GB in bytes
	}

	fmt.Printf("🏗️  Launching instance '%s' of type %s...\n", name, instanceType)
	startTime := time.Now()

	instance, err := client.CreateInstance(ctx, attrs)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("✅ Instance created successfully in %v!\n", duration.Round(time.Second))
	fmt.Printf("   Instance ID: %s\n", instance.CloudID)
	fmt.Printf("   Name: %s\n", instance.Name)
	fmt.Printf("   Type: %s\n", instance.InstanceType)
	fmt.Printf("   Status: %s\n", instance.Status)
	fmt.Printf("   Public IP: %s\n", instance.PublicIP)
	fmt.Printf("   Private IP: %s\n", instance.PrivateIP)
	fmt.Printf("   SSH User: %s\n", instance.SSHUser)
	fmt.Printf("   SSH Port: %d\n", instance.SSHPort)
}

func destroyInstance(ctx context.Context, client v1.CloudClient, instanceID string) {
	fmt.Printf("\n💥 Destroying instance %s...\n", instanceID)

	// First, get instance details
	fmt.Println("📋 Getting instance details...")
	instance, err := client.GetInstance(ctx, v1.CloudProviderInstanceID(instanceID))
	if err != nil {
		log.Fatalf("Failed to get instance: %v", err)
	}

	fmt.Printf("   Found instance: %s (%s)\n", instance.Name, instance.InstanceType)
	fmt.Printf("   Status: %s\n", instance.Status)

	startTime := time.Now()

	// Terminate the instance
	err = client.TerminateInstance(ctx, v1.CloudProviderInstanceID(instanceID))
	if err != nil {
		log.Fatalf("Failed to terminate instance: %v", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("✅ Instance termination initiated in %v\n", duration.Round(time.Second))
	fmt.Printf("   Instance %s is now being terminated\n", instanceID)
	fmt.Println("   Note: It may take a few minutes for the instance to fully terminate")
}

func listInstances(ctx context.Context, client v1.CloudClient) {
	fmt.Println("\n📋 Listing instances...")

	instances, err := client.ListInstances(ctx, v1.ListInstancesArgs{})
	if err != nil {
		log.Fatalf("Failed to list instances: %v", err)
	}

	if len(instances) == 0 {
		fmt.Println("   No instances found")
		return
	}

	fmt.Printf("   Found %d instance(s):\n\n", len(instances))

	for _, instance := range instances {
		fmt.Printf("   🖥️  %s (%s)\n", instance.Name, instance.CloudID)
		fmt.Printf("      Type: %s\n", instance.InstanceType)
		fmt.Printf("      Status: %s\n", instance.Status)
		fmt.Printf("      Public IP: %s\n", instance.PublicIP)
		fmt.Printf("      Private IP: %s\n", instance.PrivateIP)
		fmt.Printf("      Location: %s\n", instance.Location)

		// Show tags if present
		if len(instance.Tags) > 0 {
			fmt.Printf("      Tags: ")
			for k, v := range instance.Tags {
				fmt.Printf("%s=%s ", k, v)
			}
			fmt.Println()
		}
		fmt.Println()
	}
}