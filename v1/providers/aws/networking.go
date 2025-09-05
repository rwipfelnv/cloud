package v1

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/brevdev/cloud/v1"
)

// AddFirewallRulesToInstance adds firewall rules to an instance's security groups
func (c *AWSClient) AddFirewallRulesToInstance(ctx context.Context, args v1.AddFirewallRulesToInstanceArgs) error {

	// Get the instance's network interfaces to find security groups
	describeResult, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{string(args.InstanceID)},
	})
	if err != nil {
		return fmt.Errorf("failed to describe instance: %w", err)
	}

	var securityGroups []string
	for _, reservation := range describeResult.Reservations {
		for _, ec2Instance := range reservation.Instances {
			if aws.ToString(ec2Instance.InstanceId) == string(args.InstanceID) {
				for _, sg := range ec2Instance.SecurityGroups {
					securityGroups = append(securityGroups, aws.ToString(sg.GroupId))
				}
				break
			}
		}
	}

	if len(securityGroups) == 0 {
		return fmt.Errorf("no security groups found for instance %s", args.InstanceID)
	}

	// Use the first security group for adding rules
	primarySGID := securityGroups[0]

	// Add ingress rules
	if len(args.FirewallRules.IngressRules) > 0 {
		if err := c.addIngressRules(ctx, primarySGID, args.FirewallRules.IngressRules); err != nil {
			return fmt.Errorf("failed to add ingress rules: %w", err)
		}
	}

	// Add egress rules
	if len(args.FirewallRules.EgressRules) > 0 {
		if err := c.addEgressRules(ctx, primarySGID, args.FirewallRules.EgressRules); err != nil {
			return fmt.Errorf("failed to add egress rules: %w", err)
		}
	}

	return nil
}

// RevokeSecurityGroupRules removes specific security group rules
func (c *AWSClient) RevokeSecurityGroupRules(ctx context.Context, args v1.RevokeSecurityGroupRuleArgs) error {
	// Get instance details to find its security groups
	describeResult, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{string(args.InstanceID)},
	})
	if err != nil {
		return fmt.Errorf("failed to describe instance: %w", err)
	}

	var securityGroups []string
	for _, reservation := range describeResult.Reservations {
		for _, ec2Instance := range reservation.Instances {
			if aws.ToString(ec2Instance.InstanceId) == string(args.InstanceID) {
				for _, sg := range ec2Instance.SecurityGroups {
					securityGroups = append(securityGroups, aws.ToString(sg.GroupId))
				}
				break
			}
		}
	}

	if len(securityGroups) == 0 {
		return fmt.Errorf("no security groups found for instance %s", args.InstanceID)
	}

	// Revoke rules from all security groups associated with the instance
	for _, sgID := range securityGroups {
		for _, ruleID := range args.SecurityGroupRuleIDs {
			// Try to revoke as ingress rule first
			_, ingressErr := c.ec2Client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
				GroupId:                 aws.String(sgID),
				SecurityGroupRuleIds:   []string{ruleID},
			})

			// If ingress revoke fails, try egress
			if ingressErr != nil {
				_, egressErr := c.ec2Client.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{
					GroupId:                aws.String(sgID),
					SecurityGroupRuleIds:  []string{ruleID},
				})
				if egressErr != nil {
					return fmt.Errorf("failed to revoke rule %s from security group %s: ingress error: %v, egress error: %v", ruleID, sgID, ingressErr, egressErr)
				}
			}
		}
	}

	return nil
}

func (c *AWSClient) addIngressRules(ctx context.Context, securityGroupID string, rules []v1.FirewallRule) error {
	var permissions []types.IpPermission

	for _, rule := range rules {
		permission := types.IpPermission{
			IpProtocol: aws.String("tcp"), // Default to TCP
			FromPort:   aws.Int32(rule.FromPort),
			ToPort:     aws.Int32(rule.ToPort),
		}

		// Add IP ranges
		for _, ipRange := range rule.IPRanges {
			permission.IpRanges = append(permission.IpRanges, types.IpRange{
				CidrIp: aws.String(ipRange),
			})
		}

		// If no IP ranges specified, default to 0.0.0.0/0 (open to world)
		if len(permission.IpRanges) == 0 {
			permission.IpRanges = append(permission.IpRanges, types.IpRange{
				CidrIp: aws.String("0.0.0.0/0"),
			})
		}

		permissions = append(permissions, permission)
	}

	if len(permissions) > 0 {
		_, err := c.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId:       aws.String(securityGroupID),
			IpPermissions: permissions,
		})
		if err != nil {
			// Handle duplicate rule error gracefully
			if isDuplicateRuleError(err) {
				return v1.ErrDuplicateFirewallRule
			}
			return err
		}
	}

	return nil
}

func (c *AWSClient) addEgressRules(ctx context.Context, securityGroupID string, rules []v1.FirewallRule) error {
	var permissions []types.IpPermission

	for _, rule := range rules {
		permission := types.IpPermission{
			IpProtocol: aws.String("tcp"), // Default to TCP
			FromPort:   aws.Int32(rule.FromPort),
			ToPort:     aws.Int32(rule.ToPort),
		}

		// Add IP ranges
		for _, ipRange := range rule.IPRanges {
			permission.IpRanges = append(permission.IpRanges, types.IpRange{
				CidrIp: aws.String(ipRange),
			})
		}

		// If no IP ranges specified, default to 0.0.0.0/0 (open to world)
		if len(permission.IpRanges) == 0 {
			permission.IpRanges = append(permission.IpRanges, types.IpRange{
				CidrIp: aws.String("0.0.0.0/0"),
			})
		}

		permissions = append(permissions, permission)
	}

	if len(permissions) > 0 {
		_, err := c.ec2Client.AuthorizeSecurityGroupEgress(ctx, &ec2.AuthorizeSecurityGroupEgressInput{
			GroupId:       aws.String(securityGroupID),
			IpPermissions: permissions,
		})
		if err != nil {
			// Handle duplicate rule error gracefully
			if isDuplicateRuleError(err) {
				return v1.ErrDuplicateFirewallRule
			}
			return err
		}
	}

	return nil
}

func isDuplicateRuleError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "InvalidPermission.Duplicate"
}