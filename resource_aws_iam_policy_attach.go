package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceAwsIamPolicyAttachment() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsIamPolicyAttachmentCreate,
		Read:   resourceAwsIamPolicyAttachmentRead,
		Update: resourceAwsIamPolicyAttachmentUpdate,
		Delete: resourceAwsIamPolicyAttachmentDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"users": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"roles": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"groups": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"policy_arn": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceAwsIamPolicyAttachmentCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).iamconn

	name := d.Get("name").(string)
	arn := d.Get("policy_arn").(string)
	users := expandStringList(d.Get("users").(*schema.Set).List())
	roles := expandStringList(d.Get("roles").(*schema.Set).List())
	groups := expandStringList(d.Get("groups").(*schema.Set).List())

	if users == nil && roles == nil && groups == nil {
		return fmt.Errorf("[WARN] No Users, Roles, or Groups specified for %s", name)
	} else {
		var userErr, roleErr, groupErr error
		if users != nil {
			userErr = attachPolicyToUsers(conn, users, arn)
		}
		if roles != nil {
			roleErr = attachPolicyToRoles(conn, roles, arn)
		}
		if groups != nil {
			groupErr = attachPolicyToGroups(conn, groups, arn)
		}
		if userErr != nil || roleErr != nil || groupErr != nil {
			return fmt.Errorf("[WARN] Error attaching policy with IAM Policy Attach (%s), error:\n users - %v\n roles - %v\n groups - %v", name, userErr, roleErr, groupErr)
		}
	}
	d.SetId(d.Get("name").(string))
	return resourceAwsIamPolicyAttachmentRead(d, meta)
}

func resourceAwsIamPolicyAttachmentRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).iamconn
	arn := d.Get("policy_arn").(string)
	name := d.Get("name").(string)

	_, err := conn.GetPolicy(&iam.GetPolicyInput{
		PolicyARN: aws.String(arn),
	})

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "NoSuchIdentity" {
				d.SetId("")
				return nil
			}
		}
		return err
	}

	policyEntities, err := conn.ListEntitiesForPolicy(&iam.ListEntitiesForPolicyInput{
		PolicyARN: aws.String(arn),
	})

	if err != nil {
		return err
	}

	ul := make([]string, 0, len(policyEntities.PolicyUsers))
	rl := make([]string, 0, len(policyEntities.PolicyRoles))
	gl := make([]string, 0, len(policyEntities.PolicyGroups))

	for _, u := range policyEntities.PolicyUsers {
		ul = append(ul, *u.UserName)
	}

	for _, r := range policyEntities.PolicyRoles {
		rl = append(rl, *r.RoleName)
	}

	for _, g := range policyEntities.PolicyGroups {
		gl = append(gl, *g.GroupName)
	}

	userErr := d.Set("users", ul)
	roleErr := d.Set("roles", rl)
	groupErr := d.Set("groups", gl)

	if userErr != nil || roleErr != nil || groupErr != nil {
		return fmt.Errorf("[WARN} Error setting user, role, or group list from IAM Policy Attach (%s):\n user error - %s\n role error - %s\n group error - %s", name, userErr, roleErr, groupErr)
	}

	return nil
}
func resourceAwsIamPolicyAttachmentUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).iamconn
	name := d.Get("name").(string)
	var userErr, roleErr, groupErr error

	if d.HasChange("users") {
		userErr = updateUsers(conn, d, meta)
	}
	if d.HasChange("roles") {
		roleErr = updateRoles(conn, d, meta)
	}
	if d.HasChange("groups") {
		groupErr = updateGroups(conn, d, meta)
	}
	if userErr != nil || roleErr != nil || groupErr != nil {
		return fmt.Errorf("[WARN] Error updating user, role, or group list from IAM Policy Attach (%s):\n user error - %s\n role error - %s\n group error - %s", name, userErr, roleErr, groupErr)
	}
	return resourceAwsIamPolicyAttachmentRead(d, meta)
}

func resourceAwsIamPolicyAttachmentDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).iamconn
	name := d.Get("name").(string)
	arn := d.Get("policy_arn").(string)
	users := expandStringList(d.Get("users").(*schema.Set).List())
	roles := expandStringList(d.Get("roles").(*schema.Set).List())
	groups := expandStringList(d.Get("groups").(*schema.Set).List())

	var userErr, roleErr, groupErr error
	if users != nil {
		userErr = detachPolicyFromUsers(conn, users, arn)
	}
	if roles != nil {
		roleErr = detachPolicyFromRoles(conn, roles, arn)
	}
	if groups != nil {
		groupErr = detachPolicyFromGroups(conn, groups, arn)
	}
	if userErr != nil || roleErr != nil || groupErr != nil {
		return fmt.Errorf("[WARN] Error removing user, role, or group list from IAM Policy Detach (%s), error:\n users - %v\n roles - %v\n groups - %v", name, userErr, roleErr, groupErr)
	}
	return nil
}
func attachPolicyToUsers(conn *iam.IAM, users []*string, arn string) error {
	for _, u := range users {
		_, err := conn.AttachUserPolicy(&iam.AttachUserPolicyInput{
			UserName:  u,
			PolicyARN: aws.String(arn),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func attachPolicyToRoles(conn *iam.IAM, roles []*string, arn string) error {
	for _, r := range roles {
		_, err := conn.AttachRolePolicy(&iam.AttachRolePolicyInput{
			RoleName:  r,
			PolicyARN: aws.String(arn),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func attachPolicyToGroups(conn *iam.IAM, groups []*string, arn string) error {
	for _, g := range groups {
		_, err := conn.AttachGroupPolicy(&iam.AttachGroupPolicyInput{
			GroupName: g,
			PolicyARN: aws.String(arn),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func updateUsers(conn *iam.IAM, d *schema.ResourceData, meta interface{}) error {
	arn := d.Get("policy_arn").(string)
	o, n := d.GetChange("users")
	if o == nil {
		o = new(schema.Set)
	}
	if n == nil {
		n = new(schema.Set)
	}
	os := o.(*schema.Set)
	ns := n.(*schema.Set)
	remove := expandStringList(os.Difference(ns).List())
	add := expandStringList(ns.Difference(os).List())

	if rErr := detachPolicyFromUsers(conn, remove, arn); rErr != nil {
		return rErr
	}
	if aErr := attachPolicyToUsers(conn, add, arn); aErr != nil {
		return aErr
	}
	return nil
}
func updateRoles(conn *iam.IAM, d *schema.ResourceData, meta interface{}) error {
	arn := d.Get("policy_arn").(string)
	o, n := d.GetChange("roles")
	if o == nil {
		o = new(schema.Set)
	}
	if n == nil {
		n = new(schema.Set)
	}
	os := o.(*schema.Set)
	ns := n.(*schema.Set)
	remove := expandStringList(os.Difference(ns).List())
	add := expandStringList(ns.Difference(os).List())

	if rErr := detachPolicyFromRoles(conn, remove, arn); rErr != nil {
		return rErr
	}
	if aErr := attachPolicyToRoles(conn, add, arn); aErr != nil {
		return aErr
	}
	return nil
}
func updateGroups(conn *iam.IAM, d *schema.ResourceData, meta interface{}) error {
	arn := d.Get("policy_arn").(string)
	o, n := d.GetChange("groups")
	if o == nil {
		o = new(schema.Set)
	}
	if n == nil {
		n = new(schema.Set)
	}
	os := o.(*schema.Set)
	ns := n.(*schema.Set)
	remove := expandStringList(os.Difference(ns).List())
	add := expandStringList(ns.Difference(os).List())

	if rErr := detachPolicyFromGroups(conn, remove, arn); rErr != nil {
		return rErr
	}
	if aErr := attachPolicyToGroups(conn, add, arn); aErr != nil {
		return aErr
	}
	return nil

}
func detachPolicyFromUsers(conn *iam.IAM, users []*string, arn string) error {
	for _, u := range users {
		_, err := conn.DetachUserPolicy(&iam.DetachUserPolicyInput{
			UserName:  u,
			PolicyARN: aws.String(arn),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func detachPolicyFromRoles(conn *iam.IAM, roles []*string, arn string) error {
	for _, r := range roles {
		_, err := conn.DetachRolePolicy(&iam.DetachRolePolicyInput{
			RoleName:  r,
			PolicyARN: aws.String(arn),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
func detachPolicyFromGroups(conn *iam.IAM, groups []*string, arn string) error {
	for _, g := range groups {
		_, err := conn.DetachGroupPolicy(&iam.DetachGroupPolicyInput{
			GroupName: g,
			PolicyARN: aws.String(arn),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
