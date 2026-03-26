package cloud

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/egoscale/v3/credentials"
)

var zoneEndpoints = map[string]v3.Endpoint{
	"ch-gva-2": v3.CHGva2,
	"ch-dk-2":  v3.CHDk2,
	"de-fra-1": v3.DEFra1,
	"de-muc-1": v3.DEMuc1,
	"at-vie-1": v3.ATVie1,
	"at-vie-2": v3.ATVie2,
	"bg-sof-1": v3.BGSof1,
}

// Client wraps the Exoscale v3 API client with zone-scoped helpers.
type Client struct {
	client *v3.Client
}

// NewClient creates a Client bound to the given Exoscale zone.
// Credentials are loaded from the EXOSCALE_API_KEY and EXOSCALE_API_SECRET
// environment variables.
func NewClient(zone string) (*Client, error) {
	endpoint, ok := zoneEndpoints[zone]
	if !ok {
		return nil, fmt.Errorf("unsupported Exoscale zone %q", zone)
	}
	creds := credentials.NewEnvCredentials()
	c, err := v3.NewClient(creds, v3.ClientOptWithEndpoint(endpoint))
	if err != nil {
		return nil, fmt.Errorf("creating Exoscale client: %w", err)
	}
	return &Client{client: c}, nil
}

// --- Security Groups ---

// FindSecurityGroupByName returns the ID of the first security group with the
// given name.  Returns ("", false, nil) when none is found.
func (c *Client) FindSecurityGroupByName(ctx context.Context, name string) (string, bool, error) {
	resp, err := c.client.ListSecurityGroups(ctx)
	if err != nil {
		return "", false, fmt.Errorf("listing security groups: %w", err)
	}
	for _, sg := range resp.SecurityGroups {
		if sg.Name == name {
			return sg.ID.String(), true, nil
		}
	}
	return "", false, nil
}

// CreateSecurityGroup creates a new security group and returns its ID.
func (c *Client) CreateSecurityGroup(ctx context.Context, name, description string) (string, error) {
	op, err := c.client.CreateSecurityGroup(ctx, v3.CreateSecurityGroupRequest{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return "", fmt.Errorf("creating security group %q: %w", name, err)
	}
	op, err = c.wait(ctx, op)
	if err != nil {
		return "", fmt.Errorf("waiting for security group creation: %w", err)
	}
	return op.Reference.ID.String(), nil
}

// GetSecurityGroup retrieves a security group by ID.
// Returns (nil, false, nil) when the resource does not exist.
func (c *Client) GetSecurityGroup(ctx context.Context, id string) (*v3.SecurityGroup, bool, error) {
	uid, err := v3.ParseUUID(id)
	if err != nil {
		return nil, false, fmt.Errorf("parsing UUID %q: %w", id, err)
	}
	sg, err := c.client.GetSecurityGroup(ctx, uid)
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("getting security group: %w", err)
	}
	return sg, true, nil
}

// DeleteSecurityGroup deletes a security group by ID.  Not-found errors are
// silently ignored.
func (c *Client) DeleteSecurityGroup(ctx context.Context, id string) error {
	uid, err := v3.ParseUUID(id)
	if err != nil {
		return fmt.Errorf("parsing UUID %q: %w", id, err)
	}
	op, err := c.client.DeleteSecurityGroup(ctx, uid)
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("deleting security group: %w", err)
	}
	_, err = c.wait(ctx, op)
	return err
}

// IngressRule describes an ingress rule to be added to a security group.
type IngressRule struct {
	Protocol  string // "tcp", "udp", "icmp", …
	StartPort int64
	EndPort   int64
	Network   string // CIDR, e.g. "0.0.0.0/0"
}

// AddIngressRule adds an ingress rule to the specified security group.
func (c *Client) AddIngressRule(ctx context.Context, sgID string, rule IngressRule) error {
	uid, err := v3.ParseUUID(sgID)
	if err != nil {
		return fmt.Errorf("parsing UUID %q: %w", sgID, err)
	}
	op, err := c.client.AddRuleToSecurityGroup(ctx, uid, v3.AddRuleToSecurityGroupRequest{
		FlowDirection: v3.AddRuleToSecurityGroupRequestFlowDirectionIngress,
		Protocol:      v3.AddRuleToSecurityGroupRequestProtocol(rule.Protocol),
		Network:       rule.Network,
		StartPort:     rule.StartPort,
		EndPort:       rule.EndPort,
	})
	if err != nil {
		return fmt.Errorf("adding ingress rule: %w", err)
	}
	_, err = c.wait(ctx, op)
	return err
}

