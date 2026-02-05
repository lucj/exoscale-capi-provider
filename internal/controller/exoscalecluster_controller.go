package controller

import (
	"context"
	"errors"
	"fmt"

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

const exoscaleClusterFinalizer = "exoscalecluster.infrastructure.cluster.x-k8s.io"

// ExoscaleClusterReconciler reconciles ExoscaleCluster objects.
type ExoscaleClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ExoscaleClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := logf.FromContext(ctx).WithValues("exoscalecluster", req.NamespacedName)
	ctx = logf.IntoContext(ctx, log)

	cluster := &infrav1.ExoscaleCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(cluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if patchErr := patchHelper.Patch(ctx, cluster); patchErr != nil {
			err = errors.Join(err, patchErr)
		}
	}()

	cc, err := cloud.NewClient(cluster.Spec.Zone)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster, cc)
	}

	if !controllerutil.ContainsFinalizer(cluster, exoscaleClusterFinalizer) {
		controllerutil.AddFinalizer(cluster, exoscaleClusterFinalizer)
	}

	return r.reconcileNormal(ctx, cluster, cc)
}

func (r *ExoscaleClusterReconciler) reconcileNormal(ctx context.Context, cluster *infrav1.ExoscaleCluster, cc *cloud.Client) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if err := r.reconcileSecurityGroups(ctx, cluster, cc); err != nil {
		setCondition(cluster, infrav1.SecurityGroupsReadyCondition, corev1.ConditionFalse, "ReconcileFailed", err.Error())
		return ctrl.Result{}, err
	}

	if err := r.reconcileEIP(ctx, cluster, cc); err != nil {
		setCondition(cluster, infrav1.EIPReadyCondition, corev1.ConditionFalse, "ReconcileFailed", err.Error())
		return ctrl.Result{}, err
	}

	// If any resource was cleared (e.g. vanished externally), requeue so the
	// next iteration recreates it.
	if cluster.Status.MasterSecurityGroupID == "" ||
		cluster.Status.NodeSecurityGroupID == "" ||
		cluster.Status.EIPID == "" {
		log.Info("Cloud resources not yet ready, requeueing")
		return ctrl.Result{Requeue: true}, nil
	}

	cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: cluster.Status.EIPAddress,
		Port: 6443,
	}

	setCondition(cluster, infrav1.SecurityGroupsReadyCondition, corev1.ConditionTrue, "Ready", "Security groups are ready")
	setCondition(cluster, infrav1.EIPReadyCondition, corev1.ConditionTrue, "Ready", "Elastic IP is ready")
	cluster.Status.Ready = true

	return ctrl.Result{}, nil
}

// reconcileSecurityGroups ensures both the master and node security groups
// exist and carry the expected baseline rules.
func (r *ExoscaleClusterReconciler) reconcileSecurityGroups(ctx context.Context, cluster *infrav1.ExoscaleCluster, cc *cloud.Client) error {
	log := logf.FromContext(ctx)

	masterRules := []cloud.IngressRule{
		{Protocol: "tcp", StartPort: 22, EndPort: 22, Network: "0.0.0.0/0"},      // SSH
		{Protocol: "tcp", StartPort: 6443, EndPort: 6443, Network: "0.0.0.0/0"},  // kube-apiserver
	}
	nodeRules := []cloud.IngressRule{
		{Protocol: "tcp", StartPort: 22, EndPort: 22, Network: "0.0.0.0/0"},      // SSH
	}

	masterName := cluster.Spec.MasterSecurityGroup
	if masterName == "" {
		masterName = cluster.Name + "-master"
	}
	masterID, err := r.ensureSecurityGroup(ctx, cc, masterName,
		fmt.Sprintf("Master security group for cluster %s", cluster.Name), masterRules)
	if err != nil {
		return err
	}
	if masterID != cluster.Status.MasterSecurityGroupID {
		log.Info("Master security group reconciled", "name", masterName, "id", masterID)
		cluster.Status.MasterSecurityGroupID = masterID
	}

	nodeName := cluster.Spec.NodeSecurityGroup
	if nodeName == "" {
		nodeName = cluster.Name + "-node"
	}
	nodeID, err := r.ensureSecurityGroup(ctx, cc, nodeName,
		fmt.Sprintf("Node security group for cluster %s", cluster.Name), nodeRules)
	if err != nil {
		return err
	}
	if nodeID != cluster.Status.NodeSecurityGroupID {
		log.Info("Node security group reconciled", "name", nodeName, "id", nodeID)
		cluster.Status.NodeSecurityGroupID = nodeID
	}

	return nil
}

