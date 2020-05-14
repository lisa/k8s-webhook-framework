package group

import (
	"fmt"
	"testing"

	"github.com/lisa/k8s-webhook-framework/pkg/testutils"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Raw JSON for a Namespace, used as runtime.RawExtension, and represented here
// because sometimes we need it for OldObject as well as Object.
const testGroupRaw string = `{
  "metadata": {
    "name": "%s",
    "uid": "%s",
    "creationTimestamp": "2020-05-10T07:51:00Z"
  },
  "users": null
}`

type groupTestsuites struct {
	testID          string
	groupName       string
	username        string
	userGroups      []string
	operation       v1beta1.Operation
	shouldBeAllowed bool
}

func runGroupTests(t *testing.T, tests []groupTestsuites) {
	gvk := metav1.GroupVersionKind{
		Group:   "",
		Version: "v1beta1",
		Kind:    "Group",
	}
	gvr := metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1beta1",
		Resource: "groups",
	}
	for _, test := range tests {
		rawObjString := fmt.Sprintf(testGroupRaw, test.groupName, test.testID)
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
			t.Fatalf("Mismatch: %s (groups=%s) %s %s the %s Group. Test's expectation is that the user %s",
				test.username, test.userGroups, testutils.CanCanNot(response.Allowed), string(test.operation), test.groupName, testutils.CanCanNot(test.shouldBeAllowed))
		}

	}
}

func TestAdminUsers(t *testing.T) {
	tests := []groupTestsuites{
		{
			// Should be able to do everything
			testID:          "dedi-create-nonpriv-ns",
			groupName:       "osd-sre-admins",
			username:        "kube:admin",
			userGroups:      []string{"system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
	}
	runGroupTests(t, tests)
}
func TestDedicatedAdminUsers(t *testing.T) {
	tests := []groupTestsuites{
		{
			// Should be able to do nothing
			testID:          "dedi-create-nonpriv-ns",
			groupName:       "osd-sre-admins",
			username:        "dedi-admin",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: false,
		},
		{
			testID:          "dedi-create-nonpriv-ns",
			groupName:       "my-group",
			username:        "dedi-admin",
			userGroups:      []string{"dedicated-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
	}
	runGroupTests(t, tests)
}
func TestSREAdminUsers(t *testing.T) {
	tests := []groupTestsuites{
		{
			// Should be able to do everything
			testID:          "dedi-create-nonpriv-ns",
			groupName:       "osd-sre-admins",
			username:        "kube:admin",
			userGroups:      []string{"osd-sre-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
		{
			testID:          "dedi-create-nonpriv-ns",
			groupName:       "my-group",
			username:        "kube:admin",
			userGroups:      []string{"osd-sre-admins", "system:authenticated", "system:authenticated:oauth"},
			operation:       v1beta1.Update,
			shouldBeAllowed: true,
		},
	}
	runGroupTests(t, tests)
}
