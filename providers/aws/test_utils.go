package aws

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Create a VPC with one subnet and internet access
func createVpcWithInternetAccess(ec2Client *ec2.EC2) (*string, error) {
	vpcCreateResponse, err := ec2Client.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: aws.String("172.31.0.0/16"),
	})
	if err != nil {
		return nil, err
	}
	vpcID := vpcCreateResponse.Vpc.VpcId

	err = tagResource(ec2Client, vpcID)
	if err != nil {
		return nil, err
	}

	subnetResponse, err := ec2Client.CreateSubnet(&ec2.CreateSubnetInput{
		CidrBlock: aws.String("172.31.0.0/16"),
		VpcId:     vpcID,
	})
	if err != nil {
		return nil, err
	}
	subnetID := subnetResponse.Subnet.SubnetId

	err = tagResource(ec2Client, subnetID)
	if err != nil {
		return nil, err
	}

	igResponse, err := ec2Client.CreateInternetGateway(&ec2.CreateInternetGatewayInput{})
	if err != nil {
		return nil, err
	}
	igID := igResponse.InternetGateway.InternetGatewayId

	err = tagResource(ec2Client, igID)
	if err != nil {
		return nil, err
	}

	_, err = ec2Client.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
		InternetGatewayId: igID,
		VpcId:             vpcID,
	})
	if err != nil {
		return nil, err
	}

	routeTableResponse, err := ec2Client.CreateRouteTable(&ec2.CreateRouteTableInput{
		VpcId: vpcID,
	})
	if err != nil {
		return nil, err
	}

	routeTableID := routeTableResponse.RouteTable.RouteTableId

	err = tagResource(ec2Client, routeTableID)
	if err != nil {
		return nil, err
	}

	_, err = ec2Client.AssociateRouteTable(&ec2.AssociateRouteTableInput{
		RouteTableId: routeTableID,
		SubnetId:     subnetID,
	})
	if err != nil {
		return nil, err
	}

	_, err = ec2Client.CreateRoute(&ec2.CreateRouteInput{
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            igID,
		RouteTableId:         routeTableID,
	})
	if err != nil {
		return nil, err
	}

	return subnetID, nil
}

func tagResource(ec2Client *ec2.EC2, resourceID *string) error {
	attempts := 10
	sleep := time.Second * 5

	for {
		if attempts == 0 {
			return fmt.Errorf("Failed to tag resource: %s", *resourceID)
		}
		_, err := ec2Client.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{resourceID},
			Tags: []*ec2.Tag{&ec2.Tag{
				Key:   aws.String("Name"),
				Value: aws.String("vk-aws-e2e-test"),
			}},
		})

		if err == nil {
			return nil
		}

		fmt.Printf("Failed tagging: %s, err: %s, retrying...", *resourceID, err)

		time.Sleep(sleep)
	}

}

// Conservatively deleting all resources of the created VPC by enumarating all tagged
// resources and deleting them with multiple retries
func deleteVpc(ec2Client *ec2.EC2) error {
	retryDelete(func() error {
		resourceIDs, err := findResourceByTag(ec2Client, "route-table")
		if err != nil {
			return err
		}

		describeResponse, err := ec2Client.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
			RouteTableIds: resourceIDs,
		})
		if err != nil {
			return err
		}

		for _, routeTable := range describeResponse.RouteTables {
			for _, association := range routeTable.Associations {
				_, err = ec2Client.DisassociateRouteTable(&ec2.DisassociateRouteTableInput{
					AssociationId: association.RouteTableAssociationId,
				})

				if err != nil {
					return err
				}
			}

			_, err = ec2Client.DeleteRouteTable(&ec2.DeleteRouteTableInput{
				RouteTableId: routeTable.RouteTableId,
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	retryDelete(func() error {
		resourceIDs, err := findResourceByTag(ec2Client, "internet-gateway")
		if err != nil {
			return err
		}

		describeResponse, err := ec2Client.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
			InternetGatewayIds: resourceIDs,
		})
		if err != nil {
			return err
		}

		for _, internetGateway := range describeResponse.InternetGateways {
			for _, attachment := range internetGateway.Attachments {
				_, err = ec2Client.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
					InternetGatewayId: internetGateway.InternetGatewayId,
					VpcId:             attachment.VpcId,
				})

				if err != nil {
					return err
				}
			}

			_, err = ec2Client.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
				InternetGatewayId: internetGateway.InternetGatewayId,
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	retryDelete(func() error {
		resourceIDs, err := findResourceByTag(ec2Client, "subnet")
		if err != nil {
			return err
		}

		for _, resourceID := range resourceIDs {
			_, err = ec2Client.DeleteSubnet(&ec2.DeleteSubnetInput{
				SubnetId: resourceID,
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	retryDelete(func() error {
		resourceIDs, err := findResourceByTag(ec2Client, "vpc")
		if err != nil {
			return err
		}

		for _, resourceID := range resourceIDs {
			_, err = ec2Client.DeleteVpc(&ec2.DeleteVpcInput{
				VpcId: resourceID,
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

type fn func() error

func retryDelete(action fn) error {
	attempts := 10
	sleep := time.Second * 5

	for {
		if attempts == 0 {
			return fmt.Errorf("Failed to delete")
		}

		err := action()
		if err == nil {
			return nil
		}

		fmt.Printf("Failed deleting, err: %s retrying...", err)

		time.Sleep(sleep)
	}
}

func findResourceByTag(ec2Client *ec2.EC2, resourceType string) ([]*string, error) {
	describeResponse, err := ec2Client.DescribeTags(&ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("key"),
				Values: []*string{aws.String("Name")},
			},
			&ec2.Filter{
				Name:   aws.String("value"),
				Values: []*string{aws.String("vk-aws-e2e-test")},
			},
			&ec2.Filter{
				Name:   aws.String("resource-type"),
				Values: []*string{aws.String(resourceType)},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	resourceIDs := make([]*string, len(describeResponse.Tags))
	for i, tag := range describeResponse.Tags {
		resourceIDs[i] = tag.ResourceId
	}

	return resourceIDs, nil
}
