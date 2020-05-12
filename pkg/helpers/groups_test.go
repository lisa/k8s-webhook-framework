package helpers

import (
	"testing"
)

func TestAmDedicatedAdmin(t *testing.T) {
	tests := []struct {
		groups         []string
		expectedResult bool
	}{
		{
			groups:         []string{"test-group"},
			expectedResult: false,
		},
		{
			groups:         []string{"test-group", "dedicated-admins"},
			expectedResult: true,
		},
	}

	for _, test := range tests {
		if IsDedicatedAdmin(test.groups) != test.expectedResult {
			t.Fatalf("expected %t, got %t from this group list: %s", test.expectedResult, IsDedicatedAdmin(test.groups), test.groups)
		}
	}
}

// TestConstantMatches will keep us informed if the constant changes since a
// bunch of things are predicated on it having a certain value.
func TestConstantMatches(t *testing.T) {
	if DedicatedAdminGroupName != "dedicated-admins" {
		t.Fatalf("Expected the DedicatedAdminGroupName constant to be 'dedicated-admins', but got %s", DedicatedAdminGroupName)
	}
}
