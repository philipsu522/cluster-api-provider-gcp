package scope

import (
	"context"
	"sigs.k8s.io/cluster-api-provider-gcp/api/exp"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"

	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type GKEMachinePoolScopeParams struct {
	GKEClients
	Client     client.Client
	Logger     logr.Logger
	Cluster    *clusterv1.Cluster
	MachinePool    *capiv1exp.MachinePool
	GKECluster *exp.GKECluster
	GKEMachinePool *exp.GKEMachinePool
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewGKEMachinePoolScope(params GKEMachinePoolScopeParams) (*GKEMachinePoolScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachineScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machine is required when creating a MachineScope")
	}
	if params.Cluster == nil {
		return nil, errors.New("cluster is required when creating a MachineScope")
	}
	if params.GKECluster == nil {
		return nil, errors.New("gke cluster is required when creating a MachineScope")
	}
	if params.GKEMachinePool == nil {
		return nil, errors.New("gke machine is required when creating a MachineScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.GKEMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &GKEMachinePoolScope{
		client:      params.Client,
		Cluster:     params.Cluster,
		MachinePool:     params.MachinePool,
		GKECluster:  params.GKECluster,
		GKEMachinePool:  params.GKEMachinePool,
		Logger:      params.Logger,
		patchHelper: helper,
	}, nil
}

// MachineScope defines a scope defined around a machine and its cluster.
type GKEMachinePoolScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster    *clusterv1.Cluster
	MachinePool    *capiv1exp.MachinePool
	GKECluster *exp.GKECluster
	GKEMachinePool *exp.GKEMachinePool
}

// Region returns the GCPMachine region.
func (m *GKEMachinePoolScope) Region() string {
	return m.GKECluster.Spec.Region
}

// Name returns the GCPMachine name.
func (m *GKEMachinePoolScope) Name() string {
	return m.GKEMachinePool.Name
}

// Namespace returns the namespace name.
func (m *GKEMachinePoolScope) Namespace() string {
	return m.GKEMachinePool.Namespace
}

// SetReady sets the GCPMachine Ready Status.
func (m *GKEMachinePoolScope) SetReady() {
	m.GKEMachinePool.Status.Ready = true
}

// SetFailureMessage sets the GCPMachine status failure message.
func (m *GKEMachinePoolScope) SetFailureMessage(v error) {
	m.GKEMachinePool.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the GCPMachine status failure reason.
func (m *GKEMachinePoolScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.GKEMachinePool.Status.FailureReason = &v
}

// SetAnnotation sets a key value annotation on the GCPMachine.
func (m *GKEMachinePoolScope) SetAnnotation(key, value string) {
	if m.GKEMachinePool.Annotations == nil {
		m.GKEMachinePool.Annotations = map[string]string{}
	}
	m.GKEMachinePool.Annotations[key] = value
}

// PatchObject persists the cluster configuration and status.
func (m *GKEMachinePoolScope) PatchObject() error {
	return m.patchHelper.Patch(context.TODO(), m.GKEMachinePool)
}

// Close closes the current scope persisting the cluster configuration and status.
func (m *GKEMachinePoolScope) Close() error {
	return m.PatchObject()
}
