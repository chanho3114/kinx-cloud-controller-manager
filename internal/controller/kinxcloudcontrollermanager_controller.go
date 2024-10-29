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
	"encoding/base64"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crdv1alpha1 "github.com/chanho3114/kinx-cloud-controller-manager/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const kccmFinalizer = "github.com/chanho3114/kinx_cloud_controller_manager_finalizer"

// KinxCloudControllerManagerReconciler reconciles a KinxCloudControllerManager object
type KinxCloudControllerManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
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
func (r *KinxCloudControllerManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("kinxcloudcontrollermanager", req.NamespacedName)
	log.Info("Reconciling KinxCloudControllerManager")

	ccm := &crdv1alpha1.KinxCloudControllerManager{}

	if err := r.Get(ctx, req.NamespacedName, ccm); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(ccm, kccmFinalizer) {
		log.Info("Finalizer 추가")
		controllerutil.AddFinalizer(ccm, kccmFinalizer)
		return ctrl.Result{}, r.Update(ctx, ccm)
	}

	if !ccm.DeletionTimestamp.IsZero() {
		log.Info("KinxCloudControllerManager 삭제")
		return r.reconcileDelete(ctx, ccm)
	}

	// log.Info("KinxCloudControllerManager 생성")

	log.Info("Secret Reconcile")
	err := r.reconcileSecret(ctx, ccm)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("ServiceAccount Reconcile")
	err = r.reconcileServiceAccount(ctx, ccm)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Role Reconcile")
	err = r.reconcileRole(ctx, ccm)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("RoleBinding Reconcile")
	err = r.reconcileRoleBinding(ctx, ccm)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Deployment Reconcile")
	err = r.reconcileDeployment(ctx, ccm)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Status Reconcile")
	err = r.reconcileStatus(ctx, ccm)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileDelete(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error) {
	log := r.Log.WithValues("Delete", ccm.Namespace, ccm.Name)
	log.Info("removing application")

	controllerutil.RemoveFinalizer(ccm, kccmFinalizer)

	err := r.Update(ctx, ccm)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error removing finalizer %v", err)
	}

	return ctrl.Result{}, nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileSecret(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) error {
	log := r.Log.WithValues("Secret", ccm.Namespace, ccm.Name)

	// Secret 데이터를 생성
	cloudConf := fmt.Sprintf(`
        [Global]
        application-credential-id = "%s"
        application-credential-secret = "%s"
        auth-url = "%s"
        domain-name = "%s"
        tenant-name = "%s"

        [Networking]

        [LoadBalancer]
        use-octavia = "%t"

        [BlockStorage]

        [Metadata]

        [Route]
    `, ccm.Spec.ApplicationCredentialID, ccm.Spec.ApplicationCredentialSecret, ccm.Spec.AuthURL, ccm.Spec.UserDomainName, ccm.Spec.ProjectName, ccm.Spec.UseOctavia)

	// cloud.conf를 base64로 인코딩
	encodedCloudConf := base64.StdEncoding.EncodeToString([]byte(cloudConf))

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "ccm-cloud-config"),
			Namespace: ccm.ObjectMeta.Namespace,
		},
		Data: map[string][]byte{
			"cloud.conf": []byte(encodedCloudConf),
		},
		Type: corev1.SecretTypeOpaque,
	}

	controllerutil.SetOwnerReference(ccm, secret, r.Scheme, func(or *metav1.OwnerReference) {
		or.BlockOwnerDeletion = boolPtr(true)
		or.Controller = boolPtr(true)
	})

	result, err := controllerutil.CreateOrPatch(ctx, r.Client, secret, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("secret를 생성할 수 없습니다!: %v", err)
	}

	log.Info("Result : ", result)

	return nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileServiceAccount(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) error {
	log := r.Log.WithValues("ServiceAccount", ccm.Namespace, ccm.Name)

	// ServiceAccount 객체를 생성합니다.
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.ObjectMeta.Namespace,
		},
	}

	controllerutil.SetOwnerReference(ccm, serviceAccount, r.Scheme, func(or *metav1.OwnerReference) {
		or.BlockOwnerDeletion = boolPtr(true)
		or.Controller = boolPtr(true)
	})

	result, err := controllerutil.CreateOrPatch(ctx, r.Client, serviceAccount, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("serviceAccount를 생성할 수 없습니다!: %v", err)
	}

	log.Info("Result : ", result)

	return nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileRole(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) error {
	log := r.Log.WithValues("Role", ccm.Namespace, ccm.Name)

	// Role 객체 생성
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "create", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/status"},
				Verbs:     []string{"patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services/status"},
				Verbs:     []string{"patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"serviceaccounts/token"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"serviceaccounts"},
				Verbs:     []string{"create", "get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"create", "get", "list", "watch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"list", "get", "watch"},
			},
		},
	}

	controllerutil.SetOwnerReference(ccm, role, r.Scheme, func(or *metav1.OwnerReference) {
		or.BlockOwnerDeletion = boolPtr(true)
		or.Controller = boolPtr(true)
	})

	result, err := controllerutil.CreateOrPatch(ctx, r.Client, role, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("role를 생성할 수 없습니다!: %v", err)
	}

	log.Info("Result : ", result)

	return nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileRoleBinding(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) error {
	log := r.Log.WithValues("Rolebinding", ccm.Namespace, ccm.Name)

	// RoleRef 정보
	roleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Kind:     "Role",
		Name:     fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
	}

	// Subjects 정보 (ServiceAccount)
	subjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.Namespace,
		},
	}

	// RoleBinding 객체를 생성합니다.
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.Namespace,
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}

	controllerutil.SetOwnerReference(ccm, roleBinding, r.Scheme, func(or *metav1.OwnerReference) {
		or.BlockOwnerDeletion = boolPtr(true)
		or.Controller = boolPtr(true)
	})

	result, err := controllerutil.CreateOrPatch(ctx, r.Client, roleBinding, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("rolebinding를 생성할 수 없습니다!: %v", err)
	}

	log.Info("Result : ", result)

	return nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileDeployment(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) error {
	log := r.Log.WithValues("Deployment", ccm.Namespace, ccm.Name)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "kinx-cloud-controller-manager"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "kinx-cloud-controller-manager"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kinx-cloud-controller-manager",
							Image: "registry.k8s.io/provider-os/openstack-cloud-controller-manager:v1.30.0",
							Args: []string{
								"/bin/openstack-cloud-controller-manager",
								"--v=2",
								"--cloud-config=$(CLOUD_CONFIG)",
								"--cluster-name=$(CLUSTER_NAME)",
								"--cloud-provider=openstack",
								"--kubeconfig=$(KUBECONFIG)",
								"--use-service-account-credentials=false",
								"--leader-elect=false",
								"--controllers=cloud-node,cloud-node-lifecycle,route,service",
								"--bind-address=127.0.0.1",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "CLOUD_CONFIG",
									Value: "/etc/config/cloud.conf",
								},
								{
									Name:  "CLUSTER_NAME",
									Value: "kubernetes",
								},
								{
									Name:  "KUBECONFIG",
									Value: "/etc/config/kubeconfig/admin.conf",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cloud-config-volume",
									MountPath: "/etc/config",
									ReadOnly:  true,
								},
								{
									Name:      "user-kubeconfig",
									MountPath: "/etc/config/kubeconfig",
									ReadOnly:  true,
								},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: int64Ptr(1001),
					},
					ServiceAccountName: fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
					Volumes: []corev1.Volume{
						{
							Name: "cloud-config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "ccm-cloud-config"),
									DefaultMode: int32Ptr(420),
								},
							},
						},
						{
							Name: "user-kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "admin-kubeconfig"),
									DefaultMode: int32Ptr(420),
								},
							},
						},
					},
				},
			},
		},
	}

	controllerutil.SetOwnerReference(ccm, deployment, r.Scheme, func(or *metav1.OwnerReference) {
		or.BlockOwnerDeletion = boolPtr(true)
		or.Controller = boolPtr(true)
	})

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("deployment를 생성할 수 없습니다!: %v", err)
	}

	log.Info("Result : ", result)

	return nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileStatus(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) error {
	// log := r.Log.WithValues("Status", ccm.Namespace, ccm.Name)

	dep := &appsv1.Deployment{}

	if err := r.Get(ctx, types.NamespacedName{Namespace: ccm.Namespace, Name: ccm.Spec.ClusterName + "cloud-controller-manager"}, dep); err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KinxCloudControllerManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1alpha1.KinxCloudControllerManager{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&appsv1.Deployment{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(r)
}

// helper functions for pointer types.
func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int) *int32 {
	return ptr.To(int32(i))
}

func int64Ptr(i int) *int64 {
	return ptr.To(int64(i))
}
