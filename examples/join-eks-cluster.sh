#!/bin/bash

# EKS Cluster Join Script for EKS-Optimized AMIs
# This script joins an EC2 instance (launched with EKS-optimized AMI) to an existing EKS cluster

set -e

echo "🚀 EKS Cluster Join Script (EKS-Optimized AMI)"
echo "==============================================="

# Configuration
EKS_CLUSTER_NAME="${EKS_CLUSTER_NAME:-my-eks-cluster}"
AWS_REGION="${AWS_REGION:-us-west-2}"
INSTANCE_ID="${INSTANCE_ID}"
SSH_KEY_PATH="${SSH_KEY_PATH:-~/.ssh/aws-test-key}"

# Validate inputs
if [ -z "$INSTANCE_ID" ]; then
    echo "❌ Error: INSTANCE_ID environment variable must be set"
    echo "   Usage: INSTANCE_ID=i-1234567890abcdef0 $0"
    exit 1
fi

echo "✅ Configuration:"
echo "   EKS Cluster: $EKS_CLUSTER_NAME"
echo "   Instance ID: $INSTANCE_ID"
echo "   AWS Region: $AWS_REGION"

# Step 1: Get EKS cluster info
echo ""
echo "🔍 Step 1: Getting EKS cluster information..."
EKS_ENDPOINT=$(aws eks describe-cluster --region "$AWS_REGION" --name "$EKS_CLUSTER_NAME" --query "cluster.endpoint" --output text)
EKS_CA_DATA=$(aws eks describe-cluster --region "$AWS_REGION" --name "$EKS_CLUSTER_NAME" --query "cluster.certificateAuthority.data" --output text)

echo "   ✅ Cluster endpoint: $EKS_ENDPOINT"

# Step 2: Get instance IP
echo ""
echo "🔍 Step 2: Getting instance information..."
PUBLIC_IP=$(aws ec2 describe-instances --region "$AWS_REGION" --instance-ids "$INSTANCE_ID" --query "Reservations[0].Instances[0].PublicIpAddress" --output text)
PRIVATE_IP=$(aws ec2 describe-instances --region "$AWS_REGION" --instance-ids "$INSTANCE_ID" --query "Reservations[0].Instances[0].PrivateIpAddress" --output text)

echo "   ✅ Public IP: $PUBLIC_IP"
echo "   ✅ Private IP: $PRIVATE_IP"

# Step 3: Setup IAM instance profile for EKS
echo ""
echo "🔐 Step 3: Setting up IAM permissions for EKS..."

ROLE_NAME="BrevEKSNodeRole-${EKS_CLUSTER_NAME}"
INSTANCE_PROFILE_NAME="BrevEKSNodeProfile-${EKS_CLUSTER_NAME}"

# Check if instance already has a profile
EXISTING_PROFILE=$(aws ec2 describe-instances --region "$AWS_REGION" --instance-ids "$INSTANCE_ID" --query "Reservations[0].Instances[0].IamInstanceProfile.Arn" --output text)

if [ "$EXISTING_PROFILE" != "None" ] && [ -n "$EXISTING_PROFILE" ]; then
    echo "   ✅ Instance already has profile: $EXISTING_PROFILE"
else
    echo "   Creating EKS node IAM role and instance profile..."
    
    # Create IAM role for EKS nodes
    if ! aws iam get-role --role-name "$ROLE_NAME" > /dev/null 2>&1; then
        echo "   Creating IAM role: $ROLE_NAME"
        
        # Trust policy for EC2 instances
        TRUST_POLICY=$(cat <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF
)
        
        aws iam create-role --role-name "$ROLE_NAME" --assume-role-policy-document "$TRUST_POLICY"
        
        # Attach required EKS policies
        aws iam attach-role-policy --role-name "$ROLE_NAME" --policy-arn arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy
        aws iam attach-role-policy --role-name "$ROLE_NAME" --policy-arn arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy  
        aws iam attach-role-policy --role-name "$ROLE_NAME" --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly
        
        echo "   ✅ IAM role created with EKS policies attached"
        
        # Wait for role propagation
        echo "   Waiting for IAM role propagation..."
        sleep 10
    else
        echo "   ✅ IAM role already exists: $ROLE_NAME"
    fi
    
    # Create instance profile
    if ! aws iam get-instance-profile --instance-profile-name "$INSTANCE_PROFILE_NAME" > /dev/null 2>&1; then
        echo "   Creating instance profile: $INSTANCE_PROFILE_NAME"
        aws iam create-instance-profile --instance-profile-name "$INSTANCE_PROFILE_NAME"
        aws iam add-role-to-instance-profile --instance-profile-name "$INSTANCE_PROFILE_NAME" --role-name "$ROLE_NAME"
        
        # Wait for instance profile propagation
        echo "   Waiting for instance profile propagation..."
        sleep 20
    else
        echo "   ✅ Instance profile already exists: $INSTANCE_PROFILE_NAME"
    fi
    
    # Attach instance profile to EC2 instance
    echo "   Attaching instance profile to instance..."
    aws ec2 associate-iam-instance-profile --region "$AWS_REGION" --instance-id "$INSTANCE_ID" --iam-instance-profile Name="$INSTANCE_PROFILE_NAME"
    
    echo "   ✅ Instance profile attached to instance"
    echo "   Waiting for IAM permissions to propagate..."
    sleep 30
fi

# Step 4: Fix security group for EKS connectivity  
echo ""
echo "🔧 Step 4: Configuring security groups for EKS connectivity..."

# Get the EKS cluster security group
EKS_CLUSTER_SG=$(aws eks describe-cluster --region "$AWS_REGION" --name "$EKS_CLUSTER_NAME" --query "cluster.resourcesVpcConfig.clusterSecurityGroupId" --output text)

