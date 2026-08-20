package core

import (
	"context"
	"encoding/json"
	"errors"
)

// Subscriptions are not supported in the slim build.

var errSubscriptionsDisabled = errors.New("subscriptions are not supported")

// Member is a stub kept so older call sites compile if referenced.
type Member struct{}

func (m *Member) Close() {}

// Subscribe rejects subscription operations.
func (g *GraphJin) Subscribe(
	c context.Context,
	query string,
	vars json.RawMessage,
	rc *RequestConfig,
) (*Member, error) {
	return nil, errSubscriptionsDisabled
}

func (g *GraphJin) SubscribeByName(
	c context.Context,
	name string,
	vars json.RawMessage,
	rc *RequestConfig,
) (*Member, error) {
	return nil, errSubscriptionsDisabled
}
