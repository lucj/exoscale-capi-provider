package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "sigs.k8s.io/cluster-api-provider-exoscale/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-exoscale/internal/cloud"
)

const exoscaleMachineFinalizer = "exoscalemachine.infrastructure.cluster.x-k8s.io"

// machineControlPlaneLabel is the well-known label CAPI applies to control-plane Machines.
const machineControlPlaneLabel = "cluster.x-k8s.io/control-plane"

// ExoscaleMachineReconciler reconciles ExoscaleMachine objects.
type ExoscaleMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ExoscaleMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := logf.FromContext(ctx).WithValues("exoscalemachine", req.NamespacedName)
	ctx = logf.IntoContext(ctx, log)

	exoscaleMachine := &infrav1.ExoscaleMachine{}
	if err := r.Get(ctx, req.NamespacedName, exoscaleMachine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(exoscaleMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if patchErr := patchHelper.Patch(ctx, exoscaleMachine); patchErr != nil {
			err = errors.Join(err, patchErr)
		}
	}()

	// Locate the owning CAPI Machine.
	machine, err := r.getOwnerMachine(ctx, exoscaleMachine)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("No owner Machine yet, requeueing")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Walk Machine → Cluster → ExoscaleCluster.
	exoscaleCluster, err := r.getExoscaleCluster(ctx, machine)
	if err != nil {
		return ctrl.Result{}, err
	}

	cc, err := cloud.NewClient(exoscaleMachine.Spec.Zone)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !exoscaleMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, exoscaleMachine, cc)
	}

	if !controllerutil.ContainsFinalizer(exoscaleMachine, exoscaleMachineFinalizer) {
		controllerutil.AddFinalizer(exoscaleMachine, exoscaleMachineFinalizer)
	}

	return r.reconcileNormal(ctx, exoscaleMachine, exoscaleCluster, machine, cc)
}

