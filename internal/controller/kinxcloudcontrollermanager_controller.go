/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crdv1alpha1 "github.com/chanho3114/kinx-cloud-controller-manager/api/v1alpha1"
	"github.com/chanho3114/kinx-cloud-controller-manager/util/ssa"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
)

const kccmFinalizer = "kinx.net/cloud-controller-manager-finalizer"

type ccmReconcileFunc func(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error)

// KinxCloudControllerManagerReconciler reconciles a KinxCloudControllerManager object
type KinxCloudControllerManagerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	ssaCache ssa.Cache
}

//+kubebuilder:rbac:groups=crd.kinx.net,resources=kinxcloudcontrollermanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.kinx.net,resources=kinxcloudcontrollermanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.kinx.net,resources=kinxcloudcontrollermanagers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KinxCloudControllerManager object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *KinxCloudControllerManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := r.Log.WithValues("kinxcloudcontrollermanager", req.NamespacedName)
	log.Info("Reconciling KinxCloudControllerManager")

	ccm := &crdv1alpha1.KinxCloudControllerManager{}

	if err := r.Get(ctx, req.NamespacedName, ccm); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(ccm, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		log.Info("Status Reconcile")

		dep := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: ccm.Namespace,
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
		}, dep); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}

		log.Info("Current Deployment status")
		log.Info(dep.Status.String())

		if !reflect.DeepEqual(ccm.Status.Conditions, dep.Status.Conditions) {
			log.Info("CCM Status와 다름")
			ccm.Status.Conditions = dep.Status.Conditions
		} else {
			log.Info("No need to update status")
		}

		patchOpts := []patch.Option{}
		if reterr == nil {
			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
		}
		if err := patchHelper.Patch(ctx, ccm, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if !controllerutil.ContainsFinalizer(ccm, kccmFinalizer) {
		log.Info("Finalizer 추가")
		controllerutil.AddFinalizer(ccm, kccmFinalizer)
		// return ctrl.Result{}, r.Update(ctx, ccm)
	}

	if !ccm.DeletionTimestamp.IsZero() {
		log.Info("KinxCloudControllerManager 삭제")
		return r.reconcileDelete(ctx, ccm)
	}

	alwaysReconcile := []ccmReconcileFunc{
		r.reconcileSecret,
		r.reconcileServiceAccount,
		r.reconcileRole,
		r.reconcileRoleBinding,
		r.reconcileDeployment,
	}

	return doReconcile(ctx, alwaysReconcile, ccm)
}

func (r *KinxCloudControllerManagerReconciler) reconcileDelete(_ context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error) {
	log := r.Log.WithValues("Delete Reconcile", types.NamespacedName{
		Namespace: ccm.Namespace,
		Name:      ccm.Name,
	})

	log.Info("removing application")

	controllerutil.RemoveFinalizer(ccm, kccmFinalizer)

	return ctrl.Result{}, nil
}

func doReconcile(ctx context.Context, funcs []ccmReconcileFunc, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error) {
	res := ctrl.Result{}
	errs := []error{}

	for _, reconcileFunc := range funcs {
		// Call the inner reconciliation methods.
		result, err := reconcileFunc(ctx, ccm)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			continue
		}
		res = util.LowestNonZeroResult(res, result)
	}

	if len(errs) > 0 {
		return ctrl.Result{}, kerrors.NewAggregate(errs)
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KinxCloudControllerManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&crdv1alpha1.KinxCloudControllerManager{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(r)
	if err != nil {
		return err
	}

	r.ssaCache = ssa.NewCache()

	return nil
}