// HasIngressRule reports whether sg already contains a matching ingress rule.
func HasIngressRule(sg *v3.SecurityGroup, rule IngressRule) bool {
	for _, r := range sg.Rules {
		if r.FlowDirection == v3.SecurityGroupRuleFlowDirectionIngress &&
			string(r.Protocol) == rule.Protocol &&
			r.StartPort == rule.StartPort &&
			r.EndPort == rule.EndPort &&
			r.Network == rule.Network {
			return true
		}
	}
	return false
}

// --- Elastic IPs ---

// CreateElasticIP allocates a new Elastic IP.  It returns (id, ipAddress, err).
func (c *Client) CreateElasticIP(ctx context.Context, description string) (string, string, error) {
	op, err := c.client.CreateElasticIP(ctx, v3.CreateElasticIPRequest{
		Description: description,
	})
	if err != nil {
		return "", "", fmt.Errorf("creating elastic IP: %w", err)
	}
	op, err = c.wait(ctx, op)
	if err != nil {
		return "", "", fmt.Errorf("waiting for elastic IP creation: %w", err)
	}
	eip, err := c.client.GetElasticIP(ctx, op.Reference.ID)
	if err != nil {
		return "", "", fmt.Errorf("getting elastic IP details: %w", err)
	}
	return eip.ID.String(), eip.IP, nil
}

// GetElasticIP returns the IP address for an Elastic IP.
// Returns ("", false, nil) when the resource does not exist.
func (c *Client) GetElasticIP(ctx context.Context, id string) (string, bool, error) {
	uid, err := v3.ParseUUID(id)
	if err != nil {
		return "", false, fmt.Errorf("parsing UUID %q: %w", id, err)
	}
	eip, err := c.client.GetElasticIP(ctx, uid)
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("getting elastic IP: %w", err)
	}
	return eip.IP, true, nil
}

// AttachElasticIPToInstance associates an Elastic IP with a compute instance so
// that inbound traffic to the EIP is routed to that instance.  The call is
// idempotent: if the EIP is already attached to the given instance the
// operation succeeds without error.
func (c *Client) AttachElasticIPToInstance(ctx context.Context, instanceID, eipID string) error {
	instanceUID, err := v3.ParseUUID(instanceID)
	if err != nil {
		return fmt.Errorf("parsing instance UUID %q: %w", instanceID, err)
	}
	eipUID, err := v3.ParseUUID(eipID)
	if err != nil {
		return fmt.Errorf("parsing EIP UUID %q: %w", eipID, err)
	}
	// The v3 API is EIP-centric: you call AttachInstanceToElasticIP on the EIP
	// and pass the instance in the request body.
	op, err := c.client.AttachInstanceToElasticIP(ctx, eipUID, v3.AttachInstanceToElasticIPRequest{
		Instance: &v3.InstanceTarget{ID: instanceUID},
	})
	if err != nil {
		return fmt.Errorf("attaching EIP %s to instance %s: %w", eipID, instanceID, err)
	}
	_, err = c.wait(ctx, op)
	return err
}

// DeleteElasticIP releases an Elastic IP by ID.  Not-found errors are silently
// ignored.
func (c *Client) DeleteElasticIP(ctx context.Context, id string) error {
	uid, err := v3.ParseUUID(id)
	if err != nil {
		return fmt.Errorf("parsing UUID %q: %w", id, err)
	}
	op, err := c.client.DeleteElasticIP(ctx, uid)
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("deleting elastic IP: %w", err)
	}
	_, err = c.wait(ctx, op)
	return err
}

// --- Instances ---

// CreateInstanceParams collects the parameters needed to provision an instance.
type CreateInstanceParams struct {
	Name              string
	TemplateName      string // name or UUID
	InstanceTypeName  string // "family.size" or UUID
	DiskSize          int64
	SSHKeyName        string
	SecurityGroupIDs  []string
	AntiAffinityGroup string // name or UUID; empty to skip
	EnableIPv6        bool
	// UserData is the base64-encoded cloud-init payload injected at boot time.
	// The caller is responsible for base64-encoding the raw script before setting
	// this field, as the Exoscale API expects the encoded form directly.
	UserData string
}

// InstanceInfo holds the observed state of a compute instance.
type InstanceInfo struct {
	ID       string
	State    string // "running", "stopped", "starting", …
	PublicIP string
	IPv6     string
}

