package webhooks

import (
	"net/http"

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
}

// WebhookFactory return a kind of Webhook
type WebhookFactory func() Webhook

// Register webhooks
func Register(name string, input WebhookFactory) {
	Webhooks[name] = input
}
