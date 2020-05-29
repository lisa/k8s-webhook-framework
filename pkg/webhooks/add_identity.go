package webhooks

import (
	"github.com/lisa/k8s-webhook-framework/pkg/webhooks/identity"
)

func init() {
	Register(identity.WebhookName, func() Webhook { return identity.NewWebhook() })
}