// CreateInstance provisions a new compute instance and blocks until it is
// created.  It returns the instance ID.
func (c *Client) CreateInstance(ctx context.Context, params CreateInstanceParams) (string, error) {
	templateID, err := c.resolveTemplate(ctx, params.TemplateName)
	if err != nil {
		return "", err
	}
	instanceTypeID, err := c.resolveInstanceType(ctx, params.InstanceTypeName)
	if err != nil {
		return "", err
	}

	req := v3.CreateInstanceRequest{
		Name:         params.Name,
		DiskSize:     params.DiskSize,
		Template:     &v3.Template{ID: templateID},
		InstanceType: &v3.InstanceType{ID: instanceTypeID},
		SSHKey:       &v3.SSHKey{Name: params.SSHKeyName},
	}

	for _, sgID := range params.SecurityGroupIDs {
		if sgID == "" {
			continue
		}
		uid, err := v3.ParseUUID(sgID)
		if err != nil {
			return "", fmt.Errorf("parsing security group ID %q: %w", sgID, err)
		}
		req.SecurityGroups = append(req.SecurityGroups, v3.SecurityGroup{ID: uid})
	}

	if params.AntiAffinityGroup != "" {
		aagID, err := c.resolveAntiAffinityGroup(ctx, params.AntiAffinityGroup)
		if err != nil {
			return "", err
		}
		req.AntiAffinityGroups = append(req.AntiAffinityGroups, v3.AntiAffinityGroup{ID: aagID})
	}

	if params.EnableIPv6 {
		req.PublicIPAssignment = v3.PublicIPAssignmentDual
	}

	if params.UserData != "" {
		req.UserData = params.UserData
	}

	op, err := c.client.CreateInstance(ctx, req)
	if err != nil {
		return "", fmt.Errorf("creating instance %q: %w", params.Name, err)
	}
	op, err = c.wait(ctx, op)
	if err != nil {
		return "", fmt.Errorf("waiting for instance creation: %w", err)
	}
	return op.Reference.ID.String(), nil
}

// GetInstance returns the current state of an instance.  Returns nil when the
// instance does not exist.
func (c *Client) GetInstance(ctx context.Context, id string) (*InstanceInfo, error) {
	uid, err := v3.ParseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("parsing UUID %q: %w", id, err)
	}
	inst, err := c.client.GetInstance(ctx, uid)
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting instance: %w", err)
	}
	info := &InstanceInfo{
		ID:    inst.ID.String(),
		State: string(inst.State),
		IPv6:  inst.Ipv6Address,
	}
	if inst.PublicIP != nil {
		info.PublicIP = inst.PublicIP.String()
	}
	return info, nil
}

// DeleteInstance terminates a compute instance by ID.  Not-found errors are
// silently ignored.
func (c *Client) DeleteInstance(ctx context.Context, id string) error {
	uid, err := v3.ParseUUID(id)
	if err != nil {
		return fmt.Errorf("parsing UUID %q: %w", id, err)
	}
	op, err := c.client.DeleteInstance(ctx, uid)
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("deleting instance: %w", err)
	}
	_, err = c.wait(ctx, op)
	return err
}

// --- Internal helpers ---

func (c *Client) wait(ctx context.Context, op *v3.Operation) (*v3.Operation, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	return c.client.Wait(ctx, op, v3.OperationStateSuccess)
}

func (c *Client) resolveTemplate(ctx context.Context, nameOrID string) (v3.UUID, error) {
	if uid, err := v3.ParseUUID(nameOrID); err == nil {
		return uid, nil
	}
	for _, vis := range []v3.ListTemplatesVisibility{
		v3.ListTemplatesVisibilityPublic,
		v3.ListTemplatesVisibilityPrivate,
	} {
		resp, err := c.client.ListTemplates(ctx, v3.ListTemplatesWithVisibility(vis))
		if err != nil {
			return "", fmt.Errorf("listing templates (visibility=%s): %w", vis, err)
		}
		for _, t := range resp.Templates {
			if t.Name == nameOrID {
				return t.ID, nil
			}
		}
	}
	return "", fmt.Errorf("template %q not found", nameOrID)
}

func (c *Client) resolveInstanceType(ctx context.Context, nameOrID string) (v3.UUID, error) {
	if uid, err := v3.ParseUUID(nameOrID); err == nil {
		return uid, nil
	}
	resp, err := c.client.ListInstanceTypes(ctx)
	if err != nil {
		return "", fmt.Errorf("listing instance types: %w", err)
	}
	parts := strings.SplitN(nameOrID, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid instance type %q (expected family.size, e.g. standard.medium)", nameOrID)
	}
	for _, it := range resp.InstanceTypes {
		if string(it.Family) == parts[0] && string(it.Size) == parts[1] {
			return it.ID, nil
		}
	}
	return "", fmt.Errorf("instance type %q not found", nameOrID)
}

func (c *Client) resolveAntiAffinityGroup(ctx context.Context, nameOrID string) (v3.UUID, error) {
	if uid, err := v3.ParseUUID(nameOrID); err == nil {
		return uid, nil
	}
	resp, err := c.client.ListAntiAffinityGroups(ctx)
	if err != nil {
		return "", fmt.Errorf("listing anti-affinity groups: %w", err)
	}
	for _, aag := range resp.AntiAffinityGroups {
		if aag.Name == nameOrID {
			return aag.ID, nil
		}
	}
	return "", fmt.Errorf("anti-affinity group %q not found", nameOrID)
}
