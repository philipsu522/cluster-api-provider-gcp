package scope

import (
	"context"
	"sigs.k8s.io/cluster-api-provider-gcp/api/exp"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"google.golang.org/api/container/v1beta1"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GKEClusterScopeParams defines the input parameters used to create a new Scope.
type GKEClusterScopeParams struct {
	GKEClients
	Client     client.Client
	Logger     logr.Logger
	Cluster    *clusterv1.Cluster
	GKECluster *exp.GKECluster
}

// NewGKEClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewGKEClusterScope(params GKEClusterScopeParams) (*GKEClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.GKECluster == nil {
		return nil, errors.New("failed to generate new scope from nil GKECluster")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	containerSvc, err := container.NewService(context.TODO())
	if err != nil {
		return nil, errors.Errorf("failed to create gke compute client: %v", err)
	}

	if params.GKEClients.Container == nil {
		params.GKEClients.Container = containerSvc
	}

	helper, err := patch.NewHelper(params.GKECluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &GKEClusterScope{
		Logger:      params.Logger,
		client:      params.Client,
		GKEClients:  params.GKEClients,
		Cluster:     params.Cluster,
		GKECluster:  params.GKECluster,
		patchHelper: helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type GKEClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	GKEClients
	Cluster    *clusterv1.Cluster
	GKECluster *exp.GKECluster
}

// Project returns the current project name.
func (s *GKEClusterScope) Project() string {
	return s.GKECluster.Spec.Project
}
// Name returns the cluster name.
func (s *GKEClusterScope) Name() string {
	return s.GKECluster.Name
}

// Namespace returns the cluster namespace.
func (s *GKEClusterScope) Namespace() string {
	return s.GKECluster.Namespace
}

// Region returns the cluster region.
func (s *GKEClusterScope) Region() string {
	return s.GKECluster.Spec.Region
}
// PatchObject persists the cluster configuration and status.
func (s *GKEClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.GKECluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *GKEClusterScope) Close() error {
	return s.PatchObject()
}
