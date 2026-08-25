package scenarios

import (
	"fmt"
	"strings"

	"github.com/aegion-dynamic/graphjin-slim/bench/v3/harness"
)

func init() {
	register(Scenario{Name: "rest_saved_queries", Fn: restSavedQueries,
		Seeds: map[string]harness.SeedQuery{
			"getUser": {Query: `query getUser($id = ID) { users(id: $id) { id full_name email } }`, Vars: map[string]any{"id": 1}},
		}})
	register(Scenario{Name: "strict_vars", Fn: strictVars})
	register(Scenario{Name: "apq", Fn: apq})
	register(Scenario{Name: "openapi_spec", Fn: openapiSpec,
		Seeds: map[string]harness.SeedQuery{
			"getUser": {Query: `query getUser($id = ID) { users(id: $id) { id full_name email } }`, Vars: map[string]any{"id": 1}},
		}})
}

const getUserQuery = `query getUser($id = ID) {
  users(id: $id) { id full_name email }
}`

// restSavedQueries covers the full saved-query lifecycle: save → files on
// disk → list/load → REST by-name (GET params, POST body) → loud
// missing-variable error → delete.
func restSavedQueries(h *harness.H) error {
	// Seeded pre-start (see runner); artifacts must exist on disk.
	gql, errG := fileExists(h.Root, "config/queries/getUser.gql")
	jsn, errJ := fileExists(h.Root, "config/queries/getUser.json")
	if errG != nil || errJ != nil || !gql || !jsn {
		return fmt.Errorf("artifact pair missing: gql=%v(%v) json=%v(%v)", gql, errG, jsn, errJ)
	}

	_, list, err := h.Console("GET", "/api/v1/queries", nil)
	if err != nil {
		return err
	}
	if !listContains(list, "getUser") {
		return fmt.Errorf("list missing getUser: %v", list)
	}

	_, loaded, err := h.Console("GET", "/api/v1/queries?name=getUser", nil)
	if err != nil {
		return err
	}
	if q, _ := loaded["query"].(string); !strings.Contains(q, "users") {
		return fmt.Errorf("loaded query = %q", q)
	}
	if v, _ := loaded["variables"].(map[string]any); v["id"] != float64(1) {
		return fmt.Errorf("loaded variables = %v, want id=1", v)
	}

	// GET with individual typed params.
	status, body, _, err := h.Rest("GET", "getUser", urlVals("id", "2"), nil)
	if err != nil {
		return err
	}
	if name, _ := harness.Walk(body, "data.users.full_name"); name != "Alan Turing" {
		return fmt.Errorf("GET ?id=2 → %v", body)
	}
	_ = status

	// POST with a flat JSON body.
	_, body, _, err = h.Rest("POST", "getUser", nil, map[string]any{"id": 3})
	if err != nil {
		return err
	}
	if name, _ := harness.Walk(body, "data.users.full_name"); name != "Grace Hopper" {
		return fmt.Errorf("POST {id:3} → %v", body)
	}

	// Bare call must fail loudly naming the variable.
	_, body, _, err = h.Rest("GET", "getUser", nil, nil)
	if err != nil {
		return err
	}
	msg := firstError(body)
	if !strings.Contains(msg, "required variable") {
		return fmt.Errorf("bare call error = %q, want required-variable message", msg)
	}

	// Delete removes both artifacts; the endpoint disappears.
	if _, resp, err := h.Console("DELETE", "/api/v1/queries?name=getUser", nil); err != nil || resp["ok"] != true {
		return fmt.Errorf("delete: %v %v", err, resp)
	}
	gone, _ := fileExists(h.Root, "config/queries/getUser.gql")
	if gone {
		return fmt.Errorf("getUser.gql still exists after delete")
	}
	status, body, raw, err := h.Rest("GET", "getUser", urlVals("id", "1"), nil)
	if err != nil {
		return err
	}
	executed := status == 200 && len(body) > 0 && firstError(body) == "" && body["data"] != nil
	if executed {
		return fmt.Errorf("deleted query still executed: %s", raw)
	}
	return nil
}

