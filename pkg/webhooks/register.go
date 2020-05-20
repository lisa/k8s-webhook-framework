package webhooks

import (
	"net/http"

	admissionregv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Webhooks are all registered webhooks mapping name to hook
var Webhooks = map[string]WebhookFactory{}

// Webhook interface
type Webhook interface {
	// HandleRequest handles an incoming webhook
	HandleRequest(http.ResponseWriter, *http.Request)
	// GetURI returns the URI for the webhook
	GetURI() string
	// Validate will validate the incoming request
	Validate(admissionctl.Request) bool
	// Name is the name of the webhook
	Name() string
	// FailurePolicy is how the hook config should react if k8s can't access it
	FailurePolicy() admissionregv1beta1.FailurePolicyType
	// MatchPolicy mirrors validatingwebhookconfiguration.webhooks[].matchPolicy.
	// If it is important to the webhook, be sure to check subResource vs
	// requestSubResource.
	MatchPolicy() *admissionregv1beta1.MatchPolicyType
	// Rules is a slice of rules on which this hook should trigger
	Rules() []admissionregv1beta1.RuleWithOperations
}

// WebhookFactory return a kind of Webhook
type WebhookFactory func() Webhook

// Register webhooks
func Register(name string, input WebhookFactory) {
	Webhooks[name] = input
}
