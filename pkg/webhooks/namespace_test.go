package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// Raw JSON for a Namespace, used as runtime.RawExtension, and represented here
// because sometimes we need it for OldObject as well as Object.
const testNamespaceRaw string = `{
  "metadata": {
    "name": "%s",
    "uid": "%s",
    "creationTimestamp": "2020-05-10T07:51:00Z"
  },
  "users": null
}`

type testSuites struct {
	testID          string
	targetNamespace string
	username        string
	userGroups      []string
	operation       v1beta1.Operation
	shouldBeAllowed bool
}

// helper to make English a bit nicer
func canCanNot(b bool) string {
	if b {
		return "can"
	}
	return "can not"
}

func createFakeNamespaceRequestJSON(uid, namespaceName, username string, operation v1beta1.Operation, userGroups []string) ([]byte, error) {
	req := v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{
			UID: types.UID(uid),
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Namespace",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "namespaces",
			},
			Operation: operation,
			UserInfo: authenticationv1.UserInfo{
				Username: username,
				Groups:   userGroups,
			},
		},
	}

	rawObjString := fmt.Sprintf(testNamespaceRaw, namespaceName, uid)

	obj := runtime.RawExtension{
		Raw: []byte(rawObjString),
	}
	switch operation {
	case v1beta1.Create:
		req.Request.Object = obj
	case v1beta1.Update:
		req.Request.Object = obj
	case v1beta1.Delete:
		req.Request.OldObject = obj
	}
	b, err := json.Marshal(req)
	if err != nil {
		return []byte{}, err
	}
	return b, nil
}

func createHTTPRequest(uid, namespaceName, username string, operation v1beta1.Operation, userGroups []string) (*http.Request, error) {
	req, err := createFakeNamespaceRequestJSON(uid, namespaceName, username, operation, userGroups)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(req)
	httprequest := httptest.NewRequest("POST", "/namespace-validation", buf)
	httprequest.Header["Content-Type"] = []string{"application/json"}
	return httprequest, nil
}

func sendHTTPRequest(req *http.Request) (*v1beta1.AdmissionResponse, error) {

	httpResponse := httptest.NewRecorder()
	s := newNamespaceHook()
	s.HandleRequest(httpResponse, req)
	// at this popint, httpResponse should contain the data sent in response to the webhook query, which is the success/fail
	ret := &v1beta1.AdmissionReview{}
	err := json.Unmarshal(httpResponse.Body.Bytes(), ret)
	if err != nil {
		return nil, err
	}
	return ret.Response, nil
}

func newNamespaceHook() *NamespaceWebhook {
	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	return &NamespaceWebhook{s: *scheme}
}

func runtests(t *testing.T, tests []testSuites) {
	for _, test := range tests {
		httprequest, err := createHTTPRequest(test.testID, test.targetNamespace, test.username, test.operation, test.userGroups)
		if err != nil {
			t.Fatalf("Expected no error, got %s", err.Error())
		}
		response, err := sendHTTPRequest(httprequest)
		if err != nil {
			t.Fatalf("Expected no error, got %s", err.Error())
		}

		if response.Allowed != test.shouldBeAllowed {
			t.Fatalf("Mismatch: %s (groups=%s) %s %s the %s namespace. Test's expectation is that the user %s", test.username, test.userGroups, canCanNot(response.Allowed), string(test.operation), test.targetNamespace, canCanNot(test.shouldBeAllowed))
		}
	}
}