// strictVars proves per-type coercion on individual GET parameters.
func strictVars(h *harness.H) error {
	code, resp, err := h.Console("POST", "/api/v1/queries", map[string]any{
		"name":  "findUsers",
		"query": `query findUsers($limit = Int, $full_name = String) { users(limit: $limit, where: { full_name: { eq: $full_name } }) { id full_name } }`,
	})
	if err != nil || code != 200 {
		return fmt.Errorf("save: code=%d resp=%v err=%v", code, resp, err)
	}

	// Typed values: Int limit + String name.
	_, body, _, err := h.Rest("GET", "findUsers", urlVals("limit", "1", "full_name", "Ada Lovelace"), nil)
	if err != nil {
		return err
	}
	rows, _ := harness.Walk(body, "data.users")
	list, _ := rows.([]any)
	if len(list) != 1 {
		return fmt.Errorf("typed params → %v, want exactly Ada", body)
	}
	if name, _ := harness.Walk(list[0].(map[string]any), "full_name"); name != "Ada Lovelace" {
		return fmt.Errorf("row = %v", list[0])
	}

	// A non-integer for an Int variable must not silently pass.
	_, body, _, err = h.Rest("GET", "findUsers", urlVals("limit", "abc", "full_name", "x"), nil)
	if err != nil {
		return err
	}
	if firstError(body) == "" && body["data"] != nil {
		return fmt.Errorf("limit=abc should error, got %v", body)
	}
	return nil
}

// apq exercises automatic persisted queries: first request stores the
// query under its hash, second sends only the hash.
func apq(h *harness.H) error {
	q := `{ users(limit: 1) { id } }`
	hash := sha256Hex(q)

	ext := map[string]any{
		"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash},
	}
	resp, err := h.GQLFull(q, nil, ext)
	if err != nil {
		return err
	}
	if errs := resp["errors"]; errs != nil {
		return fmt.Errorf("apq store: %v", errs)
	}
	want, _ := harness.Walk(resp, "data")

	resp2, err := h.GQLFull("", nil, ext)
	if err != nil {
		return err
	}
	if errs := resp2["errors"]; errs != nil {
		return fmt.Errorf("apq replay: %v", errs)
	}
	if fmt.Sprint(resp2["data"]) != fmt.Sprint(want) {
		return fmt.Errorf("apq replay mismatch: %v vs %v", resp2["data"], want)
	}
	return nil
}

// openapiSpec checks the served spec against live reality and verifies the
// startup export to disk matches byte-for-byte.
func openapiSpec(h *harness.H) error {
	if _, _, err := h.Console("POST", "/api/v1/queries", map[string]any{
		"name":      "getUser",
		"query":     getUserQuery,
		"variables": map[string]any{"id": 1},
	}); err != nil {
		return err
	}

	status, raw, err := h.Get("/api/v1/openapi.json")
	if err != nil || status != 200 {
		return fmt.Errorf("spec fetch: status=%d err=%v", status, err)
	}
	spec := parseJSON(raw)

	if spec["openapi"] != "3.0.0" {
		return fmt.Errorf("openapi version = %v", spec["openapi"])
	}
	paths := spec["paths"].(map[string]any)
	if _, ok := paths["/getUser"]; !ok {
		return fmt.Errorf("spec missing /getUser (have %v)", keys(paths))
	}
	getOp := paths["/getUser"].(map[string]any)["get"].(map[string]any)
	for _, pv := range getOp["parameters"].([]any) {
		p := pv.(map[string]any)
		if p["name"] == "variables" {
			return fmt.Errorf("untyped blob param leaked into spec")
		}
		if p["name"] == "id" {
			sm := p["schema"].(map[string]any)
			if sm["type"] == "" || sm["type"] == nil {
				return fmt.Errorf("id param untyped: %v", p)
			}
		}
	}
	// Startup export must match what the endpoint serves.
	disk, err := fileBytes(h.Root, "specs/openapi.json")
	if err != nil {
		return fmt.Errorf("specs dir export: %w", err)
	}
	if string(disk) != string(raw) {
		return fmt.Errorf("on-disk spec differs from served spec")
	}
	return nil
}
