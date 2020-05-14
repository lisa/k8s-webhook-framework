package subscription

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	responsehelper "github.com/lisa/k8s-webhook-framework/pkg/helpers"
	"github.com/lisa/k8s-webhook-framework/pkg/webhooks/utils"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	webhookName                                           string = "subscription_validator"
	defaultSafelistedDedicatedAdminsSubscriptionNamespace string = "openshift-marketplace"
)

// SubscriptionWebhook to handle the thing
type SubscriptionWebhook struct {
	mu sync.Mutex
	s  runtime.Scheme
}

var log = logf.Log.WithName(webhookName)

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

func (s *SubscriptionWebhook) authorized(request admissionctl.Request) admissionctl.Response {
	var ret admissionctl.Response

	ns, set := os.LookupEnv("SUBSCRIPTION_VALIDATION_NAMESPACES")
	if !set {
		ns = defaultSafelistedDedicatedAdminsSubscriptionNamespace
	}
	safelistedNamespaces := strings.Split(ns, ",")

	sub := &subscriptionRequest{}
	// If the user is a dedicated admin, they may only make changes to
	// Subscriptions in SUBSCRIPTION_VALIDATION_NAMESPACES namespace(s)
	if responsehelper.IsDedicatedAdmin(request.UserInfo.Groups) {
		err := json.Unmarshal(request.Object.Raw, sub)
		if err != nil {
			log.Error(err, "Couldn't parse Subscription information from request")
			ret = admissionctl.Errored(http.StatusBadRequest, err)
			ret.UID = request.AdmissionRequest.UID
			return ret
		}
		log.Info(fmt.Sprintf("Checking if dedicated admin %s can %s a Subscription (name=%s) in namespace %s (Safelisted=%s)", request.UserInfo.Username, request.Operation, sub.Metadata.Name, sub.Metadata.Namespace, safelistedNamespaces))
		// For a dedicated admin, check to see if the Subscription in question is one of
		// the safelisted ones they can access
		if utils.SliceContains(sub.Metadata.Namespace, safelistedNamespaces) {
			ret = admissionctl.Allowed("Dedicated-admin may access")
			ret.UID = request.AdmissionRequest.UID
			return ret
		}
		ret = admissionctl.Denied("Dedicaed-admins may not access")
		ret.UID = request.AdmissionRequest.UID
		return ret
	}
	// Getting here means normal RBAC let us do the thing
	ret = admissionctl.Allowed("RBAC allowed")
	ret.UID = request.AdmissionRequest.UID
	return ret
}

// HandleRequest handle it
func (s *SubscriptionWebhook) HandleRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, _, err := utils.ParseHTTPRequest(r)
	if err != nil {
		log.Error(err, "Error parsing HTTP Request Body")
		responsehelper.SendResponse(w, admissionctl.Errored(http.StatusBadRequest, err))
		return
	}
	if !s.Validate(request) {
		responsehelper.SendResponse(w,
			admissionctl.Errored(http.StatusBadRequest,
				fmt.Errorf("Could not parse Subscription from request")))
		return
	}

	responsehelper.SendResponse(w, s.authorized(request))
}

func NewWebhook() Webhook {
	scheme := runtime.NewScheme()
	return &SubscriptionWebhook{
		s: *scheme,
	}
}
