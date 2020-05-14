package group

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sync"
	"time"

	responsehelper "github.com/lisa/k8s-webhook-framework/pkg/helpers"
	"github.com/lisa/k8s-webhook-framework/pkg/webhooks/utils"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// GroupWebhook validates a Namespace change
type GroupWebhook struct {
	mu sync.Mutex
	s  runtime.Scheme
}

// GroupRequest represents a fragment of the data sent as part as part of
// the request
type groupRequest struct {
	Metadata struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		UID               string    `json:"uid"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
	} `json:"metadata"`
	Users []string `json:"users"`
}

const (
	webhookName     string = "group_validator"
	protectedGroups string = `(^osd-sre.*|^dedicated-admins$|^cluster-admins$|^layered-cs-sre-admins$)`
)

var (
	protectedGroupsRe = regexp.MustCompile(protectedGroups)
	clusterAdminUsers = []string{"kube:admin", "system:admin"}
	adminGroups       = []string{"osd-sre-admins,osd-sre-cluster-admins"}
)

// GetURI - where am I?
func (s *GroupWebhook) GetURI() string {
	return "/group-validation"
}

// Is the request authorized?
func (s *GroupWebhook) authorized(request admissionctl.Request) (bool, error) {
	// Cluster admins can do anything
	if utils.SliceContains(request.AdmissionRequest.UserInfo.Username, clusterAdminUsers) {
		return true, nil
	}
	group := &groupRequest{}
	err := json.Unmarshal(request.Object.Raw, group)
	if err != nil {
		return false, err
	}
	if protectedGroupsRe.Match([]byte(group.Metadata.Name)) {
		// protected group trying to be accessed, so let's check
		for _, usersgroup := range request.AdmissionRequest.UserInfo.Groups {
			// are they an admin?
			return utils.SliceContains(usersgroup, adminGroups), nil
		}
	}
	// it isn't protected, so let's not be bothered
	return true, nil
}

// Validate - Make sure we're working with a well-formed Admission Request object
func (s *GroupWebhook) Validate(req admissionctl.Request) bool {
	valid := true
	valid = valid && (req.UserInfo.Username != "")
	valid = valid && (req.Kind.Kind == "Group")

	return valid
}

// HandleRequest Decide if the incoming request is allowed
// Based on https://github.com/openshift/managed-cluster-validating-webhooks/blob/33aae59f588643fb8d1fe19cea9572c759586dd6/src/webhook/group_validation.py
func (s *GroupWebhook) HandleRequest(w http.ResponseWriter, r *http.Request) {
	var log = logf.Log.WithName(webhookName)
	s.mu.Lock()
	defer s.mu.Unlock()
	request, response, err := utils.ParseHTTPRequest(r)
	if err != nil {
		log.Error(err, "Error parsing HTTP Request Body")
		responsehelper.SendResponse(w, admissionctl.Errored(http.StatusBadRequest, err))
		return
	}
	// Is this a valid request?
	if !s.Validate(request) {
		responsehelper.SendResponse(w, admissionctl.Errored(http.StatusBadRequest, err))
		return
	}
	// should the request be authorized?
	response.AdmissionResponse.Allowed, err = s.authorized(request)
	responsehelper.SendResponse(w, response)
}

// NewWebhook creates a new webhook
func NewWebhook() *GroupWebhook {
	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)

	return &GroupWebhook{
		s: *scheme,
	}
}
