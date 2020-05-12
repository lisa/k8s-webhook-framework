package webhooks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	responsehelper "github.com/lisa/k8s-webhook-framework/pkg/helpers"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const defaultSafelistedDedicatedAdminsSubscriptionNamespace string = "openshift-marketplace"

// SubscriptionWebhook to handle the thing
type SubscriptionWebhook struct {
	mu sync.Mutex
	s  runtime.Scheme
}

// subscriptionRequest represents a fragment of the data sent as part as part of
// the request
type subscriptionRequest struct {
	Metadata struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		UID               string    `json:"uid"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
	} `json:"metadata"`
	Users []string `json:"users"`
}

// Validate - Make sure we're working with a well-formed Admission Request object
func (s *SubscriptionWebhook) Validate(req admissionctl.Request) bool {
	valid := true
	valid = valid && (req.UserInfo.Username != "")
	valid = valid && (req.Kind.Kind == "Subscription")

	return valid
}

// GetURI I answer at this URI
func (s *SubscriptionWebhook) GetURI() string {
	return "/subscription-validation"
}

// HandleRequest handle it
func (s *SubscriptionWebhook) HandleRequest(w http.ResponseWriter, r *http.Request) {
	var log = logf.Log.WithName("subscription_validator")
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, set := os.LookupEnv("SUBSCRIPTION_VALIDATION_NAMESPACES")
	if !set {
		ns = defaultSafelistedDedicatedAdminsSubscriptionNamespace
	}
	safelistedNamespaces := strings.Split(ns, ",")

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
	sub := &subscriptionRequest{}
	// If the user is a dedicated admin, they may only make changes to
	// Subscriptions in SUBSCRIPTION_VALIDATION_NAMESPACES namespace(s)
	if responsehelper.IsDedicatedAdmin(request.UserInfo.Groups) {
		err := json.Unmarshal(request.Object.Raw, sub)
		if err != nil {
			log.Error(err, "Couldn't parse Subscription information from request")
			responsehelper.SendResponse(w, response)
			return
		}
		log.Info(fmt.Sprintf("Checking if dedicated admin %s can %s a Subscription (name=%s) in namespace %s (Safelisted=%s)", request.UserInfo.Username, request.Operation, sub.Metadata.Name, sub.Metadata.Namespace, safelistedNamespaces))
		// For a dedicated admin, check to see if the Subscription in question is one of
		// the safelisted ones they can access
		response.AdmissionResponse.Allowed = sliceContains(sub.Metadata.Namespace, safelistedNamespaces)
	} else {
		// Getting here means normal RBAC let us do the thing
		log.Info("Not a dedicated admin. Allowing", "namespace", sub.Metadata.Namespace, "UserInfo", request.UserInfo)
		response.AdmissionResponse.Allowed = true
	}
	responsehelper.SendResponse(w, response)
}

func init() {
	scheme := runtime.NewScheme()
	Register("subscription_webhook", func() Webhook { return &SubscriptionWebhook{s: *scheme} })
}