// ensureSecurityGroup creates the security group if it does not exist and
// guarantees every rule in rules is present.  It returns the security group ID.
func (r *ExoscaleClusterReconciler) ensureSecurityGroup(ctx context.Context, cc *cloud.Client, name, description string, rules []cloud.IngressRule) (string, error) {
	log := logf.FromContext(ctx)

	id, found, err := cc.FindSecurityGroupByName(ctx, name)
	if err != nil {
		return "", err
	}
	if !found {
		id, err = cc.CreateSecurityGroup(ctx, name, description)
		if err != nil {
			return "", err
		}
		log.Info("Created security group", "name", name, "id", id)
	}

	// Ensure all baseline rules exist (idempotent).
	sg, exists, err := cc.GetSecurityGroup(ctx, id)
	if err != nil {
		return "", err
	}
	if exists {
		for _, rule := range rules {
			if !cloud.HasIngressRule(sg, rule) {
				if err := cc.AddIngressRule(ctx, id, rule); err != nil {
					return "", fmt.Errorf("adding rule to security group %s: %w", id, err)
				}
			}
		}
	}

	return id, nil
}

// reconcileEIP ensures an Elastic IP exists for the control-plane endpoint.
func (r *ExoscaleClusterReconciler) reconcileEIP(ctx context.Context, cluster *infrav1.ExoscaleCluster, cc *cloud.Client) error {
	log := logf.FromContext(ctx)

	if cluster.Status.EIPID == "" {
		id, ip, err := cc.CreateElasticIP(ctx, fmt.Sprintf("Control plane endpoint for cluster %s", cluster.Name))
		if err != nil {
			return err
		}
		cluster.Status.EIPID = id
		cluster.Status.EIPAddress = ip
		log.Info("Created Elastic IP", "id", id, "ip", ip)
		return nil
	}

	ip, exists, err := cc.GetElasticIP(ctx, cluster.Status.EIPID)
	if err != nil {
		return err
	}
	if !exists {
		log.Info("Elastic IP disappeared, clearing")
		cluster.Status.EIPID = ""
		cluster.Status.EIPAddress = ""
		return nil // Will be recreated on the next reconcile.
	}
	cluster.Status.EIPAddress = ip
	return nil
}

func (r *ExoscaleClusterReconciler) reconcileDelete(ctx context.Context, cluster *infrav1.ExoscaleCluster, cc *cloud.Client) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(cluster, exoscaleClusterFinalizer) {
		return ctrl.Result{}, nil
	}

	if cluster.Status.EIPID != "" {
		log.Info("Deleting Elastic IP", "id", cluster.Status.EIPID)
		if err := cc.DeleteElasticIP(ctx, cluster.Status.EIPID); err != nil {
			return ctrl.Result{}, err
		}
		cluster.Status.EIPID = ""
		cluster.Status.EIPAddress = ""
	}

	if cluster.Status.MasterSecurityGroupID != "" {
		log.Info("Deleting master security group", "id", cluster.Status.MasterSecurityGroupID)
		if err := cc.DeleteSecurityGroup(ctx, cluster.Status.MasterSecurityGroupID); err != nil {
			return ctrl.Result{}, err
		}
		cluster.Status.MasterSecurityGroupID = ""
	}

	if cluster.Status.NodeSecurityGroupID != "" {
		log.Info("Deleting node security group", "id", cluster.Status.NodeSecurityGroupID)
		if err := cc.DeleteSecurityGroup(ctx, cluster.Status.NodeSecurityGroupID); err != nil {
			return ctrl.Result{}, err
		}
		cluster.Status.NodeSecurityGroupID = ""
	}

	controllerutil.RemoveFinalizer(cluster, exoscaleClusterFinalizer)
	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler with the controller manager.
func (r *ExoscaleClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ExoscaleCluster{}).
		Complete(r)
}
