// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"testing"

	"github.com/juju/juju/api/jujuclient"
	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/juju/osenv"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func credentialTestClient(t *testing.T, user string) (*credentialsClient, *MockConnection) {
	t.Helper()
	ctrl := gomock.NewController(t)
	shared := NewMockSharedClient(ctrl)
	conn := NewMockConnection(ctrl)
	shared.EXPECT().GetConnection(gomock.Any(), gomock.Nil()).Return(conn, nil).AnyTimes()
	conn.EXPECT().Close().Return(nil).AnyTimes()
	conn.EXPECT().AuthTag().Return(names.NewUserTag(user)).AnyTimes()
	return newCredentialsClient(shared), conn
}

// TestCredentialReadOwnership verifies that successful controller credential reads
// return the authenticated owner for both legacy and owner-aware inputs. API errors,
// missing results, and mismatched credential names must fail without returning a credential.
func TestCredentialReadOwnership(t *testing.T) {
	content := params.CredentialContentResult{Result: &params.ControllerCredentialInfo{
		Content: params.CredentialContent{
			Name: "prod", Cloud: "openstack", AuthType: "userpass", Attributes: map[string]string{"password": "old"},
		},
	}}
	for _, tc := range []struct {
		name   string
		owner  string
		result params.CredentialContentResult
		error  string
	}{
		{name: "adopt legacy owner", result: content},
		{name: "preserve recorded owner", owner: "bob@external", result: content},
		{name: "not found", result: params.CredentialContentResult{Error: &params.Error{Message: "credential not found", Code: "not found"}}, error: "credential not found"},
		{name: "permission denied", result: params.CredentialContentResult{Error: &params.Error{Message: "permission denied"}}, error: "permission denied"},
		{name: "missing result", error: "missing result"},
		{name: "wrong credential", result: params.CredentialContentResult{Result: &params.ControllerCredentialInfo{
			Content: params.CredentialContent{Name: "different"},
		}}, error: "not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, conn := credentialTestClient(t, "bob@external")
			conn.EXPECT().BestFacadeVersion("Cloud").Return(7)
			conn.EXPECT().APICall(gomock.Any(), "Cloud", 7, "", "CredentialContents", gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, _ int, _, _ string, _, response interface{}) error {
					response.(*params.CredentialContentResults).Results = []params.CredentialContentResult{tc.result}
					return nil
				})
			response, err := client.ReadCredential(context.Background(), ReadCredentialInput{
				CloudName: "openstack", Name: "prod", Owner: tc.owner, ControllerCredential: true,
			})
			if tc.error != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.error)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "bob@external", response.Owner)
			require.Equal(t, "old", response.CloudCredential.Attributes()["password"])
		})
	}
}

// TestCredentialOwnerClientOnly verifies that creating, updating, and reading local
// credentials never records a Juju owner. Different authenticated users can read the
// same updated local credential, and deletion removes it from the client store.
func TestCredentialOwnerClientOnly(t *testing.T) {
	jujuDataHomeLock.Lock()
	t.Cleanup(jujuDataHomeLock.Unlock)
	oldHome := osenv.SetJujuXDGDataHome(t.TempDir())
	t.Cleanup(func() { osenv.SetJujuXDGDataHome(oldHome) })

	store := jujuclient.NewFileClientStore()
	require.NoError(t, store.UpdateCredential("openstack", jujucloud.CloudCredential{
		AuthCredentials: map[string]jujucloud.Credential{
			"prod": jujucloud.NewNamedCredential("prod", "userpass", map[string]string{"password": "old"}, false),
		},
	}))

	client, conn := credentialTestClient(t, "bob")
	conn.EXPECT().BestFacadeVersion("Cloud").Return(7).AnyTimes()
	conn.EXPECT().APICall(gomock.Any(), "Cloud", 7, "", "Cloud", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ int, _, _ string, _, response interface{}) error {
			response.(*params.CloudResults).Results = []params.CloudResult{{
				Cloud: &params.Cloud{AuthTypes: []string{"userpass"}},
			}}
			return nil
		}).Times(2)

	created, err := client.CreateCredential(context.Background(), CreateCredentialInput{
		CloudName: "openstack", Name: "prod", AuthType: "userpass",
		ClientCredential: true, Attributes: map[string]string{"password": "old"},
	})
	require.NoError(t, err)
	require.Empty(t, created.Owner)

	updated, err := client.UpdateCredential(context.Background(), UpdateCredentialInput{
		CloudName: "openstack", Name: "prod", AuthType: "userpass",
		ClientCredential: true, Attributes: map[string]string{"password": "new"},
	})
	require.NoError(t, err)
	require.Empty(t, updated.Owner)

	for _, user := range []string{"bob", "alice"} {
		client, conn := credentialTestClient(t, user)
		conn.EXPECT().BestFacadeVersion("Cloud").Return(7)
		response, err := client.ReadCredential(context.Background(), ReadCredentialInput{
			CloudName: "openstack", Name: "prod", ClientCredential: true,
		})
		require.NoError(t, err)
		require.Empty(t, response.Owner)
		require.Equal(t, "new", response.CloudCredential.Attributes()["password"])
	}

	require.NoError(t, client.DestroyCredential(context.Background(), DestroyCredentialInput{
		CloudName: "openstack", Name: "prod", ClientCredential: true,
	}))

	credentials, err := store.AllCredentials()
	require.NoError(t, err)
	require.NotContains(t, credentials["openstack"].AuthCredentials, "prod")
}
