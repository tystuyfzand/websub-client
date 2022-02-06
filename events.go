package client

import "meow.tf/websub/model"

type SubscriptionDenied struct {
    Subscription *model.Subscription
    Reason string
}

// Publish is called when items are published
type Publish struct {
    Subscription model.Subscription
    ContentType string
    Data        []byte
}