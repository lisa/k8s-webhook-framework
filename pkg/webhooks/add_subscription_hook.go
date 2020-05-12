package webhooks

import (
	"github.com/lisa/k8s-webhook-framework/pkg/webhooks/subscription"
)

func init() {
	Register("namespace", func() Webhook { return subscription.NewWebhook() })
}