func (r *ExoscaleMachineReconciler) reconcileNormal(
	ctx context.Context,
	exoscaleMachine *infrav1.ExoscaleMachine,
	exoscaleCluster *infrav1.ExoscaleCluster,
	machine *clusterv1.Machine,
	cc *cloud.Client,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !exoscaleCluster.Status.Ready {
		log.Info("ExoscaleCluster not ready, requeueing")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// Create instance if not yet provisioned.
	if exoscaleMachine.Status.InstanceID == "" {
		// Per CAPI contract: the bootstrap provider sets DataSecretName once the
		// cloud-init payload is ready.  Block until that happens so that the
		// instance is launched with the correct initialisation script.
		if machine.Spec.Bootstrap.DataSecretName == nil {
			log.Info("Bootstrap data not yet available, requeueing")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		// Read the cloud-init user-data written by the bootstrap provider.
		bootstrapSecret := &corev1.Secret{}
		if err := r.Get(ctx, client.ObjectKey{
			Namespace: exoscaleMachine.Namespace,
			Name:      *machine.Spec.Bootstrap.DataSecretName,
		}, bootstrapSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("getting bootstrap secret %q: %w", *machine.Spec.Bootstrap.DataSecretName, err)
		}
		// Exoscale expects the user-data as a base64-encoded string.
		userData := base64.StdEncoding.EncodeToString(bootstrapSecret.Data["value"])

		var sgIDs []string
		if _, isCP := machine.Labels[machineControlPlaneLabel]; isCP {
			sgIDs = []string{exoscaleCluster.Status.MasterSecurityGroupID}
		} else {
			sgIDs = []string{exoscaleCluster.Status.NodeSecurityGroupID}
		}

		instanceID, err := cc.CreateInstance(ctx, cloud.CreateInstanceParams{
			Name:              exoscaleMachine.Name,
			TemplateName:      exoscaleMachine.Spec.Template,
			InstanceTypeName:  exoscaleMachine.Spec.InstanceType,
			DiskSize:          exoscaleMachine.Spec.DiskSize,
			SSHKeyName:        exoscaleMachine.Spec.SSHKey,
			SecurityGroupIDs:  sgIDs,
			AntiAffinityGroup: exoscaleMachine.Spec.AntiAffinityGroup,
			EnableIPv6:        exoscaleMachine.Spec.IPv6,
			UserData:          userData,
		})
		if err != nil {
			setCondition(exoscaleMachine, infrav1.InstanceReadyCondition, corev1.ConditionFalse, "CreateFailed", err.Error())
			return ctrl.Result{}, fmt.Errorf("creating instance: %w", err)
		}
		exoscaleMachine.Status.InstanceID = instanceID
		log.Info("Instance created", "instanceID", instanceID)
	}

	// Poll current state.
	info, err := cc.GetInstance(ctx, exoscaleMachine.Status.InstanceID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting instance state: %w", err)
	}
	if info == nil {
		log.Info("Instance not found, clearing ID for recreation")
		exoscaleMachine.Status.InstanceID = ""
		return ctrl.Result{Requeue: true}, nil
	}

	// Update observed addresses.
	var addresses []clusterv1.MachineAddress
	if info.PublicIP != "" {
		addresses = append(addresses, clusterv1.MachineAddress{
			Type:    clusterv1.MachineExternalIP,
			Address: info.PublicIP,
		})
	}
	if info.IPv6 != "" {
		addresses = append(addresses, clusterv1.MachineAddress{
			Type:    clusterv1.MachineExternalIP,
			Address: info.IPv6,
		})
	}
	exoscaleMachine.Status.Addresses = addresses

	// Set ProviderID once the instance exists.
	providerID := fmt.Sprintf("exoscale://%s", exoscaleMachine.Status.InstanceID)
	exoscaleMachine.Spec.ProviderID = &providerID

	if info.State == "running" {
		// For control-plane machines, attach the cluster Elastic IP so that the
		// announced ControlPlaneEndpoint (the EIP address) actually routes to this
		// instance.  The call is idempotent — re-attaching to the same instance is
		// a no-op, and in a single-master scenario it only needs to succeed once.
		if _, isCP := machine.Labels[machineControlPlaneLabel]; isCP && exoscaleCluster.Status.EIPID != "" {
			if err := cc.AttachElasticIPToInstance(ctx, exoscaleMachine.Status.InstanceID, exoscaleCluster.Status.EIPID); err != nil {
				// Log but do not hard-fail: the EIP may already be attached from a
				// previous reconcile loop that was interrupted before updating status.
				log.Error(err, "Failed to attach EIP to control-plane instance")
			} else {
				log.Info("Elastic IP attached to control-plane instance",
					"instanceID", exoscaleMachine.Status.InstanceID,
					"eipID", exoscaleCluster.Status.EIPID)
			}
		}

		exoscaleMachine.Status.Ready = true
		setCondition(exoscaleMachine, infrav1.InstanceReadyCondition, corev1.ConditionTrue, "InstanceReady", "Instance is running")
		return ctrl.Result{}, nil
	}

	exoscaleMachine.Status.Ready = false
	setCondition(exoscaleMachine, infrav1.InstanceReadyCondition, corev1.ConditionFalse, "InstanceNotReady", fmt.Sprintf("instance state: %s", info.State))
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *ExoscaleMachineReconciler) reconcileDelete(ctx context.Context, exoscaleMachine *infrav1.ExoscaleMachine, cc *cloud.Client) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(exoscaleMachine, exoscaleMachineFinalizer) {
		return ctrl.Result{}, nil
	}

	if exoscaleMachine.Status.InstanceID != "" {
		log.Info("Deleting instance", "id", exoscaleMachine.Status.InstanceID)
		if err := cc.DeleteInstance(ctx, exoscaleMachine.Status.InstanceID); err != nil {
			return ctrl.Result{}, err
		}
		exoscaleMachine.Status.InstanceID = ""
	}

	controllerutil.RemoveFinalizer(exoscaleMachine, exoscaleMachineFinalizer)
	return ctrl.Result{}, nil
}

// getOwnerMachine returns the CAPI Machine referenced in the owner references
// of exoscaleMachine, or nil when the reference has not been set yet.
func (r *ExoscaleMachineReconciler) getOwnerMachine(ctx context.Context, exoscaleMachine *infrav1.ExoscaleMachine) (*clusterv1.Machine, error) {
	for _, ref := range exoscaleMachine.GetOwnerReferences() {
		if ref.Kind == "Machine" {
			machine := &clusterv1.Machine{}
			if err := r.Get(ctx, client.ObjectKey{
				Namespace: exoscaleMachine.Namespace,
				Name:      ref.Name,
			}, machine); err != nil {
				return nil, fmt.Errorf("getting owner Machine %q: %w", ref.Name, err)
			}
			return machine, nil
		}
	}
	return nil, nil
}

// getExoscaleCluster resolves Machine → Cluster → ExoscaleCluster.
func (r *ExoscaleMachineReconciler) getExoscaleCluster(ctx context.Context, machine *clusterv1.Machine) (*infrav1.ExoscaleCluster, error) {
	cluster := &clusterv1.Cluster{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: machine.Namespace,
		Name:      machine.Spec.ClusterName,
	}, cluster); err != nil {
		return nil, fmt.Errorf("getting Cluster %q: %w", machine.Spec.ClusterName, err)
	}

	if cluster.Spec.InfrastructureRef == nil {
		return nil, fmt.Errorf("Cluster %q has no infrastructure ref", cluster.Name)
	}

	exoscaleCluster := &infrav1.ExoscaleCluster{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}, exoscaleCluster); err != nil {
		return nil, fmt.Errorf("getting ExoscaleCluster %q: %w", cluster.Spec.InfrastructureRef.Name, err)
	}

	return exoscaleCluster, nil
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *ExoscaleMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ExoscaleMachine{}).
		Complete(r)
}
