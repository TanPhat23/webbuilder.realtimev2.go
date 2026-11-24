package logic

import (
	"strings"
	"testing"
)

func TestMergeWithLiteralNullDefaults(t *testing.T) {
	base := Empty()
	patch := map[string]any{"variables": map[string]any{"object": map[string]any{"type": "object", "default": map[string]any{"nested": map[string]any{}}}, "empty": map[string]any{"type": "object"}}}
	merged, err := MergeWithNulls(base, patch, [][]string{{"variables", "object", "default", "nested", "value"}, {"variables", "empty", "default"}})
	if err != nil {
		t.Fatal(err)
	}
	variables := merged["variables"].(map[string]any)
	if value, ok := variables["empty"].(map[string]any)["default"]; !ok || value != nil {
		t.Fatal("missing null default")
	}
	nested := variables["object"].(map[string]any)["default"].(map[string]any)["nested"].(map[string]any)
	if value, ok := nested["value"]; !ok || value != nil {
		t.Fatal("missing nested null")
	}
	if _, err := MergeWithNulls(base, patch, [][]string{{"flows", "f"}}); err == nil {
		t.Fatal("accepted a null outside a variable default")
	}
	if _, err := MergeWithNulls(base, patch, [][]string{{"variables", "missing", "default"}}); err == nil {
		t.Fatal("accepted a missing parent")
	}
}

