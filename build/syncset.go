package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/lisa/k8s-webhook-framework/pkg/webhooks"
	templatev1 "github.com/openshift/api/template/v1"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	admissionregv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/ghodss/yaml"
)

var (
	listenPort    = flag.Int("port", 5000, "On which port should the Webhook binary listen? (Not the Service port)")
	image         = flag.String("image", "#IMG#:${IMAGE_TAG}", "Image and tag to use for webhooks")
	secretName    = flag.String("secretname", "webhook-cert", "Secret where TLS certs are created")
	caBundleName  = flag.String("cabundlename", "webhook-cert", "ConfigMap where CA cert is created")
	templateFile  = flag.String("outfile", "", "Path to where the SelectorSyncSet template should be written")
	excludes      = flag.String("exclude", "echo-hook", "Comma-separated list of webhook names to skip")
	only          = flag.String("only", "", "Only include these comma-separated webhooks")
	showHookNames = flag.Bool("showhooks", false, "Print registered webhook names and exit")

	namespace = flag.String("namespace", "openshift-validation-webhook", "In what namespace should resources exist?")

	sssLabels = map[string]string{
		"managed.openshift.io/gitHash":     "${IMAGE_TAG}",
		"managed.openshift.io/gitRepoName": "${REPO_NAME}",
		"managed.openshift.io/osd":         "true",
	}
)

func readHooks() map[string]webhooks.WebhookFactory {
	return webhooks.Webhooks
}

func createServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "validation-webhook",
			Namespace: *namespace,
		},
	}
}

func createClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "webhook-validation-cr",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"admissionregistration.k8s.io"},
				Resources: []string{"validatingwebhookconfigurations"},
				Verbs:     []string{"list", "patch", "get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"list", "get"},
			},
		},
	}
}
func createClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "webhook-validation",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "webhook-validation-cr",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "validation-webhook",
				Namespace: *namespace,
			},
		},
	}
}
func createNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: *namespace,
			Labels: map[string]string{
				"openshift.io/cluster-monitoring": "true",
			},
		},
	}
}
func createCACertConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
			Name:      "webhook-cert",
			Namespace: *namespace,
		},
	}
}

func createDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app":        "validation-webhook",
				"deployment": "validation-webhook",
			},
			Name:      "validation-webhook",
			Namespace: *namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "validation-webhook",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "validation-webhook",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "validation-webhook",
					RestartPolicy:      corev1.RestartPolicyAlways,
					Volumes: []corev1.Volume{
						{
							Name: "service-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: *secretName,
								},
							},
						},
						{
							Name: "service-ca",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: *caBundleName,
									},
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Image: *image,
							Name:  "inject-cert",
							Command: []string{
								"injector",
							},
						},
					},
					Containers: []corev1.Container{
						{
							ImagePullPolicy: corev1.PullAlways,
							Name:            "webhooks",
							Image:           *image,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "service-certs",
									MountPath: "/service-certs",
									ReadOnly:  true,
								},
								{
									Name:      "service-ca",
									MountPath: "/service-ca",
									ReadOnly:  true,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(*listenPort),
								},
							},
							Command: []string{
								"webhooks",
								"-tlskey", "/service-certs/tls.key",
								"-tlscert", "/service-certs/tls.crt",
								"-cacert", "/service-ca/service-ca.crt",
								"-tls",
							},
						},
					},
				},
			},
		},
	}
}

func createService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": *secretName,
			},
			Labels: map[string]string{
				"name": "validation-webhook",
			},
			Name:      "validation-webhook",
			Namespace: *namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "validation-webhook",
			},
			Ports: []corev1.ServicePort{
				{
					Name: "https",
					Port: int32(*listenPort),
					TargetPort: intstr.IntOrString{
						IntVal: 443,
						Type:   intstr.Int,
					},
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}

// hookToResources turns a Webhook into a ValidatingWebhookConfiguration and Service.
// The Webhook is expected to implement Rules() which will return a
func createValidatingWebhookConfiguration(hook webhooks.Webhook) admissionregv1beta1.ValidatingWebhookConfiguration {
	failPolicy := hook.FailurePolicy()

	return admissionregv1beta1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ValidatingWebhookConfiguration",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("sre-%s", hook.Name()),
			Annotations: map[string]string{
				"managed.openshift.io/inject-cabundle-from": fmt.Sprintf("%s/webhook-cert", *namespace),
			},
		},
		Webhooks: []admissionregv1beta1.ValidatingWebhook{
			{
				MatchPolicy:   hook.MatchPolicy(),
				Name:          fmt.Sprintf("%s.managed.openshift.io", hook.Name()),
				FailurePolicy: &failPolicy,
				ClientConfig: admissionregv1beta1.WebhookClientConfig{
					Service: &admissionregv1beta1.ServiceReference{
						Namespace: *namespace,
						Path:      pointer.StringPtr(hook.GetURI()),
						Name:      hook.Name(),
					},
				},
				Rules: hook.Rules(),
			},
		},
	}
}

