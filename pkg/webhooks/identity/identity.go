package identity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	responsehelper "github.com/lisa/k8s-webhook-framework/pkg/helpers"
	"github.com/lisa/k8s-webhook-framework/pkg/webhooks/utils"
	"k8s.io/api/admission/v1beta1"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	WebhookName             string = "identity-validation"
	defaultIdentityProvider string = "OpenShift_SRE"
)

var (
	privilegedUsers = []string{"kube:admin", "system:admin", "system:serviceaccount:openshift-authentication:oauth-openshift"}
	adminGroups     = []string{"osd-sre-admins", "osd-sre-cluster-admins"}

	log = logf.Log.WithName(WebhookName)

	sideEffects = admissionregv1.SideEffectClassNone
	matchPolicy = admissionregv1.Exact
	scope       = admissionregv1.ClusterScope
	rules       = []admissionregv1.RuleWithOperations{
		{
			Operations: []admissionregv1.OperationType{"UPDATE", "CREATE", "DELETE"},
			Rule: admissionregv1.Rule{
				APIGroups:   []string{"user.openshift.io"},
				APIVersions: []string{"*"},
				Resources:   []string{"identities"},
				Scope:       &scope,
			},
		},
	}
)

type identityRequest struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	ProviderName string `json:"providerName"`
}

// IdentityWebhook validates a Namespace change
type IdentityWebhook struct {
	mu sync.Mutex
	s  runtime.Scheme
}

func (s *IdentityWebhook) TimeoutSeconds() int32                        { return 2 }
func (s *IdentityWebhook) SideEffects() *admissionregv1.SideEffectClass { return &sideEffects }
func (s *IdentityWebhook) MatchPolicy() *admissionregv1.MatchPolicyType { return &matchPolicy }
func (s *IdentityWebhook) Rules() []admissionregv1.RuleWithOperations {
	return rules
}

func (s *IdentityWebhook) FailurePolicy() admissionregv1.FailurePolicyType {
	return admissionregv1.Ignore
}

func (s *IdentityWebhook) Name() string {
	return WebhookName
}

// Validate - Make sure we're working with a well-formed Admission Request object
func (s *IdentityWebhook) Validate(req admissionctl.Request) bool {
	valid := true
	valid = valid && (req.UserInfo.Username != "")
	valid = valid && (req.Kind.Kind == "Identity")

	return valid
}

// GetURI where am I?
func (s *IdentityWebhook) GetURI() string {
	return "/identity-validation"
}

// Is the request authorized?
func (s *IdentityWebhook) authorized(request admissionctl.Request) admissionctl.Response {
	var ret admissionctl.Response
	var err error
	idReq := &identityRequest{}

	// if we delete, then look to OldObject in the request.
	if request.Operation == v1beta1.Delete {
		err = json.Unmarshal(request.OldObject.Raw, idReq)
	} else {
		err = json.Unmarshal(request.Object.Raw, idReq)
	}
	if err != nil {
		ret = admissionctl.Errored(http.StatusBadRequest, err)
		return ret
	}
	// Admin user
	if utils.SliceContains(request.AdmissionRequest.UserInfo.Username, privilegedUsers) {
		ret = admissionctl.Allowed("Allowed")
		ret.UID = request.AdmissionRequest.UID
		return ret
	}
	if idReq.ProviderName == defaultIdentityProvider {
		for _, group := range request.AdmissionRequest.UserInfo.Groups {
			if utils.SliceContains(group, adminGroups) {
				ret = admissionctl.Allowed("")
				ret.UID = request.AdmissionRequest.UID
				return ret
			}
		}
		ret = admissionctl.Denied("Permission denied")
		ret.UID = request.AdmissionRequest.UID
		return ret
	}

	ret = admissionctl.Allowed("Allowed by RBAC")
	ret.UID = request.AdmissionRequest.UID
	return ret

}

// HandleRequest Decide if the incoming request is allowed
func (s *IdentityWebhook) HandleRequest(w http.ResponseWriter, r *http.Request) {

	s.mu.Lock()
	defer s.mu.Unlock()
	request, _, err := utils.ParseHTTPRequest(r)
	if err != nil {
		log.Error(err, "Error parsing HTTP Request Body")
		responsehelper.SendResponse(w, admissionctl.Errored(http.StatusBadRequest, err))
		return
	}
	// Is this a valid request?
	if !s.Validate(request) {
		responsehelper.SendResponse(w,
			admissionctl.Errored(http.StatusBadRequest,
				fmt.Errorf("Could not parse Namespace from request")))
		return
	}
	// should the request be authorized?
	responsehelper.SendResponse(w, s.authorized(request))
}

// NewWebhook creates a new webhook
func NewWebhook() *IdentityWebhook {
	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)

	return &IdentityWebhook{
		s: *scheme,
	}
}