func TestMergeAndValidateScopes(t *testing.T) {
	project := Document{"v": float64(1), "variables": map[string]any{"count": map[string]any{"type": "number", "default": float64(0)}}, "flows": map[string]any{}}
	page := Document{"v": float64(1), "variables": map[string]any{}, "flows": map[string]any{}}
	merged, err := Merge(page, map[string]any{"flows": map[string]any{"greet": map[string]any{"do": []any{map[string]any{"toast": "Hi"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateScopes(project, map[string]Document{"page": merged}); err != nil {
		t.Fatal(err)
	}
	if len(page["flows"].(map[string]any)) != 0 {
		t.Fatal("mutated original")
	}
	duplicate, err := Merge(merged, map[string]any{"variables": map[string]any{"count": map[string]any{"type": "number", "default": float64(1)}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateScopes(project, map[string]Document{"page": duplicate}); err == nil {
		t.Fatal("duplicate name accepted")
	}
	if _, err := Merge(page, map[string]any{"flows": map[string]any{"bad": map[string]any{"do": []any{map[string]any{"toast": 1}}}}}); err == nil {
		t.Fatal("invalid step accepted")
	}
	removed, err := Merge(merged, map[string]any{"flows": map[string]any{"greet": nil}})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed["flows"].(map[string]any)) != 0 {
		t.Fatal("null did not remove flow")
	}
}

func TestRejectMalformedStepFields(t *testing.T) {
	for _, step := range []map[string]any{
		{"call": "api", "method": "FETCH", "url": "/"},
		{"call": "api", "method": "POST", "url": "/", "headers": map[string]any{"X": 42}},
		{"class": "add", "target": "x", "name": 42},
		{"animate": "x", "preset": "unknown"},
		{"toast": "Hi", "duration": "wrong"},
		{"set": "bad-name", "to": "1"},
		{"run": "bad-name"},
	} {
		doc := Empty()
		doc["flows"] = map[string]any{"f": map[string]any{"do": []any{step}}}
		if err := Validate(doc); err == nil {
			t.Errorf("accepted invalid step %#v", step)
		}
	}
}

func TestRejectInaccessibleVariableNames(t *testing.T) {
	doc := Empty()
	doc["variables"] = map[string]any{"constructor": map[string]any{"type": "string", "default": ""}}
	if err := Validate(doc); err == nil {
		t.Fatal("accepted inaccessible variable name")
	}
}

func TestStepFieldRules(t *testing.T) {
	steps := func(ss ...any) any { return ss }
	for _, tc := range []struct {
		name string
		step map[string]any
		want string // "" accepts
	}{
		{"incomplete fields are accepted", map[string]any{"toast": "", "duration": "", "kind": ""}, ""},
		{"toast duration must be positive", map[string]any{"toast": "Hi", "duration": float64(0)}, "step 0: invalid option type or value"},
		{"show duration may be zero", map[string]any{"show": "#a", "duration": float64(0)}, ""},
		{"animate duration must be positive", map[string]any{"animate": "#a", "duration": float64(0)}, "step 0: invalid option type or value"},
		{"scroll offset may be negative", map[string]any{"scroll": "#a", "offset": float64(-5)}, ""},
		{"call timeout must be positive", map[string]any{"call": "api", "timeout": float64(0)}, "step 0: invalid option type or value"},
		{"call timeout is capped", map[string]any{"call": "api", "timeout": float64(60001)}, "step 0: invalid option type or value"},
		{"navigate flags are booleans", map[string]any{"navigate": "/x", "newTab": "yes"}, "step 0: invalid option type or value"},
		{"wait is capped", map[string]any{"wait": float64(60001)}, "invalid wait"},
		{"wait is a number", map[string]any{"wait": "5"}, "invalid wait"},
		{"scroll target may be a number", map[string]any{"scroll": float64(10)}, ""},
		{"scroll target is not a bool", map[string]any{"scroll": true}, "invalid scroll"},
		{"other verbs take strings", map[string]any{"show": float64(1)}, "invalid show"},
		{"no verb", map[string]any{"id": "a"}, "step 0 has no verb"},
		{"two verbs", map[string]any{"show": "#a", "hide": "#b"}, "step 0 has multiple verbs"},
		{"unknown field", map[string]any{"show": "#a", "zzz": "1"}, "step 0 has unknown fields"},
		{"field of another verb", map[string]any{"show": "#a", "kind": "info"}, "step 0 has unknown fields"},
		{"bad step id", map[string]any{"show": "#a", "id": "bad-id"}, "step 0 has invalid id"},
		{"modal action", map[string]any{"modal": "toggle"}, "step 0: invalid modal"},
		{"toast kind", map[string]any{"toast": "Hi", "kind": "loud"}, "step 0: invalid toast kind"},
		{"download filename", map[string]any{"download": "#a", "filename": float64(1)}, "step 0: invalid filename"},
		{"class action", map[string]any{"class": "flip"}, "step 0: invalid class"},
		{"class name template is deferred", map[string]any{"class": "add", "name": "a b {{ x"}, ""},
		{"class name with spaces", map[string]any{"class": "add", "name": "a b"}, "step 0: invalid class name"},
		{"set to must be a string", map[string]any{"set": "x", "to": float64(1)}, "step 0: invalid set"},
		{"call method", map[string]any{"call": "api", "method": "get"}, "step 0: invalid call"},
		{"call accepts PUT", map[string]any{"call": "api", "method": "PUT"}, ""},
		{"call accepts DELETE", map[string]any{"call": "api", "method": "DELETE"}, ""},
		{"call accepts PATCH", map[string]any{"call": "api", "method": "PATCH"}, ""},
		{"call accepts POST", map[string]any{"call": "api", "method": "POST", "url": "/x", "body": "{}"}, ""},
		{"valid step id", map[string]any{"show": "#a", "id": "step_1"}, ""}, {"call kind", map[string]any{"call": "rpc"}, "step 0: invalid call"},
		{"call headers shape", map[string]any{"call": "api", "headers": "x"}, "step 0: invalid headers"},
		{"call header value", map[string]any{"call": "api", "headers": map[string]any{"A": nil}}, "step 0: invalid header value"},
		{"if branch must be a step list", map[string]any{"if": "x", "then": ""}, "invalid or deeply nested steps"},
		{"if branch is validated", map[string]any{"if": "x", "else": steps(map[string]any{"show": float64(1)})}, "invalid show"},
		{"if missing branches are incomplete", map[string]any{"if": "x"}, ""},
	} {
		doc := Empty()
		doc["flows"] = map[string]any{"f": map[string]any{"do": steps(tc.step)}}
		err := Validate(doc)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if tc.want == "" && got != "" || tc.want != "" && !strings.HasSuffix(got, tc.want) {
			t.Errorf("%s: got %q, want suffix %q", tc.name, got, tc.want)
		}
	}
}

func TestStepNestingIsBounded(t *testing.T) {
	nested := func(depth int) any {
		steps := []any{}
		for i := 0; i < depth; i++ {
			steps = []any{map[string]any{"if": "x", "then": steps}}
		}
		return steps
	}
	for depth, wantErr := range map[int]bool{32: false, 34: true} {
		doc := Empty()
		doc["flows"] = map[string]any{"f": map[string]any{"do": nested(depth)}}
		if err := Validate(doc); (err != nil) != wantErr {
			t.Errorf("depth %d: err=%v, want error %v", depth, err, wantErr)
		}
	}
}