// TestDedicatedAdmins will test everything a dedicated admin can and can not do
func TestDedicatedAdmins(t *testing.T) {
	tests := []testSuites{
		{
			// Should be able to create an unprivileged namespace
			testID:          "dedi-create-nonpriv-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: true,
		},
		{
			// Should not be able to delete a privileged namespace
			testID:          "dedi-delete-priv-ns",
			targetNamespace: "kube-system",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Delete,
			shouldBeAllowed: false,
		},
		{
			// Should not be able to create a privileged namespace
			testID:          "dedi-create-priv-ns",
			targetNamespace: "openshift-test-namespace",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: false,
		},
		{
			// Should not be able to update layered product ecnamespa
			testID:          "dedi-update-layered-prod-ns",
			targetNamespace: "redhat-layered-product-ns",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: false,
		},
		{
			// Should be able to delete a general namespace
			testID:          "dedi-delete-random-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Delete,
			shouldBeAllowed: true,
		},
		{
			// Should be able to updte a general namespace
			testID:          "dedi-update-random-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
	}
	runtests(t, tests)
}

// TestNormalUser will test everything a normal user can and can not do
func TestNormalUser(t *testing.T) {
	tests := []testSuites{
		{
			// Should be able to create an unprivileged namespace
			testID:          "nonpriv-create-nonpriv-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: true,
		},
		{
			// Should be able to create an unprivileged namespace
			testID:          "nonpriv-update-nonpriv-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
		{
			// Should be able to create an unprivileged namespace
			testID:          "nonpriv-delete-nonpriv-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Delete,
			shouldBeAllowed: true,
		},
		{
			// Shouldn't be able to create a privileged namespace
			testID:          "nonpriv-create-priv-ns",
			targetNamespace: "kube-system",
			username:        "test-user",
			userGroups:      []string{"system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: false,
		},
		{
			// Shouldn't be able to delete a privileged namespace
			testID:          "nonpriv-delete-priv-ns",
			targetNamespace: "kube-system",
			username:        "test-user",
			userGroups:      []string{"system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Delete,
			shouldBeAllowed: false,
		},
	}
	runtests(t, tests)
}

// TestLayeredProducts
func TestLayeredProducts(t *testing.T) {
	tests := []testSuites{
		{
			// Layered admins can manipulate in the lp ns, but not privileged ones
			// note: ^redhat.* is a privileged ns, but lp admins have an exception in
			// it (but not other privileged ns)
			testID:          "lp-create-layered-ns",
			targetNamespace: "redhat-layered-product",
			username:        "test-user",
			userGroups:      []string{"layered-sre-cluster-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: true,
		},
		{
			// Layered admins can't create a privileged ns
			testID:          "lp-create-priv-ns",
			targetNamespace: "openshift-test",
			username:        "test-user",
			userGroups:      []string{"layered-sre-cluster-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: false,
		},
		{
			// Layered admins can make an unprivileged ns
			testID:          "lp-create-priv-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"layered-sre-cluster-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: true,
		},
	}
	runtests(t, tests)
}

// TestServiceAccounts
func TestServiceAccounts(t *testing.T) {
	tests := []testSuites{
		{
			// serviceaccounts in privileged namespaces can interact with privileged namespaces
			testID:          "sa-create-priv-ns",
			targetNamespace: "openshift-test-ns",
			username:        "system:serviceaccounts:openshift-test-ns",
			userGroups:      []string{"system:serviceaccounts:openshift-test-ns", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: true,
		},
		{
			// serviceaccounts in privileged namespaces can interact with customer namespaces
			// this is counterintuitive because they shouldn't. Recall that RBAC would
			// deny any disallowed access, so the "true" here is deferring to
			// Kubernetes RBAC
			testID:          "sa-create-priv-ns",
			targetNamespace: "customer-ns",
			username:        "system:serviceaccounts:openshift-test-ns",
			userGroups:      []string{"system:serviceaccounts:openshift-test-ns", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Delete,
			shouldBeAllowed: true,
		},
	}
	runtests(t, tests)
}

// TestAdminUser
func TestAdminUser(t *testing.T) {
	tests := []testSuites{
		{
			// admin users gonna admin
			testID:          "admin-test",
			targetNamespace: "kube-system",
			username:        "kube:admin",
			userGroups:      []string{"kube:system", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
	}
	runtests(t, tests)
}
