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

	if !s.Validate(request) {
		response.AdmissionResponse.Allowed = false
		responsehelper.SendResponse(w, response)
		return
	}

	ns, err := s.renderNamespace(request)
	if err != nil {
		log.Error(err, "Couldn't parse any Namespace from the request")
		response.AdmissionResponse.Allowed = false
		responsehelper.SendResponse(w, response)
		return
	}
	// L49-L56
	// TODO: This does not seem to make sense. Why would the groups match against the username?
	for _, group := range request.UserInfo.Groups {
		if privilegedServiceAccountsRe.Match([]byte(group)) {
			response.AdmissionResponse.Allowed = true
			log.Info("L49-L56 Odd check", "namespace", ns.GetName(), "username", request.UserInfo.Username)
			responsehelper.SendResponse(w, response)
			return
		}
	}
	// L58-L62
	if sliceContains(layeredProductAdminGroupName, request.UserInfo.Groups) &&
		layeredProductNamespaceRe.Match([]byte(ns.GetName())) {
		log.Info("L58-L62 Layered Product", "namespace", ns.GetName(), "username", request.UserInfo.Username)
		response.AdmissionResponse.Allowed = true
		responsehelper.SendResponse(w, response)
		return
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
		response.AdmissionResponse.Allowed = amIClusterAdmin || amISREAdmin
		log.Info("L64-L73 Privileged Namespace", "namespace", ns.GetName(), "allowing", response.AdmissionResponse.Allowed, "username", request.UserInfo.Username, "namespace", request.Name, "clusterAdmin", amIClusterAdmin, "SREAdmin", amISREAdmin)
		responsehelper.SendResponse(w, response)
		return
	}
	// L75-L77
	log.Info("L75-L77 Responding allow", "userInfo", request.UserInfo, "namespace", ns.GetName())
	response.AdmissionResponse.Allowed = true
	responsehelper.SendResponse(w, response)

}

func init() {
	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	Register(webhookName, func() Webhook { return &NamespaceWebhook{s: *scheme} })
}
