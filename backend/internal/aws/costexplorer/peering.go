package costexplorer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2peerings"
	"github.com/kaskol10/org-cost-api/backend/internal/errmsg"
)

// PeeringConnectionDetail is one VPC peering with estimated In/Out costs.
type PeeringConnectionDetail struct {
	PeeringID       string  `json:"peering_id"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	Region          string  `json:"region"`
	LocalVPC        string  `json:"local_vpc"`
	LocalCIDR       string  `json:"local_cidr,omitempty"`
	PeerVPC         string  `json:"peer_vpc"`
	PeerCIDR        string  `json:"peer_cidr,omitempty"`
	PeerRegion      string  `json:"peer_region,omitempty"`
	PeerAccount     string  `json:"peer_account,omitempty"`
	LocalRole       string  `json:"local_role"`
	OutTotal        float64 `json:"out_total"`
	InTotal         float64 `json:"in_total"`
	OutVPC          string  `json:"out_vpc,omitempty"`
	InVPC           string  `json:"in_vpc,omitempty"`
	AttributionNote string  `json:"attribution_note,omitempty"`
}

const costExplorerResourceLookbackDays = 14

func enrichVPCPeeringCategory(
	ctx context.Context,
	ceClient *ce.Client,
	ec2Client *ec2.Client,
	accountID, linkedAccountID, region, start, end string,
	cat *UsageCategory,
	byUsage map[string]float64,
) error {
	keys := usageTypeKeysForCategory(byUsage, "vpc-peering")
	if len(keys) == 0 {
		return nil
	}

	peerings, err := ec2peerings.List(ctx, ec2Client, accountID, region)
	if err != nil {
		cat.PeeringDetails = []PeeringConnectionDetail{}
		cat.PeeringDriversNote = errmsg.Note("Could not list VPC peerings", err)
		return nil
	}
	cat.PeeringDetails = []PeeringConnectionDetail{}

	filter := filterForUsageTypes(linkedAccountID, keys)
	ops, err := getGroupedCosts(ctx, ceClient, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionOperation))
	if err == nil {
		cat.APIOperations = toAPIOpCosts(ops, cat.Amount)
	}

	resourceStart := resourcePeriodStart(start, end)
	inKeys, outKeys := splitPeeringUsageTypes(keys)

	inCosts, outCosts, err := peeringDirectionCosts(ctx, ceClient, linkedAccountID, resourceStart, end, inKeys, outKeys)
	if err != nil {
		cat.PeeringDetails = mapPeeringDetailsWithoutCosts(peerings)
		cat.PeeringDriversNote = errmsg.Note("Could not load resource-level peering costs", err)
		return nil
	}

	vpcIDs := uniqueVPCsForAttribution(peerings, accountID)
	instancesByVPC, listErr := listInstancesByVPC(ctx, ec2Client, vpcIDs)
	if listErr != nil {
		cat.PeeringDriversNote = errmsg.Note("Could not list instances by VPC", listErr) + ". "
		instancesByVPC = map[string][]string{}
	}

	outCounts, inCounts := countDirectionPeeringsPerVPC(peerings, accountID)
	details := buildPeeringDetails(peerings, inCosts, outCosts, instancesByVPC, outCounts, inCounts, accountID)
	sort.Slice(details, func(i, j int) bool {
		return (details[i].OutTotal + details[i].InTotal) > (details[j].OutTotal + details[j].InTotal)
	})

	cat.PeeringDetails = details
	note := peeringCostNote(start, resourceStart)
	if cat.PeeringDriversNote != "" {
		note = cat.PeeringDriversNote + note
	}
	cat.PeeringDriversNote = note
	return nil
}

func splitPeeringUsageTypes(keys []string) (inKeys, outKeys []string) {
	for _, k := range keys {
		lower := strings.ToLower(k)
		switch {
		case strings.Contains(lower, "vpcpeering-in"):
			inKeys = append(inKeys, k)
		case strings.Contains(lower, "vpcpeering-out"):
			outKeys = append(outKeys, k)
		}
	}
	return inKeys, outKeys
}

func peeringDirectionCosts(
	ctx context.Context,
	ceClient *ce.Client,
	linkedAccountID, start, end string,
	inKeys, outKeys []string,
) (inCosts, outCosts map[string]float64, err error) {
	if len(outKeys) > 0 {
		outFilter := filterForUsageTypes(linkedAccountID, outKeys)
		outCosts, err = getResourceCosts(ctx, ceClient, start, end, outFilter)
		if err != nil {
			return nil, nil, err
		}
	} else {
		outCosts = map[string]float64{}
	}
	if len(inKeys) > 0 {
		inFilter := filterForUsageTypes(linkedAccountID, inKeys)
		inCosts, err = getResourceCosts(ctx, ceClient, start, end, inFilter)
		if err != nil {
			return nil, nil, err
		}
	} else {
		inCosts = map[string]float64{}
	}
	return inCosts, outCosts, nil
}

func uniqueVPCsForAttribution(peerings []ec2peerings.Connection, accountID string) []string {
	seen := make(map[string]bool)
	for _, p := range peerings {
		outVPC, inVPC := peeringTrafficVPCs(p, accountID)
		if outVPC != "" {
			seen[outVPC] = true
		}
		if inVPC != "" {
			seen[inVPC] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func countDirectionPeeringsPerVPC(peerings []ec2peerings.Connection, accountID string) (outCounts, inCounts map[string]int) {
	outCounts = make(map[string]int)
	inCounts = make(map[string]int)
	for _, p := range peerings {
		outVPC, inVPC := peeringTrafficVPCs(p, accountID)
		if outVPC != "" {
			outCounts[outVPC]++
		}
		if inVPC != "" {
			inCounts[inVPC]++
		}
	}
	return outCounts, inCounts
}

func sumVPCInstanceCosts(vpcID string, costs map[string]float64, instancesByVPC map[string][]string) float64 {
	var sum float64
	for _, instID := range instancesByVPC[vpcID] {
		sum += costs[instID]
	}
	return sum
}

func buildPeeringDetails(
	peerings []ec2peerings.Connection,
	inCosts, outCosts map[string]float64,
	instancesByVPC map[string][]string,
	outCounts, inCounts map[string]int,
	accountID string,
) []PeeringConnectionDetail {
	vpcOutTotal := make(map[string]float64)
	for vpc := range outCounts {
		vpcOutTotal[vpc] = sumVPCInstanceCosts(vpc, outCosts, instancesByVPC)
	}
	vpcInTotal := make(map[string]float64)
	for vpc := range inCounts {
		vpcInTotal[vpc] = sumVPCInstanceCosts(vpc, inCosts, instancesByVPC)
	}

	details := make([]PeeringConnectionDetail, 0, len(peerings))
	for _, p := range peerings {
		outVPC, inVPC := peeringTrafficVPCs(p, accountID)

		var outTotal, inTotal float64
		if outVPC != "" && outCounts[outVPC] > 0 {
			outTotal = vpcOutTotal[outVPC] / float64(outCounts[outVPC])
		}
		if inVPC != "" && inCounts[inVPC] > 0 {
			inTotal = vpcInTotal[inVPC] / float64(inCounts[inVPC])
		}

		note := peeringAttributionNote(p, outVPC, inVPC, outCounts[outVPC], inCounts[inVPC], accountID)

		details = append(details, PeeringConnectionDetail{
			PeeringID:       p.PeeringID,
			Name:            p.Name,
			Status:          p.Status,
			Region:          p.Region,
			LocalVPC:        p.LocalVPC,
			LocalCIDR:       p.LocalCIDR,
			PeerVPC:         p.PeerVPC,
			PeerCIDR:        p.PeerCIDR,
			PeerRegion:      p.PeerRegion,
			PeerAccount:     p.PeerAccount,
			LocalRole:       p.LocalRole,
			OutTotal:        outTotal,
			InTotal:         inTotal,
			OutVPC:          outVPC,
			InVPC:           inVPC,
			AttributionNote: note,
		})
	}
	return details
}

func peeringAttributionNote(
	p ec2peerings.Connection,
	outVPC, inVPC string,
	outPeerings, inPeerings int,
	accountID string,
) string {
	var parts []string
	if outPeerings > 1 {
		parts = append(parts, fmt.Sprintf(
			"Out is estimated: VPC %s has %d peerings; Out $ is split equally (÷%d) until flow-log bytes per pcx-* are wired in.",
			outVPC, outPeerings, outPeerings,
		))
	}
	if inPeerings > 1 {
		parts = append(parts, fmt.Sprintf(
			"In is estimated: VPC %s has %d peerings; In $ is split equally (÷%d) until flow-log bytes per pcx-* are wired in.",
			inVPC, inPeerings, inPeerings,
		))
	}
	if inVPC == "" && p.PeerAccount != "" && p.PeerAccount != accountID {
		parts = append(parts, "In-traffic receivers are in the peer account and are not shown here.")
	}
	return strings.Join(parts, " ")
}

// peeringTrafficVPCs returns which VPCs in this account bill Out vs In for this peering.
// Out-Bytes are charged on instances in the sending VPC; In-Bytes on the receiving VPC.
func peeringTrafficVPCs(p ec2peerings.Connection, accountID string) (outVPC, inVPC string) {
	if p.LocalRole == "requester" {
		outVPC = p.LocalVPC
		if p.PeerAccount == accountID {
			inVPC = p.PeerVPC
		}
		return outVPC, inVPC
	}
	inVPC = p.LocalVPC
	if p.PeerAccount == accountID {
		outVPC = p.PeerVPC
	}
	return outVPC, inVPC
}

func mapPeeringDetailsWithoutCosts(peerings []ec2peerings.Connection) []PeeringConnectionDetail {
	out := make([]PeeringConnectionDetail, len(peerings))
	for i, p := range peerings {
		out[i] = PeeringConnectionDetail{
			PeeringID:   p.PeeringID,
			Name:        p.Name,
			Status:      p.Status,
			Region:      p.Region,
			LocalVPC:    p.LocalVPC,
			LocalCIDR:   p.LocalCIDR,
			PeerVPC:     p.PeerVPC,
			PeerCIDR:    p.PeerCIDR,
			PeerRegion:  p.PeerRegion,
			PeerAccount: p.PeerAccount,
			LocalRole:   p.LocalRole,
		}
	}
	return out
}

func resourcePeriodStart(start, end string) string {
	endT, err := time.Parse("2006-01-02", end)
	if err != nil {
		return start
	}
	startT, err := time.Parse("2006-01-02", start)
	if err != nil {
		return start
	}
	maxStart := endT.AddDate(0, 0, -costExplorerResourceLookbackDays)
	if startT.Before(maxStart) {
		return maxStart.Format("2006-01-02")
	}
	return start
}

func peeringCostNote(requestedStart, resourceStart string) string {
	base := "AWS bills VpcPeering-Out/In-Bytes per EC2 instance, not per pcx-*; this dashboard allocates each VPC’s peering $ across its peerings (equal split when a VPC has more than one). " +
		"Byte-proportional split per connection requires VPC flow logs (traffic_path 4/5, peer CIDR filters) — not available via Cost Explorer alone. "
	if requestedStart == resourceStart {
		return base + "Costs use the full selected date range."
	}
	return base + fmt.Sprintf(
		"Resource-level CE data is limited to %s through today (max %d days).",
		resourceStart, costExplorerResourceLookbackDays,
	)
}

func listInstancesByVPC(ctx context.Context, client *ec2.Client, vpcIDs []string) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, vpcID := range vpcIDs {
		paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			}},
		})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("describe instances in %s: %w", vpcID, err)
			}
			for _, res := range page.Reservations {
				for _, inst := range res.Instances {
					if inst.State != nil && inst.State.Name == ec2types.InstanceStateNameTerminated {
						continue
					}
					id := aws.ToString(inst.InstanceId)
					if id != "" {
						result[vpcID] = append(result[vpcID], id)
					}
				}
			}
		}
	}
	return result, nil
}

func getResourceCosts(ctx context.Context, client *ce.Client, start, end string, filter types.Expression) (map[string]float64, error) {
	if err := withCESem(ctx); err != nil {
		return nil, err
	}
	defer releaseCESem()

	input := &ce.GetCostAndUsageWithResourcesInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
		Filter:      &filter,
		GroupBy: []types.GroupDefinition{{
			Type: types.GroupDefinitionTypeDimension,
			Key:  aws.String("RESOURCE_ID"),
		}},
	}
	totals := make(map[string]float64)
	var nextToken *string
	for {
		input.NextPageToken = nextToken
		out, err := client.GetCostAndUsageWithResources(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("get cost by resource: %w", err)
		}
		for _, result := range out.ResultsByTime {
			for _, group := range result.Groups {
				if len(group.Keys) == 0 {
					continue
				}
				id := strings.TrimSpace(group.Keys[0])
				amt, _ := parseAmount(group.Metrics)
				totals[id] += amt
			}
		}
		if out.NextPageToken == nil {
			break
		}
		nextToken = out.NextPageToken
	}
	return totals, nil
}
