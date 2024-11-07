package controller

import (
	"context"
	"fmt"

	crdv1alpha1 "github.com/chanho3114/kinx-cloud-controller-manager/api/v1alpha1"
	kinxutil "github.com/chanho3114/kinx-cloud-controller-manager/util"
	"github.com/chanho3114/kinx-cloud-controller-manager/util/ssa"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const KinxClusterNameLabel = "kinx.net/cluster-name"
const fieldManagerName = "kinx-cloud-controller-manager"
const secretTemplate = `[Global]
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
`

func (r *KinxCloudControllerManagerReconciler) reconcileSecret(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error) {
	log := r.Log.WithValues("Secret", types.NamespacedName{Namespace: ccm.Namespace, Name: ccm.Name})

	currentSecret := &corev1.Secret{}

	desiredCloudConf := fmt.Sprintf(secretTemplate, ccm.Spec.ApplicationCredentialID, ccm.Spec.ApplicationCredentialSecret, ccm.Spec.AuthURL, ccm.Spec.UserDomainName, ccm.Spec.ProjectName, ccm.Spec.UseOctavia)

	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "ccm-cloud-config"),
			Namespace: ccm.ObjectMeta.Namespace,
			Labels: map[string]string{
				KinxClusterNameLabel: ccm.Spec.ClusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         ccm.APIVersion,
					Kind:               ccm.Kind,
					Name:               ccm.Name,
					UID:                ccm.UID,
					Controller:         kinxutil.BoolPtr(true),
					BlockOwnerDeletion: kinxutil.BoolPtr(false),
				},
			},
		},
		Data: map[string][]byte{
			"cloud.conf": []byte(desiredCloudConf),
		},
		Type: corev1.SecretTypeOpaque,
	}

	// 현재 Secret 객체 조회
	err := r.Get(ctx, types.NamespacedName{Namespace: ccm.Namespace, Name: fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "ccm-cloud-config")}, currentSecret)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Secret이 없으면 새로 생성
	if errors.IsNotFound(err) {
		log.Info("Secret not found, creating new Secret")
		log.Info(desiredSecret.String())

		err := ssa.Patch(ctx, r.Client, fieldManagerName, desiredSecret)
		if err != nil {
			log.Info("Failed to create Secret")
			return ctrl.Result{}, err
		}

		log.Info("Secret created successfully")

		return ctrl.Result{}, nil
	}

	// 기존 Secret이 있으면 업데이트
	log.Info("Secret found, updating existing Secret")

	log.Info("Existing Secret")
	log.Info(currentSecret.String())
	log.Info("Desired Secret")
	log.Info(desiredSecret.String())

	err = ssa.Patch(ctx, r.Client, fieldManagerName, desiredSecret, ssa.WithCachingProxy{Cache: r.ssaCache, Original: currentSecret})
	if err != nil {
		log.Info("Failed to update Secret")
		return ctrl.Result{}, err
	}

	log.Info("Secret updated successfully")

	return ctrl.Result{}, nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileServiceAccount(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error) {
	log := r.Log.WithValues("ServiceAccount", types.NamespacedName{Namespace: ccm.Namespace, Name: ccm.Name})

	currentServiceaccount := &corev1.ServiceAccount{}

	desiredServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.ObjectMeta.Namespace,
			Labels: map[string]string{
				KinxClusterNameLabel: ccm.Spec.ClusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         ccm.APIVersion,
					Kind:               ccm.Kind,
					Name:               ccm.Name,
					UID:                ccm.UID,
					Controller:         kinxutil.BoolPtr(true),
					BlockOwnerDeletion: kinxutil.BoolPtr(false),
				},
			},
		},
	}

	// 현재 Serviceaccount 객체 조회
	err := r.Get(ctx, types.NamespacedName{Namespace: ccm.Namespace, Name: fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager")}, currentServiceaccount)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Serviceaccount 없으면 새로 생성
	if errors.IsNotFound(err) {
		log.Info("Serviceaccount not found, creating new Serviceaccount")

		err := ssa.Patch(ctx, r.Client, fieldManagerName, desiredServiceAccount)
		if err != nil {
			log.Info("Failed to create Serviceaccount")
			return ctrl.Result{}, err
		}

		log.Info("Serviceaccount created successfully")

		return ctrl.Result{}, nil
	}

	// 기존 Serviceaccount 있으면 업데이트
	log.Info("Serviceaccount found, updating existing Serviceaccount")

	err = ssa.Patch(ctx, r.Client, fieldManagerName, desiredServiceAccount, ssa.WithCachingProxy{Cache: r.ssaCache, Original: currentServiceaccount})
	if err != nil {
		log.Info("Failed to update Serviceaccount")
		return ctrl.Result{}, err
	}

	log.Info("Serviceaccount updated successfully")

	return ctrl.Result{}, nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileRole(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error) {
	log := r.Log.WithValues("Role", types.NamespacedName{Namespace: ccm.Namespace, Name: ccm.Name})

	currentRole := &rbacv1.Role{}

	desiredRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.Namespace,
			Labels: map[string]string{
				KinxClusterNameLabel: ccm.Spec.ClusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         ccm.APIVersion,
					Kind:               ccm.Kind,
					Name:               ccm.Name,
					UID:                ccm.UID,
					Controller:         kinxutil.BoolPtr(true),
					BlockOwnerDeletion: kinxutil.BoolPtr(false),
				},
			},
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

	// 현재 Role 객체 조회
	err := r.Get(ctx, types.NamespacedName{Namespace: ccm.Namespace, Name: fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager")}, currentRole)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Role 없으면 새로 생성
	if errors.IsNotFound(err) {
		log.Info("Role not found, creating new Role")

		err := ssa.Patch(ctx, r.Client, fieldManagerName, desiredRole)
		if err != nil {
			log.Info("Failed to create Role")
			return ctrl.Result{}, err
		}

		log.Info("Role created successfully")

		return ctrl.Result{}, nil
	}

	log.Info("Existing Role")
	log.Info(currentRole.String())
	log.Info("Desired Role")
	log.Info(desiredRole.String())

	// 기존 Role 있으면 업데이트
	log.Info("Role found, updating existing Role")

	err = ssa.Patch(ctx, r.Client, fieldManagerName, desiredRole, ssa.WithCachingProxy{Cache: r.ssaCache, Original: currentRole})
	if err != nil {
		log.Info("Failed to update Role")
		return ctrl.Result{}, err
	}

	log.Info("Role updated successfully")

	return ctrl.Result{}, nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileRoleBinding(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error) {
	log := r.Log.WithValues("Rolebinding", types.NamespacedName{Namespace: ccm.Namespace, Name: ccm.Name})

	currentRoleBinding := &rbacv1.RoleBinding{}

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
	desiredRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.Namespace,
			Labels: map[string]string{
				KinxClusterNameLabel: ccm.Spec.ClusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         ccm.APIVersion,
					Kind:               ccm.Kind,
					Name:               ccm.Name,
					UID:                ccm.UID,
					Controller:         kinxutil.BoolPtr(true),
					BlockOwnerDeletion: kinxutil.BoolPtr(false),
				},
			},
		},
		RoleRef:  roleRef,
		Subjects: subjects,
	}

	// 현재 Rolebinding 객체 조회
	err := r.Get(ctx, types.NamespacedName{Namespace: ccm.Namespace, Name: fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager")}, currentRoleBinding)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Rolebinding 없으면 새로 생성
	if errors.IsNotFound(err) {
		log.Info("Rolebinding not found, creating new Rolebinding")

		err := ssa.Patch(ctx, r.Client, fieldManagerName, desiredRoleBinding)
		if err != nil {
			log.Info("Failed to create Rolebinding")
			return ctrl.Result{}, err
		}

		log.Info("Rolebinding created successfully")

		return ctrl.Result{}, nil
	}

	// 기존 Rolebinding 있으면 업데이트
	log.Info("Rolebinding found, updating existing Rolebinding")

	log.Info("Existing Rolebinding")
	log.Info(currentRoleBinding.String())
	log.Info("Desired Rolebinding")
	log.Info(desiredRoleBinding.String())

	err = ssa.Patch(ctx, r.Client, fieldManagerName, desiredRoleBinding, ssa.WithCachingProxy{Cache: r.ssaCache, Original: currentRoleBinding})
	if err != nil {
		log.Info("Failed to update Rolebinding")
		return ctrl.Result{}, err
	}

	log.Info("Rolebinding updated successfully")

	return ctrl.Result{}, nil
}

func (r *KinxCloudControllerManagerReconciler) reconcileDeployment(ctx context.Context, ccm *crdv1alpha1.KinxCloudControllerManager) (ctrl.Result, error) {
	log := r.Log.WithValues("Deployment", types.NamespacedName{Namespace: ccm.Namespace, Name: ccm.Name})

	currentDeployment := &appsv1.Deployment{}

	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
			Namespace: ccm.Namespace,
			Labels: map[string]string{
				KinxClusterNameLabel: ccm.Spec.ClusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         ccm.APIVersion,
					Kind:               ccm.Kind,
					Name:               ccm.Name,
					UID:                ccm.UID,
					Controller:         kinxutil.BoolPtr(true),
					BlockOwnerDeletion: kinxutil.BoolPtr(false),
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: kinxutil.Int32Ptr(1),
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
						RunAsUser: kinxutil.Int64Ptr(1001),
					},
					ServiceAccountName: fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager"),
					Volumes: []corev1.Volume{
						{
							Name: "cloud-config-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "ccm-cloud-config"),
									DefaultMode: kinxutil.Int32Ptr(420),
								},
							},
						},
						{
							Name: "user-kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "admin-kubeconfig"),
									DefaultMode: kinxutil.Int32Ptr(420),
								},
							},
						},
					},
				},
			},
		},
	}

	// 현재 Deployment 객체 조회
	err := r.Get(ctx, types.NamespacedName{Namespace: ccm.Namespace, Name: fmt.Sprintf("%s-%s", ccm.Spec.ClusterName, "cloud-controller-manager")}, currentDeployment)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Deployment 없으면 새로 생성
	if errors.IsNotFound(err) {
		log.Info("Deployment not found, creating new Deployment")

		err := ssa.Patch(ctx, r.Client, fieldManagerName, desiredDeployment)
		if err != nil {
			log.Info("Failed to create Deployment")
			return ctrl.Result{}, err
		}

		log.Info("Deployment created successfully")

		return ctrl.Result{}, nil
	}

	// 기존 Deployment 있으면 업데이트
	log.Info("Deployment found, updating existing Deployment")

	err = ssa.Patch(ctx, r.Client, fieldManagerName, desiredDeployment, ssa.WithCachingProxy{Cache: r.ssaCache, Original: currentDeployment})
	if err != nil {
		log.Info("Failed to update Deployment")
		return ctrl.Result{}, err
	}

	log.Info("Deployment updated successfully")

	return ctrl.Result{}, nil
}
