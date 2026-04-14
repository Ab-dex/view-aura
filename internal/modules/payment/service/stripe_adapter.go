package service

import (
	"context"
	"fmt"

	stripego "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"

	stripepkg "github.com/Ab-dex/view-aura/internal/platform/stripe"
)

// StripeAdapter is the interface for all Stripe API operations.
// Using an interface decouples the service from the stripe-go library,
// making tests runnable without real Stripe credentials.
type StripeAdapter interface {
	CreateCustomer(ctx context.Context, email, name string) (customerID string, err error)
	CreateSubscription(ctx context.Context, customerID, priceID, paymentMethod string) (*stripego.Subscription, error)
	UpdateSubscription(ctx context.Context, stripeSubID, newPriceID string) (*stripego.Subscription, error)
	CancelSubscription(ctx context.Context, stripeSubID string) error
	ConstructWebhookEvent(payload []byte, sigHeader, secret string) (*stripego.Event, error)
}

// stripeAdapter wraps the platform *stripe.Client.
type stripeAdapter struct{ client *stripepkg.Client }

func NewStripeAdapter(client *stripepkg.Client) StripeAdapter {
	return &stripeAdapter{client: client}
}

func (a *stripeAdapter) CreateCustomer(_ context.Context, email, name string) (string, error) {
	c, err := a.client.API.Customers.New(&stripego.CustomerParams{
		Email: stripego.String(email),
		Name:  stripego.String(name),
	})
	if err != nil {
		return "", fmt.Errorf("stripe: create customer: %w", err)
	}
	return c.ID, nil
}

func (a *stripeAdapter) CreateSubscription(_ context.Context, customerID, priceID, paymentMethod string) (*stripego.Subscription, error) {
	return a.client.API.Subscriptions.New(&stripego.SubscriptionParams{
		Customer: stripego.String(customerID),
		Items: []*stripego.SubscriptionItemsParams{
			{Price: stripego.String(priceID)},
		},
		DefaultPaymentMethod: stripego.String(paymentMethod),
	})
}

func (a *stripeAdapter) UpdateSubscription(_ context.Context, stripeSubID, newPriceID string) (*stripego.Subscription, error) {
	sub, err := a.client.API.Subscriptions.Get(stripeSubID, nil)
	if err != nil {
		return nil, fmt.Errorf("stripe: get subscription: %w", err)
	}
	if len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("stripe: subscription %s has no items", stripeSubID)
	}
	return a.client.API.Subscriptions.Update(stripeSubID, &stripego.SubscriptionParams{
		Items: []*stripego.SubscriptionItemsParams{
			{
				ID:    stripego.String(sub.Items.Data[0].ID),
				Price: stripego.String(newPriceID),
			},
		},
		ProrationBehavior: stripego.String("create_prorations"),
	})
}

func (a *stripeAdapter) CancelSubscription(_ context.Context, stripeSubID string) error {
	_, err := a.client.API.Subscriptions.Cancel(stripeSubID, nil)
	return err
}

func (a *stripeAdapter) ConstructWebhookEvent(payload []byte, sigHeader, secret string) (*stripego.Event, error) {
	event, err := webhook.ConstructEvent(payload, sigHeader, secret)
	if err != nil {
		return nil, err
	}
	return &event, nil
}