if [ "$EKS_CLUSTER_SG" = "None" ] || [ -z "$EKS_CLUSTER_SG" ]; then
    # Fallback: find cluster SG by tags
    EKS_CLUSTER_SG=$(aws ec2 describe-security-groups --region "$AWS_REGION" --filters "Name=tag:aws:eks:cluster-name,Values=$EKS_CLUSTER_NAME" --query "SecurityGroups[0].GroupId" --output text)
fi

if [ "$EKS_CLUSTER_SG" = "None" ] || [ -z "$EKS_CLUSTER_SG" ]; then
    echo "   ❌ Could not find EKS cluster security group"
    exit 1
fi

echo "   Found EKS cluster security group: $EKS_CLUSTER_SG"

# Get current instance security groups
CURRENT_SGS=$(aws ec2 describe-instances --region "$AWS_REGION" --instance-ids "$INSTANCE_ID" --query "Reservations[0].Instances[0].SecurityGroups[].GroupId" --output text)
echo "   Current security groups: $CURRENT_SGS"

# Check if instance already has the EKS cluster security group
if echo "$CURRENT_SGS" | grep -q "$EKS_CLUSTER_SG"; then
    echo "   ✅ Instance already has EKS cluster security group"
else
    echo "   Adding EKS cluster security group to instance..."
    
    # Add the EKS cluster security group to the instance
    ALL_SGS="$CURRENT_SGS $EKS_CLUSTER_SG"
    
    # Modify instance security groups
    aws ec2 modify-instance-attribute --region "$AWS_REGION" --instance-id "$INSTANCE_ID" --groups $ALL_SGS
    
    if [ $? -eq 0 ]; then
        echo "   ✅ Added EKS cluster security group to instance"
        echo "   Waiting for security group changes to take effect..."
        sleep 10
    else
        echo "   ❌ Failed to modify instance security groups"
        exit 1
    fi
fi

# Step 5: Update aws-auth ConfigMap to allow our role
echo ""
echo "🔧 Step 5: Configuring EKS cluster access..."

ROLE_ARN="arn:aws:iam::$(aws sts get-caller-identity --query Account --output text):role/$ROLE_NAME"
echo "   Checking if role is authorized: $ROLE_ARN"

# Check if our role is already in aws-auth ConfigMap
if kubectl get configmap aws-auth -n kube-system -o yaml | grep -q "$ROLE_ARN"; then
    echo "   ✅ Role already authorized in aws-auth ConfigMap"
else
    echo "   Adding role to aws-auth ConfigMap..."
    
    # Get current mapRoles and append our role
    CURRENT_MAP_ROLES=$(kubectl get configmap aws-auth -n kube-system -o jsonpath='{.data.mapRoles}')
    
    # Create new mapRoles with our role appended (ensure proper formatting)
    NEW_MAP_ROLES="${CURRENT_MAP_ROLES}
    - groups:
      - system:bootstrappers
      - system:nodes
      rolearn: ${ROLE_ARN}
      username: system:node:{{EC2PrivateDNSName}}"
    
    # Update the ConfigMap using kubectl create with --dry-run and apply
    kubectl create configmap aws-auth --from-literal=mapRoles="$NEW_MAP_ROLES" --from-literal=mapUsers="[]" -n kube-system --dry-run=client -o yaml | kubectl apply -f -
    
    if [ $? -eq 0 ]; then
        echo "   ✅ Role added to aws-auth ConfigMap"
        echo "   Waiting for changes to propagate..."
        sleep 10
    else
        echo "   ❌ Failed to update aws-auth ConfigMap"
        echo "   Please manually add the role:"
        echo "   kubectl edit configmap aws-auth -n kube-system"
        echo "   Add this entry under mapRoles:"
        echo "   - groups:"
        echo "     - system:bootstrappers" 
        echo "     - system:nodes"
        echo "     rolearn: $ROLE_ARN"
        echo "     username: system:node:{{EC2PrivateDNSName}}"
        exit 1
    fi
fi

# Step 6: Join cluster via SSH
echo ""
echo "🚀 Step 6: Joining EKS cluster..."

# Use EKS bootstrap script (available on EKS-optimized AMIs)
JOIN_COMMAND="sudo /etc/eks/bootstrap.sh $EKS_CLUSTER_NAME --b64-cluster-ca '$EKS_CA_DATA' --apiserver-endpoint '$EKS_ENDPOINT'"

echo "   Running bootstrap command..."
ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no ec2-user@"$PUBLIC_IP" "$JOIN_COMMAND"

echo "   ✅ Bootstrap completed!"

# Step 7: Verify
echo ""
echo "🔍 Step 7: Verifying node joined..."
sleep 30

NODE_NAME="ip-$(echo $PRIVATE_IP | tr '.' '-').$AWS_REGION.compute.internal"
echo "   Expected node name: $NODE_NAME"

# Check if node appears in cluster
if aws eks update-kubeconfig --region "$AWS_REGION" --name "$EKS_CLUSTER_NAME" --dry-run > /tmp/kubeconfig 2>/dev/null; then
    if kubectl --kubeconfig /tmp/kubeconfig get nodes | grep -q "$NODE_NAME"; then
        echo "   ✅ Node successfully joined cluster!"
    else
        echo "   ⚠️  Node still joining... check: kubectl get nodes"
    fi
fi

echo ""
echo "🎉 EKS join completed!"
echo ""
echo "Integration points for Brev AWS provider:"
echo "1. Add EKSClusterName field to CreateInstanceAttrs"
echo "2. Auto-select EKS-optimized AMI based on cluster version"
echo "3. Create/attach EKS node IAM instance profile"
echo "4. Generate user data with bootstrap.sh command"