package namespace

import (
	"net/http"

	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Webhook interface
type Webhook interface {
	// HandleRequest handles an incoming webhook
	HandleRequest(http.ResponseWriter, *http.Request)
	// GetURI returns the URI for the webhook
	GetURI() string
	// Validate will validate the incoming request
	Validate(admissionctl.Request) bool
}
