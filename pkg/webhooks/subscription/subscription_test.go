package subscription

import (
	"fmt"
	"testing"

	"github.com/lisa/k8s-webhook-framework/pkg/testutils"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Raw JSON for a Subscription, used as runtime.RawExtension, and represented here
// because sometimes we need it for OldObject as well as Object.
const testSubscriptionRaw string = `{
  "metadata": {
		"namespace": "%s",
		"uid": "%s",
		"name": "%s",
    "creationTimestamp": "2020-05-10T07:51:00Z"
  },
  "users": null
}`

type subscriptionTestSuites struct {
	testID           string
	targetNamespace  string
	subscriptionName string
	username         string
	userGroups       []string
	operation        v1beta1.Operation
	shouldBeAllowed  bool
}

func runSubscriptionTests(t *testing.T, tests []subscriptionTestSuites) {
	gvk := metav1.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "Subscription",
	}
	gvr := metav1.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	}

	for _, test := range tests {
		rawObjString := fmt.Sprintf(testSubscriptionRaw, test.targetNamespace, test.testID, test.subscriptionName)
		obj := runtime.RawExtension{
			Raw: []byte(rawObjString),
		}
		hook := NewWebhook()
		httprequest, err := testutils.CreateHTTPRequest(hook.GetURI(),
			test.testID,
			gvk, gvr, test.operation, test.username, test.userGroups, obj)
		if err != nil {
			t.Fatalf("Expected no error, got %s", err.Error())
		}

		response, err := testutils.SendHTTPRequest(httprequest, hook)
		if err != nil {
			t.Fatalf("Expected no error, got %s", err.Error())
		}
		if response.UID == "" {
			t.Fatalf("No tracking UID associated with the response.")
		}

		if response.Allowed != test.shouldBeAllowed {
			t.Fatalf("Mismatch: %s (groups=%s) %s %s the %s namespace. Test's expectation is that the user %s", test.username, test.userGroups, testutils.CanCanNot(response.Allowed), string(test.operation), test.targetNamespace, testutils.CanCanNot(test.shouldBeAllowed))
		}
	}
}

func TestDedicatedAdmin(t *testing.T) {
	tests := []subscriptionTestSuites{
		{
			// Should be able to make changes to openshift-marketplace
			testID:          "dedi-update-openshift-marketplace",
			targetNamespace: "openshift-marketplace",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
		{
			// Shouldn't be able to make changes to other places
			testID:          "dedi-create-other-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: false,
		},
		{
			// Should be able to make changes to openshift-marketplace
			testID:          "dedi-create-other-ns",
			targetNamespace: "openshift-marketplace",
			username:        "test-user",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: true,
		},
	}
	runSubscriptionTests(t, tests)
}

// Any authenticated user that is allowed through k8s RBAC should be able
// to make any other changes.
func TestOtherUser(t *testing.T) {
	tests := []subscriptionTestSuites{
		{
			testID:          "normaluser-update-openshift-marketplace",
			targetNamespace: "openshift-marketplace",
			username:        "test-user",
			userGroups:      []string{"system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
		{
			testID:          "normaluser-create-my-ns",
			targetNamespace: "my-ns",
			username:        "test-user",
			userGroups:      []string{"system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Create,
			shouldBeAllowed: true,
		},
	}
	runSubscriptionTests(t, tests)
}
