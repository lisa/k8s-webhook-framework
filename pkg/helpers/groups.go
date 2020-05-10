package helpers

// DedicatedAdminGroupName is the name of the dedicaed admins group name
const DedicatedAdminGroupName string = "dedicated-admins"

// IsDedicatedAdmin Is the user a dedicated admin?
func IsDedicatedAdmin(groupList []string) bool {
	for _, group := range groupList {
		if group == DedicatedAdminGroupName {
			return true
		}
	}
	return false
}