func encode(obj interface{}) []byte {
	o, err := json.Marshal(obj)
	if err != nil {
		fmt.Printf("Error encoding %+v\n", obj)
		os.Exit(1)
	}
	return o
}

func sliceContains(needle string, haystack []string) bool {
	for _, hay := range haystack {
		if hay == needle {
			return true
		}
	}
	return false
}

func createSelectorSyncSet(resources []runtime.RawExtension) *hivev1.SelectorSyncSet {
	return &hivev1.SelectorSyncSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SelectorSyncSet",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "managed-cluster-validating-webhooks",
			Labels: map[string]string{
				"managed.openshift.io/gitHash":     "${IMAGE_TAG}",
				"managed.openshift.io/gitRepoName": "${REPO_NAME}",
				"managed.openshift.io/osd":         "true",
			},
		},
		Spec: hivev1.SelectorSyncSetSpec{
			SyncSetCommonSpec: hivev1.SyncSetCommonSpec{
				ResourceApplyMode: hivev1.SyncResourceApplyMode,
				Resources:         resources,
			},
			ClusterDeploymentSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"api.openshift.com/managed": "true",
				},
			},
		},
	}
}
func main() {
	flag.Parse()

	skip := strings.Split(*excludes, ",")
	onlyInclude := strings.Split(*only, "")

	encoded := make([]runtime.RawExtension, 0)
	encoded = append(encoded, runtime.RawExtension{Object: createNamespace()})
	encoded = append(encoded, runtime.RawExtension{Object: createServiceAccount()})
	encoded = append(encoded, runtime.RawExtension{Object: createClusterRole()})
	encoded = append(encoded, runtime.RawExtension{Object: createClusterRoleBinding()})
	encoded = append(encoded, runtime.RawExtension{Object: createCACertConfigMap()})
	encoded = append(encoded, runtime.RawExtension{Object: createService()})
	encoded = append(encoded, runtime.RawExtension{Object: createDeployment()})
	for _, hook := range webhooks.Webhooks {
		// no rules...?
		if len(hook().Rules()) == 0 {
			continue
		}

		if *showHookNames {
			fmt.Println(hook().Name())
		}
		if sliceContains(hook().Name(), skip) {
			continue
		}
		if len(onlyInclude) > 0 {
			if sliceContains(hook().Name(), onlyInclude) {
				encoded = append(encoded, runtime.RawExtension{Raw: encode(createValidatingWebhookConfiguration(hook()))})
			}
			continue
		}
		// can't use RawExtension{Object: } here because the VWC doesn't implement DeepCopyObject
		encoded = append(encoded, runtime.RawExtension{Raw: encode(createValidatingWebhookConfiguration(hook()))})
	}
	if *showHookNames {
		os.Exit(0)
	}
	if *templateFile == "" {
		fmt.Printf("Expected -outfile option\n\n")
		flag.Usage()
		os.Exit(1)
	}

	sss := createSelectorSyncSet(encoded)

	te := templatev1.Template{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Template",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "selectorsyncset-templat",
		},
		Parameters: []templatev1.Parameter{
			{
				Name:     "IMAGE_TAG",
				Required: true,
			},
			{
				Name:     "REPO_NAME",
				Required: true,
				Value:    "managed-cluster-validating-webhooks",
			},
		},
		Objects: []runtime.RawExtension{
			{
				Raw: encode(sss),
			},
		},
	}

	y, err := yaml.Marshal(te)
	if err != nil {
		fmt.Printf("couldn't marshal: %s\n", err.Error())
	}

	err = ioutil.WriteFile(*templateFile, y, 0644)
	if err != nil {
		fmt.Printf("Failed to write to %s: %s\n", *templateFile, err.Error())
	}
}
