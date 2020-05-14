package namespace

import (
	"fmt"
	"net/http"
	"regexp"
	"sync"

	responsehelper "github.com/lisa/k8s-webhook-framework/pkg/helpers"
	"github.com/lisa/k8s-webhook-framework/pkg/webhooks/utils"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	webhookName                  string = "namespace_validator"
	privilegedNamespace          string = `(^kube.*|^openshift.*|^default$|^redhat.*)`
	privilegedServiceAccounts    string = `^system:serviceaccounts:(kube.*|openshift.*|default|redhat.*)`
	layeredProductNamespace      string = `^redhat.*`
	layeredProductAdminGroupName string = "layered-sre-cluster-admins"
)

var (
	clusterAdminUsers = []string{"kube:admin", "system:admin"}
	sreAdminGroups    = []string{"osd-sre-admins", "osd-sre-cluster-admins"}

	privilegedNamespaceRe       = regexp.MustCompile(privilegedNamespace)
	privilegedServiceAccountsRe = regexp.MustCompile(privilegedServiceAccounts)
	layeredProductNamespaceRe   = regexp.MustCompile(layeredProductNamespace)

	log = logf.Log.WithName(webhookName)
)

// NamespaceWebhook validates a Namespace change
type NamespaceWebhook struct {
	mu sync.Mutex
	s  runtime.Scheme
}

// Validate - Make sure we're working with a well-formed Admission Request object
func (s *NamespaceWebhook) Validate(req admissionctl.Request) bool {
	valid := true
	valid = valid && (req.UserInfo.Username != "")
	valid = valid && (req.Kind.Kind == "Namespace")

	return valid
}

// GetURI where am I?
func (s *NamespaceWebhook) GetURI() string {
	return "/namespace-validation"
}

// renderNamespace pluck out the Namespace from the Object or OldObject
func (s *NamespaceWebhook) renderNamespace(req admissionctl.Request) (*corev1.Namespace, error) {
	decoder, err := admissionctl.NewDecoder(&s.s)
	if err != nil {
		return nil, err
	}
	namespace := &corev1.Namespace{}
	if len(req.OldObject.Raw) > 0 {
		err = decoder.DecodeRaw(req.OldObject, namespace)
	} else {
		err = decoder.Decode(req, namespace)
	}
	if err != nil {
		return nil, err
	}
	return namespace, nil
}

// Is the request authorized?
func (s *NamespaceWebhook) authorized(request admissionctl.Request) admissionctl.Response {
	var ret admissionctl.Response
	ns, err := s.renderNamespace(request)
	if err != nil {
		log.Error(err, "Couldn't render a Namespace from the incoming request")
		return admissionctl.Errored(http.StatusBadRequest, err)
	}
	// L49-L56
	// service accounts making requests will include their name in the group
	for _, group := range request.UserInfo.Groups {
		if privilegedServiceAccountsRe.Match([]byte(group)) {
			ret = admissionctl.Allowed("Privileged service accounts may access")
			ret.UID = request.AdmissionRequest.UID
			return ret
		}
	}
	// L58-L62
	// This must be prior to privileged namespace check
	if utils.SliceContains(layeredProductAdminGroupName, request.UserInfo.Groups) &&
		layeredProductNamespaceRe.Match([]byte(ns.GetName())) {
		ret = admissionctl.Allowed("Layered product admins may access")
		ret.UID = request.AdmissionRequest.UID
		return ret
	}
	// L64-73
	if privilegedNamespaceRe.Match([]byte(ns.GetName())) {
		amISREAdmin := false
		amIClusterAdmin := utils.SliceContains(request.UserInfo.Username, clusterAdminUsers)

		for _, group := range sreAdminGroups {
			if utils.SliceContains(group, request.UserInfo.Groups) {
				amISREAdmin = true
				break
			}
		}
		if amIClusterAdmin || amISREAdmin {
			ret = admissionctl.Allowed("Cluster and SRE admins may access")
			ret.UID = request.AdmissionRequest.UID
			return ret
		}
		ret = admissionctl.Denied("Non-admin access attempt to privileged namespace")
		ret.UID = request.AdmissionRequest.UID
		return ret
	}
	// L75-L77
	ret = admissionctl.Allowed("RBAC allowed")
	ret.UID = request.AdmissionRequest.UID
	return ret
}

// HandleRequest Decide if the incoming request is allowed
// Based on https://github.com/openshift/managed-cluster-validating-webhooks/blob/ad1ecb38621c485b5832eea729244e3b5ef354cc/src/webhook/namespace_validation.py
func (s *NamespaceWebhook) HandleRequest(w http.ResponseWriter, r *http.Request) {

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
func NewWebhook() *NamespaceWebhook {
	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	return &NamespaceWebhook{
		s: *scheme,
	}
}
