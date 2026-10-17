package dto

// PushSubscriptionKeys are the client keys of a PushSubscription (base64url).
type PushSubscriptionKeys struct {
	P256dh string `json:"p256dh" validate:"required"`
	Auth   string `json:"auth" validate:"required"`
}

// SubscribePushRequest is the body of POST /push/subscriptions (the browser's
// PushSubscription.toJSON()).
type SubscribePushRequest struct {
	Endpoint string               `json:"endpoint" validate:"required,url"`
	Keys     PushSubscriptionKeys `json:"keys" validate:"required"`
}

// UnsubscribePushRequest is the body of DELETE /push/subscriptions.
type UnsubscribePushRequest struct {
	Endpoint string `json:"endpoint" validate:"required"`
}

// VAPIDPublicKeyResponse carries the application server key for pushManager.subscribe.
type VAPIDPublicKeyResponse struct {
	PublicKey string `json:"publicKey"`
}

// PushSubscriptionResponse is returned after a successful subscribe.
type PushSubscriptionResponse struct {
	Endpoint string `json:"endpoint"`
	Enabled  bool   `json:"enabled"`
}

// PushPayload is the JSON sent to the service worker's push handler. URL is a
// hash route of nexo-web (e.g. "/#/debts").
type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag,omitempty"`
}
