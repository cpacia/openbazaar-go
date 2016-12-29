package notifications

import (
	"encoding/json"
	"time"
)

type notificationWrapper struct {
	Notfication interface{} `json:"notification"`
}

type messageWrapper struct {
	Message interface{} `json:"message"`
}

type orderWrapper struct {
	OrderNotification `json:"order"`
}

type paymentWrapper struct {
	PaymentNotification `json:"payment"`
}

type orderConfirmationWrapper struct {
	OrderConfirmationNotification `json:"orderConfirmation"`
}

type orderCancelWrapper struct {
	OrderCancelNotification `json:"orderConfirmation"`
}

type refundWrapper struct {
	RefundNotification `json:"refund"`
}

type fulfillmentWrapper struct {
	FulfillmentNotification `json:"orderFulfillment"`
}

type completionWrapper struct {
	CompletionNotification `json:"orderCompletion"`
}

type disputeOpenWrapper struct {
	DisputeOpenNotification `json:"disputeOpen"`
}

type disputeUpdateWrapper struct {
	DisputeUpdateNotification `json:"disputeUpdate"`
}

type disputeCloseWrapper struct {
	DisputeCloseNotification `json:"disputeClose"`
}

type OrderNotification struct {
	Title             string `json:"title"`
	BuyerGuid         string `json:"buyerGuid"`
	BuyerBlockchainId string `json:"buyerBlockchainId"`
	Thumbnail         string `json:"thumbnail"`
	Timestamp         int    `json:"timestamp"`
	OrderId           string `json:"orderId"`
}

type PaymentNotification struct {
	OrderId      string `json:"orderId"`
	FundingTotal uint64 `json:"fundingTotal"`
}

type OrderConfirmationNotification struct {
	OrderId string `json:"orderId"`
}

type OrderCancelNotification struct {
	OrderId string `json:"orderId"`
}

type RefundNotification struct {
	OrderId string `json:"orderId"`
}

type FulfillmentNotification struct {
	OrderId string `json:"orderId"`
}

type CompletionNotification struct {
	OrderId string `json:"orderId"`
}

type DisputeOpenNotification struct {
	OrderId string `json:"orderId"`
}

type DisputeUpdateNotification struct {
	OrderId string `json:"orderId"`
}

type DisputeCloseNotification struct {
	OrderId string `json:"orderId"`
}

type FollowNotification struct {
	Follow string `json:"follow"`
}

type UnfollowNotification struct {
	Unfollow string `json:"unfollow"`
}

type ChatMessage struct {
	PeerId    string    `json:"peerId"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func Serialize(i interface{}) []byte {
	var n notificationWrapper
	switch i.(type) {
	case OrderNotification:
		n = notificationWrapper{
			orderWrapper{
				OrderNotification: i.(OrderNotification),
			},
		}
	case PaymentNotification:
		n = notificationWrapper{
			paymentWrapper{
				PaymentNotification: i.(PaymentNotification),
			},
		}
	case OrderConfirmationNotification:
		n = notificationWrapper{
			orderConfirmationWrapper{
				OrderConfirmationNotification: i.(OrderConfirmationNotification),
			},
		}
	case OrderCancelNotification:
		n = notificationWrapper{
			orderCancelWrapper{
				OrderCancelNotification: i.(OrderCancelNotification),
			},
		}
	case RefundNotification:
		n = notificationWrapper{
			refundWrapper{
				RefundNotification: i.(RefundNotification),
			},
		}
	case FulfillmentNotification:
		n = notificationWrapper{
			fulfillmentWrapper{
				FulfillmentNotification: i.(FulfillmentNotification),
			},
		}
	case CompletionNotification:
		n = notificationWrapper{
			completionWrapper{
				CompletionNotification: i.(CompletionNotification),
			},
		}
	case DisputeOpenNotification:
		n = notificationWrapper{
			disputeOpenWrapper{
				DisputeOpenNotification: i.(DisputeOpenNotification),
			},
		}
	case DisputeUpdateNotification:
		n = notificationWrapper{
			disputeUpdateWrapper{
				DisputeUpdateNotification: i.(DisputeUpdateNotification),
			},
		}
	case DisputeCloseNotification:
		n = notificationWrapper{
			disputeCloseWrapper{
				DisputeCloseNotification: i.(DisputeCloseNotification),
			},
		}
	case FollowNotification:
		n = notificationWrapper{
			i.(FollowNotification),
		}
	case UnfollowNotification:
		n = notificationWrapper{
			i.(UnfollowNotification),
		}
	case ChatMessage:
		m := messageWrapper{
			i.(ChatMessage),
		}
		b, _ := json.MarshalIndent(m, "", "    ")
		return b
	}
	b, _ := json.MarshalIndent(n, "", "    ")
	return b
}
