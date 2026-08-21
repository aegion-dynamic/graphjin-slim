package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/core/v3/allow"
	"github.com/aegion-dynamic/graphjin-slim/core/v3/graph"
)

// SaveQuery persists a named query and its default variables to the engine's
// query store without executing it. The variables become the defaults used
// when the query is later executed by name (REST or GraphQLByName).
func (g *GraphJin) SaveQuery(details *SavedQueryDetails) (err error) {
	if details == nil {
		return errors.New("saved query details are required")
	}
	if strings.TrimSpace(details.Name) == "" {
		return errors.New("saved query name is required")
	}
	if strings.TrimSpace(details.Query) == "" {
		return errors.New("saved query query is required")
	}
	gj, err := g.getEngine()
	if err != nil {
		return err
	}
	if gj.allowList == nil {
		return errors.New("query store unavailable")
	}

	operation := strings.TrimSpace(details.Operation)
	if operation == "" {
		h, parseErr := graph.FastParseBytes([]byte(details.Query))
		if parseErr != nil {
			return parseErr
		}
		operation = h.Operation
	}

	item := allow.Item{
		Namespace:  details.Namespace,
		Name:       details.Name,
		Operation:  strings.ToLower(operation),
		Query:      []byte(details.Query),
		ActionJSON: savedQueryVariablesToRaw(details.Variables),
	}

	if gj.savedQuerySaveHook != nil {
		req := SavedQuerySaveRequest{
			Namespace:  item.Namespace,
			Name:       item.Name,
			Operation:  item.Operation,
			Query:      append([]byte(nil), item.Query...),
			ActionJSON: item.ActionJSON,
		}
		handled, herr := gj.savedQuerySaveHook(context.Background(), req)
		if herr != nil {
			return herr
		}
		if handled {
			return nil
		}
	}

	return gj.allowList.Set(item)
}

// DeleteSavedQuery removes a saved query and its variables file.
func (g *GraphJin) DeleteSavedQuery(name string) error {
	gj, err := g.getEngine()
	if err != nil {
		return err
	}
	if gj.allowList == nil {
		return errors.New("query store unavailable")
	}
	return gj.allowList.Delete(name)
}

// SavedQuerySummary describes one saved query without its body.
type SavedQuerySummary struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Operation string `json:"operation,omitempty"`
}

// SavedQuery returns one saved query including its source and default
// variables.
func (g *GraphJin) SavedQuery(name string) (*SavedQueryDetails, error) {
	gj, err := g.getEngine()
	if err != nil {
		return nil, err
	}
	if gj.allowList == nil {
		return nil, allow.ErrUnknownGraphQLQuery
	}
	item, err := gj.allowList.GetByName(name, true)
	if err != nil {
		return nil, err
	}
	vars := make(map[string]any, len(item.ActionJSON))
	for k, raw := range item.ActionJSON {
		var v any
		if json.Unmarshal(raw, &v) == nil {
			vars[k] = v
		} else {
			vars[k] = json.RawMessage(raw)
		}
	}
	return &SavedQueryDetails{
		Name:      item.Name,
		Namespace: item.Namespace,
		Operation: item.Operation,
		Query:     string(item.Query),
		Variables: vars,
	}, nil
}

// SavedQueries lists the queries in the engine's store.
func (g *GraphJin) SavedQueries() ([]SavedQuerySummary, error) {
	gj, err := g.getEngine()
	if err != nil {
		return nil, err
	}
	if gj.allowList == nil {
		return nil, nil
	}
	items, err := gj.allowList.ListAll()
	if err != nil {
		return nil, err
	}
	out := make([]SavedQuerySummary, 0, len(items))
	for _, it := range items {
		out = append(out, SavedQuerySummary{
			Namespace: it.Namespace,
			Name:      it.Name,
			Operation: it.Operation,
		})
	}
	return out, nil
}
