/*
Copyright 2022 The Crossplane Authors.

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

package cluster

import (
	"context"
	"fmt"
	"net/http"

	cockroachdb "github.com/cockroachdb/cockroach-cloud-sdk-go/pkg/client"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/provider-cockroachdb/apis/database/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-cockroachdb/apis/v1alpha1"
	"github.com/crossplane/provider-cockroachdb/internal/controller/features"
	"github.com/crossplane/provider-cockroachdb/pkg/cockroachca"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errNotCluster   = "managed resource is not a Cluster custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Service"

	defaultCAURL = "https://cockroachlabs.cloud/"
)

type CockroachdbService struct {
	crdbClient cockroachdb.Service
	caClient   *cockroachca.CAClient
}

var (
	newCockroachdbService = func(creds []byte) (*CockroachdbService, error) {
		clientConfig := cockroachdb.NewConfiguration(string(creds))
		cockroachclient := cockroachdb.NewClient(clientConfig)
		service := cockroachdb.NewService(cockroachclient)

		caClient, err := cockroachca.NewCAClient(
			cockroachca.WithBaseURL(defaultCAURL),
			cockroachca.WithHTTPClient(http.DefaultClient),
		)
		if err != nil {
			return nil, fmt.Errorf("error creatint CA client: %v", err)
		}

		return &CockroachdbService{
			crdbClient: service,
			caClient:   caClient,
		}, nil
	}
)

// Setup adds a controller that reconciles Cluster managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.ClusterGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.ClusterGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: newCockroachdbService}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Cluster{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(creds []byte) (*CockroachdbService, error)
}

// Connect typically produces an ExternalClient by:
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return nil, errors.New(errNotCluster)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	svc, err := c.newServiceFn(data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{
		service: svc,
		kube:    c.kube,
	}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	service *CockroachdbService
	kube    client.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotCluster)
	}
	externalName := meta.GetExternalName(cr)

	// 'Status' is not updated in the Create method, so at this point 'Status.AtProvider.ID' will be empty.
	// As an alternative, check if we have a legit ID to perform the GET request.
	if !isValidUUID(externalName) {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	cluster, res, err := c.service.crdbClient.GetCluster(ctx, externalName)
	if err != nil {
		if res.StatusCode == http.StatusNotFound {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, err
	}

	fillAtProvider(cr, cluster)

	switch cluster.State {
	case cockroachdb.CLUSTERSTATETYPE_CREATED:
		cr.Status.SetConditions(xpv1.Available())
	case cockroachdb.CLUSTERSTATETYPE_CREATING:
		cr.Status.SetConditions(xpv1.Creating())
	case cockroachdb.CLUSTERSTATETYPE_DELETED:
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	default:
		cr.Status.SetConditions(xpv1.Unavailable())
	}

	return managed.ExternalObservation{
		ResourceExists:    true,
		ResourceUpToDate:  isUpToDate(cr, cluster),
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotCluster)
	}

	cluster, _, err := c.service.crdbClient.CreateCluster(ctx, cr.CreateClusterRequest())
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	meta.SetExternalName(cr, cluster.Id)

	pwd, err := getPassword(ctx, c.kube, cr.Spec.ForProvider.Credentials.PasswordSecretRef)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	_, _, err = c.service.crdbClient.CreateSQLUser(ctx, cluster.Id, cr.CreateSQLUserRequest(string(pwd)))
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	ca, err := c.service.caClient.ClusterCACert(ctx, cluster)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{
		ConnectionDetails: getConnectionDetails(cr, cluster, ca, pwd),
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotCluster)
	}
	externalName := meta.GetExternalName(cr)

	_, _, err := c.service.crdbClient.UpdateCluster(ctx, externalName, cr.UpdateClusterSpec(), &cockroachdb.UpdateClusterOptions{})
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Cluster)
	if !ok {
		return errors.New(errNotCluster)
	}
	externalName := meta.GetExternalName(cr)

	_, _, err := c.service.crdbClient.DeleteCluster(ctx, externalName)
	return err
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func fillAtProvider(cr *v1alpha1.Cluster, cluster *cockroachdb.Cluster) {
	cr.Status.AtProvider.ID = cluster.Id
	cr.Status.AtProvider.State = string(cluster.State)
}

func isUpToDate(cr *v1alpha1.Cluster, cluster *cockroachdb.Cluster) bool {
	return *cr.Spec.ForProvider.Serverless.SpendLimit == cluster.Config.Serverless.SpendLimit
}

func getPassword(ctx context.Context, kube client.Client, secretKeySelector *xpv1.SecretKeySelector) ([]byte, error) {
	if secretKeySelector == nil {
		password, err := password.Generate(16, 4, 0, false, false)
		if err != nil {
			return nil, fmt.Errorf("error generating random password: %v", err)
		}
		return []byte(password), nil
	}

	nn := types.NamespacedName{
		Name:      secretKeySelector.Name,
		Namespace: secretKeySelector.Namespace,
	}

	var secret corev1.Secret
	if err := kube.Get(ctx, nn, &secret); err != nil {
		return nil, err
	}

	val, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return nil, fmt.Errorf("secret key \"%s\" not found", secretKeySelector.Key)
	}

	return val, nil
}

func getConnectionDetails(cr *v1alpha1.Cluster, cluster *cockroachdb.Cluster, ca, password []byte) managed.ConnectionDetails {
	// TODO: Adapt this when supporting dedicated clusters, as they can run in multiple regions
	host := cluster.Regions[0].SqlDns
	user := cr.Spec.ForProvider.Credentials.Username
	dsn := fmt.Sprintf(
		"postgresql://%s:%s@%s:26257/defaultdb?sslmode=verify-full&options=--cluster%s%s",
		user,
		password,
		host,
		"%3D",
		cluster.Name,
	)

	return managed.ConnectionDetails{
		"ca.crt": ca,
		"dsn":    []byte(dsn),
	}
}
