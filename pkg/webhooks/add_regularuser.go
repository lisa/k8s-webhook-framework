package webhooks

import (
	"github.com/lisa/k8s-webhook-framework/pkg/webhooks/regularuser"
)

func init() {
	Register(regularuser.WebhookName, func() Webhook { return regularuser.NewWebhook() })
}
