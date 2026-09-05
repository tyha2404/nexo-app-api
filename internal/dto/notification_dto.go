package dto

type SubscribePushRequest struct {
	Endpoint   string `json:"endpoint" validate:"required"`
	Keys       PushKeys `json:"keys" validate:"required"`
	UserAgent  string `json:"userAgent,omitempty"`
	DeviceType string `json:"deviceType,omitempty"`
}

type PushKeys struct {
	P256dh string `json:"p256dh" validate:"required"`
	Auth   string `json:"auth" validate:"required"`
}

type UnsubscribePushRequest struct {
	Endpoint string `json:"endpoint" validate:"required"`
}

type TestPushRequest struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	URL   string `json:"url,omitempty"`
}

type VapidPublicKeyResponse struct {
	PublicKey string `json:"publicKey"`
}

type PushNotificationPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
	Tag   string `json:"tag,omitempty"`
}
