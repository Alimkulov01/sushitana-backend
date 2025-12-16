package structs

import "encoding/json"

type IikoWebhookEvent struct {
	EventType      string               `json:"eventType"`
	EventTime      string               `json:"eventTime"`
	OrganizationId string               `json:"organizationId"`
	CorrelationId  string               `json:"correlationId"`
	EventInfo      IikoWebhookEventInfo `json:"eventInfo"`
}

type IikoWebhookEventInfo struct {
	ID             string                `json:"id"`
	PosID          string                `json:"posId"`
	ExternalNumber string                `json:"externalNumber"`
	OrganizationId string                `json:"organizationId"`
	Timestamp      int64                 `json:"timestamp"`
	CreationStatus string                `json:"creationStatus"`
	ErrorInfo      *IikoWebhookErrorInfo `json:"errorInfo,omitempty"`
	Order          json.RawMessage       `json:"order"` // null bo‘lishi mumkin
}

type IikoWebhookErrorInfo struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Description    string `json:"description"`
	AdditionalData any    `json:"additionalData"`
}
