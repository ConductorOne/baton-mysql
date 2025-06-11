package connector

import (
	"context"
	"fmt"
	"strings"

	"github.com/conductorone/baton-mysql/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

type userSyncer struct {
	resourceType  *v2.ResourceType
	client        *client.Client
	skipDbs       map[string]struct{}
	expandCols    map[string]struct{}
	collapseUsers bool
}

func (s *userSyncer) ResourceType(ctx context.Context) *v2.ResourceType {
	return s.resourceType
}

func (s *userSyncer) List(
	ctx context.Context,
	parentResourceID *v2.ResourceId,
	pToken *pagination.Token,
) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil || parentResourceID.ResourceType != resourceTypeServer.Id {
		return nil, "", nil, nil
	}

	users, nextPageToken, err := s.client.ListUsers(ctx, s.resourceType.Id, &client.Pager{Token: pToken.Token, Size: pToken.Size}, s.collapseUsers)
	if err != nil {
		return nil, "", nil, err
	}

	var ret []*v2.Resource
	for _, u := range users {
		var annos annotations.Annotations

		ut, err := rs.NewUserTrait(
			rs.WithUserProfile(map[string]interface{}{
				"user":       u.User,
				"host":       u.Host,
				"first_name": fmt.Sprintf("%s@%s", u.User, u.Host),
				"user_id":    fmt.Sprintf("%s@%s", u.User, u.Host),
			}),
			rs.WithUserLogin(u.User),
			rs.WithStatus(v2.UserTrait_Status_STATUS_ENABLED),
		)
		if err != nil {
			return nil, "", nil, err
		}
		annos.Update(ut)

		ret = append(ret, &v2.Resource{
			DisplayName: fmt.Sprintf("%s@%s", u.User, u.Host),
			Id: &v2.ResourceId{
				ResourceType: s.resourceType.Id,
				Resource:     u.GetID(),
			},
			Annotations:      annos,
			ParentResourceId: parentResourceID,
		})
	}

	return ret, nextPageToken, nil, nil
}

func (s *userSyncer) Entitlements(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	entitlements, err := getEntitlementsForResource(resource, s.client)
	if err != nil {
		return nil, "", nil, err
	}

	return entitlements, "", nil, nil
}

func (s *userSyncer) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	grants, err := grantsForUserOrRole(ctx, s.client, resource, s.skipDbs, s.expandCols, s.collapseUsers)
	if err != nil {
		return nil, "", nil, err
	}

	return grants, "", nil, nil
}

func newUserSyncer(c *client.Client, skipDbs map[string]struct{}, expandCols map[string]struct{}, collapseUsers bool) *userSyncer {
	return &userSyncer{
		resourceType:  resourceTypeUser,
		client:        c,
		skipDbs:       skipDbs,
		expandCols:    expandCols,
		collapseUsers: collapseUsers,
	}
}

// Account provisioning With random password.
func (b *userSyncer) CreateAccountCapabilityDetails(
	_ context.Context,
) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_RANDOM_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_RANDOM_PASSWORD,
	}, nil, nil
}

func (o *userSyncer) CreateAccount(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	credentialOptions *v2.CredentialOptions,
) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	profile := accountInfo.GetProfile().AsMap()

	username, ok := profile["username"].(string)
	if !ok {
		return nil, nil, nil, fmt.Errorf("missing or invalid 'username' in profile")
	}

	host, err := o.client.GetHost(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get host: %w", err)
	}

	generatedPassword, err := generateCredentials(credentialOptions)
	if err != nil {
		return nil, nil, nil, err
	}

	userStr := fmt.Sprintf("%s@%s", username, host)
	err = o.client.CreateUser(ctx, userStr, generatedPassword)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create user failed: %w", err)
	}

	// Build resource
	user := &client.User{
		User: username,
		Host: host,
	}
	userResource, err := parseIntoUserResource(user, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to build resource: %w", err)
	}

	passResult := &v2.PlaintextData{
		Name:  "password",
		Bytes: []byte(generatedPassword),
	}

	caResponse := &v2.CreateAccountResponse_SuccessResult{
		Resource: userResource,
	}

	return caResponse, []*v2.PlaintextData{passResult}, nil, nil
}

func parseIntoUserResource(user *client.User, parent *v2.ResourceId) (*v2.Resource, error) {
	ut, err := rs.NewUserTrait(
		rs.WithUserProfile(map[string]interface{}{
			"user":       user.User,
			"host":       user.Host,
			"first_name": fmt.Sprintf("%s@%s", user.User, user.Host),
			"user_id":    fmt.Sprintf("%s@%s", user.User, user.Host),
		}),
		rs.WithUserLogin(user.User),
		rs.WithStatus(v2.UserTrait_Status_STATUS_ENABLED),
	)
	if err != nil {
		return nil, err
	}

	annos := annotations.Annotations{}
	annos.Update(ut)

	return &v2.Resource{
		DisplayName: fmt.Sprintf("%s@%s", user.User, user.Host),
		Id: &v2.ResourceId{
			ResourceType: resourceTypeUser.Id,
			Resource:     user.GetID(),
		},
		Annotations:      annos,
		ParentResourceId: parent,
	}, nil
}

func (s *userSyncer) Delete(ctx context.Context, resourceId *v2.ResourceId) (annotations.Annotations, error) {
	if resourceId.ResourceType != resourceTypeUser.Id {
		return nil, fmt.Errorf("baton-mysql: non-user resource passed to user delete")
	}
	fmt.Println("resourceId", resourceId)
	userID := strings.TrimSpace(strings.Split(resourceId.Resource, ":")[1])
	parts := strings.Split(userID, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("baton-mysql: invalid user ID format, expected 'user@host'")
	}
	user, host := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

	userStr := fmt.Sprintf("%s@%s", user, host)
	err := s.client.DropUser(ctx, userStr)
	if err != nil {
		return nil, fmt.Errorf("drop user failed: %w", err)
	}

	return nil, nil
}
