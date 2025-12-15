package structs

import "encoding/json"

type IikoWebhookDeliveryOrderUpdate struct {
	EventType      string                      `json:"eventType"`     
	EventTime      string                      `json:"eventTime"`      
	OrganizationID string                      `json:"organizationId"` 
	CorrelationID  string                      `json:"correlationId"`  
	EventInfo      IikoWebhookDeliveryEventInfo `json:"eventInfo"`
}

type IikoWebhookDeliveryEventInfo struct {
	ID             string          `json:"id"`             // eng MUHIM: siz iiko create’da shu ID’ni internal order_id qilib yuborasiz
	PosID          string          `json:"posId"`          // iiko'dagi order/delivery POS id
	ExternalNumber string          `json:"externalNumber"` // tashqi raqam bo‘lishi mumkin
	OrganizationID string          `json:"organizationId"` // qayta keladi
	Timestamp      int64           `json:"timestamp"`      // unix seconds bo‘lishi mumkin
	CreationStatus string          `json:"creationStatus"` // "Success" yoki error holat
	ErrorInfo      json.RawMessage `json:"errorInfo"`      // ba'zan null bo‘ladi
	Order          json.RawMessage `json:"order"`          // juda katta obyekt, statusni shu yerdan olasiz
}
