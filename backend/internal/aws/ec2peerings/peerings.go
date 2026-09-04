package ec2peerings

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Connection describes a VPC peering connection in the account.
type Connection struct {
	PeeringID    string `json:"peering_id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Region       string `json:"region"`
	LocalVPC     string `json:"local_vpc"`
	LocalCIDR    string `json:"local_cidr,omitempty"`
	PeerVPC      string `json:"peer_vpc"`
	PeerCIDR     string `json:"peer_cidr,omitempty"`
	PeerRegion   string `json:"peer_region,omitempty"`
	PeerAccount  string `json:"peer_account,omitempty"`
	LocalRole    string `json:"local_role"` // requester or accepter
}

// List returns VPC peering connections visible in the given region.
func List(ctx context.Context, client *ec2.Client, accountID, region string) ([]Connection, error) {
	out, err := client.DescribeVpcPeeringConnections(ctx, &ec2.DescribeVpcPeeringConnectionsInput{
		Filters: []types.Filter{{
			Name:   aws.String("status-code"),
			Values: []string{"active", "pending-acceptance", "provisioning"},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("describe vpc peering connections: %w", err)
	}

	var conns []Connection
	for _, pcx := range out.VpcPeeringConnections {
		c := mapPeeringConnection(pcx, accountID, region)
		if c.PeeringID == "" {
			continue
		}
		conns = append(conns, c)
	}
	sort.Slice(conns, func(i, j int) bool {
		if conns[i].Name == conns[j].Name {
			return conns[i].PeeringID < conns[j].PeeringID
		}
		return conns[i].Name < conns[j].Name
	})
	return conns, nil
}

func mapPeeringConnection(pcx types.VpcPeeringConnection, accountID, region string) Connection {
	id := aws.ToString(pcx.VpcPeeringConnectionId)
	req := pcx.RequesterVpcInfo
	acc := pcx.AccepterVpcInfo

	name := ""
	for _, t := range pcx.Tags {
		if aws.ToString(t.Key) == "Name" {
			name = aws.ToString(t.Value)
			break
		}
	}
	if name == "" {
		name = id
	}

	c := Connection{
		PeeringID: id,
		Name:      name,
		Status:    string(pcx.Status.Code),
		Region:    region,
	}

	reqAcct := aws.ToString(req.OwnerId)
	accAcct := aws.ToString(acc.OwnerId)

	switch accountID {
	case reqAcct:
		c.LocalRole = "requester"
		c.LocalVPC = aws.ToString(req.VpcId)
		c.LocalCIDR = aws.ToString(req.CidrBlock)
		c.PeerVPC = aws.ToString(acc.VpcId)
		c.PeerCIDR = aws.ToString(acc.CidrBlock)
		c.PeerRegion = aws.ToString(acc.Region)
		c.PeerAccount = accAcct
	case accAcct:
		c.LocalRole = "accepter"
		c.LocalVPC = aws.ToString(acc.VpcId)
		c.LocalCIDR = aws.ToString(acc.CidrBlock)
		c.PeerVPC = aws.ToString(req.VpcId)
		c.PeerCIDR = aws.ToString(req.CidrBlock)
		c.PeerRegion = aws.ToString(req.Region)
		c.PeerAccount = reqAcct
	default:
		return Connection{}
	}
	return c
}

// PeeringsForVPC returns peering IDs that involve the VPC.
func PeeringsForVPC(vpcID string, peerings []Connection) []string {
	var ids []string
	for _, p := range peerings {
		if p.LocalVPC == vpcID || p.PeerVPC == vpcID {
			ids = append(ids, p.PeeringID)
		}
	}
	return ids
}
