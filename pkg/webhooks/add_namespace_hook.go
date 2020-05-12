package webhooks

import (
	"github.com/lisa/k8s-webhook-framework/pkg/webhooks/namespace"
)

func init() {
	Register("namespace", func() Webhook { return namespace.NewWebhook() })
}
