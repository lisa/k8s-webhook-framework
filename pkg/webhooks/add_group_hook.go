package webhooks

import "github.com/lisa/k8s-webhook-framework/pkg/webhooks/group"

func init() {
	Register("group", func() Webhook { return group.NewWebhook() })
}
