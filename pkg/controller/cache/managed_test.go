/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"context"
	"strconv"
	"testing"

	redisv1 "cloud.google.com/go/redis/apiv1"
	"github.com/google/go-cmp/cmp"
	gax "github.com/googleapis/gax-go"
	"github.com/pkg/errors"
	redisv1pb "google.golang.org/genproto/googleapis/cloud/redis/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/provider-gcp/apis/cache/v1beta1"
	"github.com/crossplane/provider-gcp/pkg/clients/cloudmemorystore"
	"github.com/crossplane/provider-gcp/pkg/clients/cloudmemorystore/fake"
)

const (
	namespace     = "cool-namespace"
	region        = "us-cool1"
	project       = "coolProject"
	instanceName  = "claimns-claimname-8sdh3"
	qualifiedName = "projects/" + project + "/locations/" + region + "/instances/" + instanceName
	memorySizeGB  = 1
	host          = "172.16.0.1"
	port          = 6379

	connectionSecretName = "cool-connection-secret"
)

var (
	authorizedNetwork = "default"
	connectMode       = "DIRECT_PEERING"
	errorBoom         = errors.New("boom")
	redisConfigs      = map[string]string{"cool": "socool"}
)

type strange struct {
	resource.Managed
}

type instanceModifier func(*v1beta1.CloudMemorystoreInstance)

func withConditions(c ...xpv1.Condition) instanceModifier {
	return func(i *v1beta1.CloudMemorystoreInstance) { i.Status.SetConditions(c...) }
}

func withState(s string) instanceModifier {
	return func(i *v1beta1.CloudMemorystoreInstance) { i.Status.AtProvider.State = s }
}

func withFullName(name string) instanceModifier {
	return func(i *v1beta1.CloudMemorystoreInstance) { i.Status.AtProvider.Name = name }
}

func withHost(e string) instanceModifier {
	return func(i *v1beta1.CloudMemorystoreInstance) { i.Status.AtProvider.Host = e }
}

func withPort(p int) instanceModifier {
	return func(i *v1beta1.CloudMemorystoreInstance) { i.Status.AtProvider.Port = int32(p) }
}

func withTier(tier string) instanceModifier {
	return func(i *v1beta1.CloudMemorystoreInstance) { i.Spec.ForProvider.Tier = tier }
}

func instance(im ...instanceModifier) *v1beta1.CloudMemorystoreInstance {
	i := &v1beta1.CloudMemorystoreInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       instanceName,
			Finalizers: []string{},
			Annotations: map[string]string{
				meta.AnnotationKeyExternalName: instanceName,
			},
		},
		Spec: v1beta1.CloudMemorystoreInstanceSpec{
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: namespace,
					Name:      connectionSecretName,
				},
			},
			ForProvider: v1beta1.CloudMemorystoreInstanceParameters{
				MemorySizeGB:      memorySizeGB,
				RedisConfigs:      redisConfigs,
				AuthorizedNetwork: &authorizedNetwork,
				ConnectMode:       &connectMode,
			},
		},
	}

	for _, m := range im {
		m(i)
	}

	return i
}

var _ managed.ExternalClient = &external{}
var _ managed.ExternalConnecter = &connecter{}

func TestObserve(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg          resource.Managed
		observation managed.ExternalObservation
		err         error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"ObservedInstanceAvailable": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						State: redisv1pb.Instance_READY,
						Host:  host,
						Port:  port,
						Name:  qualifiedName,
					}, nil
				}},
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(
					withConditions(xpv1.Available()),
					withState(cloudmemorystore.StateReady),
					withHost(host),
					withPort(port),
					withFullName(qualifiedName),
					withTier(redisv1pb.Instance_TIER_UNSPECIFIED.String())),
				observation: managed.ExternalObservation{
					ResourceExists: true,
					ConnectionDetails: managed.ConnectionDetails{
						xpv1.ResourceCredentialsSecretEndpointKey: []byte(host),
						xpv1.ResourceCredentialsSecretPortKey:     []byte(strconv.Itoa(port)),
					},
				},
			},
		},
		"ObservedInstanceCreating": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						State: redisv1pb.Instance_CREATING,
						Name:  qualifiedName,
					}, nil
				}},
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(
					withConditions(xpv1.Creating()),
					withState(cloudmemorystore.StateCreating),
					withFullName(qualifiedName),
					withTier(redisv1pb.Instance_TIER_UNSPECIFIED.String())),
				observation: managed.ExternalObservation{
					ResourceExists:    true,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"ObservedInstanceDeleting": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						State: redisv1pb.Instance_DELETING,
						Name:  qualifiedName,
					}, nil
				}},
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(
					withConditions(xpv1.Deleting()),
					withState(cloudmemorystore.StateDeleting),
					withFullName(qualifiedName),
					withTier(redisv1pb.Instance_TIER_UNSPECIFIED.String())),
				observation: managed.ExternalObservation{
					ResourceExists:    true,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"ObservedInstanceDoesNotExist": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return nil, status.Error(codes.NotFound, "wat")
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:          instance(),
				observation: managed.ExternalObservation{ResourceExists: false},
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotInstance),
			},
		},
		"FailedToGetInstance": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return nil, errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(),
				err: errors.Wrap(errorBoom, errGetInstance),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Observe(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.observation, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Observe(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Observe(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg       resource.Managed
		creation managed.ExternalCreation
		err      error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"CreatedInstance": {
			client: &external{cms: &fake.MockClient{
				MockCreateInstance: func(_ context.Context, _ *redisv1pb.CreateInstanceRequest, _ ...gax.CallOption) (*redisv1.CreateInstanceOperation, error) {
					return nil, nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(withConditions(xpv1.Creating())),
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotInstance),
			},
		},
		"FailedToCreateInstance": {
			client: &external{cms: &fake.MockClient{
				MockCreateInstance: func(_ context.Context, _ *redisv1pb.CreateInstanceRequest, _ ...gax.CallOption) (*redisv1.CreateInstanceOperation, error) {
					return nil, errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(withConditions(xpv1.Creating())),
				err: errors.Wrap(errorBoom, errCreateInstance),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Create(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.creation, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Create(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Create(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg     resource.Managed
		update managed.ExternalUpdate
		err    error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"UpdatedInstance": {
			client: &external{cms: &fake.MockClient{
				MockUpdateInstance: func(_ context.Context, _ *redisv1pb.UpdateInstanceRequest, _ ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error) {
					return nil, nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(withConditions()),
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotInstance),
			},
		},
		"FailedToUpdateInstance": {
			client: &external{cms: &fake.MockClient{
				MockUpdateInstance: func(_ context.Context, _ *redisv1pb.UpdateInstanceRequest, _ ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error) {
					return nil, errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(),
				err: errors.Wrap(errorBoom, errUpdateInstance),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Update(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.update, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Update(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Update(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
func TestDelete(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg  resource.Managed
		err error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"DeletedInstance": {
			client: &external{cms: &fake.MockClient{
				MockDeleteInstance: func(_ context.Context, _ *redisv1pb.DeleteInstanceRequest, _ ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error) {
					return nil, nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(withConditions(xpv1.Deleting())),
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotInstance),
			},
		},
		"FailedToDeleteInstance": {
			client: &external{cms: &fake.MockClient{
				MockDeleteInstance: func(_ context.Context, _ *redisv1pb.DeleteInstanceRequest, _ ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error) {
					return nil, errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(withConditions(xpv1.Deleting())),
				err: errors.Wrap(errorBoom, errDeleteInstance),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.client.Delete(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Delete(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
