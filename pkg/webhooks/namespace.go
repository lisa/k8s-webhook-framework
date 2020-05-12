package webhooks

import (
	"net/http"
	"regexp"
	"sync"

	responsehelper "github.com/lisa/k8s-webhook-framework/pkg/helpers"
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
func (s *NamespaceWebhook) authorized(request admissionctl.Request) (bool, error) {

	ns, err := s.renderNamespace(request)
	if err != nil {
		return false, err
	}
	// L49-L56
	// service accounts making requests will include their name in the group
	for _, group := range request.UserInfo.Groups {
		if privilegedServiceAccountsRe.Match([]byte(group)) {
			return true, nil
		}
	}
	// L58-L62
	// This must be prior to privileged namespace check
	if sliceContains(layeredProductAdminGroupName, request.UserInfo.Groups) &&
		layeredProductNamespaceRe.Match([]byte(ns.GetName())) {
		return true, nil
	}
	// L64-73
	if privilegedNamespaceRe.Match([]byte(ns.GetName())) {
		amISREAdmin := false
		amIClusterAdmin := sliceContains(request.UserInfo.Username, clusterAdminUsers)

		for _, group := range sreAdminGroups {
			if sliceContains(group, request.UserInfo.Groups) {
				amISREAdmin = true
				break
			}
		}
		return (amIClusterAdmin || amISREAdmin), nil
	}
	// L75-L77
	return true, nil
}

// HandleRequest Decide if the incoming request is allowed
// Based on https://github.com/openshift/managed-cluster-validating-webhooks/blob/ad1ecb38621c485b5832eea729244e3b5ef354cc/src/webhook/namespace_validation.py
func (s *NamespaceWebhook) HandleRequest(w http.ResponseWriter, r *http.Request) {
	var log = logf.Log.WithName(webhookName)
	s.mu.Lock()
	defer s.mu.Unlock()
	request, response, err := parseHTTPRequest(r)
	if err != nil {
		log.Error(err, "Error parsing HTTP Request Body")
		responsehelper.SendResponse(w, response)
		return
	}
	// Is this a valid request?
	if !s.Validate(request) {
		response.AdmissionResponse.Allowed = false
		responsehelper.SendResponse(w, response)
		return
	}
	// should the request be authorized?
	response.AdmissionResponse.Allowed, err = s.authorized(request)
	if err != nil {
		log.Error(err, "Error in authorizing: %s", err.Error())
		response.AdmissionResponse.Allowed = false
		responsehelper.SendResponse(w, response)
		return
	}
	responsehelper.SendResponse(w, response)

}

func init() {
	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	Register(webhookName, func() Webhook { return &NamespaceWebhook{s: *scheme} })
}
