// Package logic validates and merges page and project interaction documents.
package logic

import (
	"fmt"
	"math"
	"regexp"

	elementutils "webbuilder/internal/utils/element"
)

type Document map[string]any

// Revisioned is a stored document with its revision: 0 until first written,
// then incremented by every committed update of that document. Clients use it
// to drop a document older than one they already have.
type Revisioned struct {
	Doc      Document
	Revision int64
}

var nameRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,63}$`)

func Empty() Document {
	return Document{"v": float64(1), "variables": map[string]any{}, "flows": map[string]any{}}
}

// Merge applies the same recursive null-deleting patch semantics as element settings.
func Merge(base Document, patch map[string]any) (Document, error) {
	return MergeWithNulls(base, patch, nil)
}

// MergeWithNulls preserves merge-patch deletion semantics while allowing
// explicit literal null values inside variable defaults.
func MergeWithNulls(base Document, patch map[string]any, nullPaths [][]string) (Document, error) {
	if base == nil {
		base = Empty()
	}
	merged := Document(elementutils.MergeMap(map[string]any(base), patch))
	if len(nullPaths) > 1024 {
		return nil, fmt.Errorf("too many literal null paths")
	}
	for _, path := range nullPaths {
		if err := setLiteralNull(merged, path); err != nil {
			return nil, err
		}
	}
	if err := Validate(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

// setLiteralNull sets the variable default at path to nil, copying each object
// on the way down so the caller's patch is never mutated.
func setLiteralNull(doc Document, path []string) error {
	if len(path) < 3 || len(path) > 64 || path[0] != "variables" || path[2] != "default" {
		return fmt.Errorf("invalid literal null path")
	}
	current := map[string]any(doc)
	for _, key := range path[:len(path)-1] {
		child, ok := current[key].(map[string]any)
		if !ok {
			return fmt.Errorf("literal null path has no object parent")
		}
		copied := make(map[string]any, len(child))
		for k, v := range child {
			copied[k] = v
		}
		current[key] = copied
		current = copied
	}
	current[path[len(path)-1]] = nil
	return nil
}

func obj(v any) (map[string]any, bool) { m, ok := v.(map[string]any); return m, ok && m != nil }
func only(m map[string]any, fields ...string) bool {
	allowed := make(map[string]bool, len(fields))
	for _, f := range fields {
		allowed[f] = true
	}
	for k := range m {
		if !allowed[k] {
			return false
		}
	}
	return true
}
func validDefault(kind string, value any) bool {
	switch kind {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		n, ok := value.(float64)
		return ok && !math.IsNaN(n) && !math.IsInf(n, 0)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		if value == nil {
			return true
		}
		_, ok := obj(value)
		return ok
	case "list":
		_, ok := value.([]any)
		return ok
	}
	return false
}

// Variable names a client would resolve through the object prototype chain.
var inaccessibleNames = map[string]bool{"__proto__": true, "constructor": true, "prototype": true}

func Validate(doc Document) error {
	variables, flows, err := documentSections(doc)
	if err != nil {
		return err
	}
	for name, raw := range variables {
		if err := validateVariable(name, raw); err != nil {
			return err
		}
	}
	for name, raw := range flows {
		if err := validateFlow(name, raw); err != nil {
			return err
		}
	}
	return nil
}

// documentSections checks the document envelope and returns its two sections.
func documentSections(doc Document) (variables, flows map[string]any, err error) {
	if doc == nil || !only(doc, "v", "variables", "flows") || doc["v"] != float64(1) {
		return nil, nil, fmt.Errorf("invalid logic document version or fields")
	}
	variables, ok := obj(doc["variables"])
	if !ok {
		return nil, nil, fmt.Errorf("variables must be an object")
	}
	flows, ok = obj(doc["flows"])
	if !ok {
		return nil, nil, fmt.Errorf("flows must be an object")
	}
	return variables, flows, nil
}

func validateVariable(name string, raw any) error {
	if !nameRE.MatchString(name) || inaccessibleNames[name] {
		return fmt.Errorf("invalid variable name %q", name)
	}
	def, ok := obj(raw)
	if !ok || !only(def, "type", "default", "lifetime") {
		return fmt.Errorf("invalid variable %q", name)
	}
	return validateVariableDef(name, def)
}

func validateVariableDef(name string, def map[string]any) error {
	kind, ok := def["type"].(string)
	if !ok || !validDefault(kind, def["default"]) {
		return fmt.Errorf("invalid default for variable %q", name)
	}
	if _, exists := def["default"]; !exists {
		return fmt.Errorf("missing default for variable %q", name)
	}
	if !optEnum(def, "lifetime", "page", "session", "local") {
		return fmt.Errorf("invalid lifetime for variable %q", name)
	}
	return nil
}

func validateFlow(name string, raw any) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid flow name %q", name)
	}
	flow, ok := obj(raw)
	if !ok || !only(flow, "do") {
		return fmt.Errorf("invalid flow %q", name)
	}
	if err := validateSteps(flow["do"], 0); err != nil {
		return fmt.Errorf("flow %s: %w", name, err)
	}
	return nil
}

// ValidateScopes enforces one namespace per definition kind across the project.
func ValidateScopes(project Document, pages map[string]Document) error {
	docs := []Document{project}
	for _, page := range pages {
		docs = append(docs, page)
	}
	seen := map[string]map[string]bool{"variables": {}, "flows": {}}
	for _, doc := range docs {
		if doc == nil {
			doc = Empty()
		}
		if err := Validate(doc); err != nil {
			return err
		}
		for kind, names := range seen {
			if err := claimNames(kind, doc, names); err != nil {
				return err
			}
		}
	}
	return nil
}

// claimNames records every name of the given kind in doc, rejecting repeats.
func claimNames(kind string, doc Document, seen map[string]bool) error {
	entries, _ := obj(doc[kind])
	for name := range entries {
		if seen[name] {
			return fmt.Errorf("duplicate %s name %q", kind, name)
		}
		seen[name] = true
	}
	return nil
}

var fields = map[string][]string{
	"navigate": {"newTab", "replace"}, "show": {"duration"}, "hide": {"duration"}, "toggle": {"duration"},
	"scroll": {"smooth", "offset"}, "modal": {"target"}, "toast": {"kind", "duration"}, "copy": {},
	"download": {"filename"}, "animate": {"preset", "duration"}, "class": {"target", "name"}, "set": {"to"},
	"call": {"method", "url", "headers", "body", "timeout"}, "submit": {}, "reset": {}, "wait": {},
	"if": {"then", "else"}, "run": {}, "code": {},
}

// A step may be stored before it is filled in, as the editor does: a field
// that is missing or "" is incomplete, not invalid (the client reports it as
// "incomplete"). Only a present value of the wrong type or shape is rejected,
// as are unknown fields, a missing or duplicated verb, and bad nesting.
// Templates are not parsed here; an unparseable one is incomplete on the
// client and fails only its own step when run.
func validateSteps(raw any, depth int) error {
	steps, ok := raw.([]any)
	if !ok || depth > 32 {
		return fmt.Errorf("invalid or deeply nested steps")
	}
	for i, rawStep := range steps {
		if err := validateStep(i, rawStep, depth); err != nil {
			return err
		}
	}
	return nil
}

func validateStep(i int, rawStep any, depth int) error {
	step, ok := obj(rawStep)
	if !ok {
		return fmt.Errorf("step %d must be an object", i)
	}
	verb, err := stepVerb(i, step)
	if err != nil {
		return err
	}
	if !only(step, append([]string{"id", verb}, fields[verb]...)...) {
		return fmt.Errorf("step %d has unknown fields", i)
	}
	filled := withoutEmpty(step)
	if id, exists := filled["id"]; exists && !validName(id) {
		return fmt.Errorf("step %d has invalid id", i)
	}
	if value, exists := filled[verb]; exists {
		if err := validateVerbValue(verb, value); err != nil {
			return err
		}
	}
	if err := validateStepFields(verb, filled); err != nil {
		return fmt.Errorf("step %d: %w", i, err)
	}
	if verb == "if" {
		return validateBranches(step, depth)
	}
	return nil
}

// withoutEmpty copies step minus its "" fields, which are incomplete, not invalid.
func withoutEmpty(step map[string]any) map[string]any {
	filled := make(map[string]any, len(step))
	for k, v := range step {
		if v != "" {
			filled[k] = v
		}
	}
	return filled
}

func validName(v any) bool {
	s, ok := v.(string)
	return ok && nameRE.MatchString(s)
}

// stepVerb returns the single verb key of step.
func stepVerb(i int, step map[string]any) (string, error) {
	verb := ""
	for k := range step {
		if _, isVerb := fields[k]; !isVerb {
			continue
		}
		if verb != "" {
			return "", fmt.Errorf("step %d has multiple verbs", i)
		}
		verb = k
	}
	if verb == "" {
		return "", fmt.Errorf("step %d has no verb", i)
	}
	return verb, nil
}

// validateVerbValue checks the value stored under the verb key itself.
func validateVerbValue(verb string, value any) error {
	switch verb {
	case "wait":
		if n, ok := value.(float64); !ok || n < 0 || n > 60000 {
			return fmt.Errorf("invalid wait")
		}
	case "scroll":
		switch value.(type) {
		case string, float64:
		default:
			return fmt.Errorf("invalid scroll")
		}
	default:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("invalid %s", verb)
		}
	}
	return nil
}

// validateBranches checks the nested step lists of an "if" step. A missing
// branch is incomplete; a present one must be a step list.
func validateBranches(step map[string]any, depth int) error {
	for _, branch := range []string{"then", "else"} {
		if v, ok := step[branch]; ok {
			if err := validateSteps(v, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// The client's class-name rule: one class, or a template checked when it runs.
var classNameRE = regexp.MustCompile(`^(?:[A-Za-z0-9_:\-/\[\].%]+|[\s\S]*\{\{[\s\S]*)$`)

// Optional fields: absent is fine, present must have the right type.
func optString(step map[string]any, key string) bool {
	v, exists := step[key]
	if !exists {
		return true
	}
	_, ok := v.(string)
	return ok
}

func optStrings(step map[string]any, keys ...string) bool {
	for _, key := range keys {
		if !optString(step, key) {
			return false
		}
	}
	return true
}

func optEnum(step map[string]any, key string, values ...string) bool {
	v, exists := step[key]
	if !exists {
		return true
	}
	for _, a := range values {
		if v == a {
			return true
		}
	}
	return false
}

func optBool(step map[string]any, key string) bool {
	v, exists := step[key]
	if !exists {
		return true
	}
	_, ok := v.(bool)
	return ok
}

// optNumber requires a finite number in [min, max]; positive makes min exclusive.
func optNumber(step map[string]any, key string, min float64, positive bool, max float64) bool {
	v, exists := step[key]
	if !exists {
		return true
	}
	n, ok := v.(float64)
	return ok && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= min && (!positive || n > min) && n <= max
}

// validateOptions checks the fields whose type does not depend on the verb.
func validateOptions(verb string, step map[string]any) error {
	for _, key := range []string{"newTab", "replace", "smooth"} {
		if !optBool(step, key) {
			return fmt.Errorf("invalid option type or value")
		}
	}
	positiveDuration := verb == "animate" || verb == "toast"
	if !optNumber(step, "duration", 0, positiveDuration, math.MaxFloat64) ||
		!optNumber(step, "offset", -math.MaxFloat64, false, math.MaxFloat64) ||
		!optNumber(step, "timeout", 0, true, 60000) {
		return fmt.Errorf("invalid option type or value")
	}
	return nil
}

// verbValidators check the fields specific to one verb. Verbs absent from the
// table have none beyond validateOptions.
var verbValidators = map[string]func(step map[string]any) error{
	"modal":    validateModal,
	"toast":    validateToast,
	"download": validateDownload,
	"animate":  validateAnimate,
	"class":    validateClass,
	"set":      validateSet,
	"run":      validateRun,
	"call":     validateCall,
}

func validateStepFields(verb string, step map[string]any) error {
	if err := validateOptions(verb, step); err != nil {
		return err
	}
	if validate, ok := verbValidators[verb]; ok {
		return validate(step)
	}
	return nil
}

func validateModal(step map[string]any) error {
	if !optEnum(step, "modal", "open", "close") || !optString(step, "target") {
		return fmt.Errorf("invalid modal")
	}
	return nil
}

func validateToast(step map[string]any) error {
	if !optEnum(step, "kind", "success", "error", "info", "warning") {
		return fmt.Errorf("invalid toast kind")
	}
	return nil
}

func validateDownload(step map[string]any) error {
	if !optString(step, "filename") {
		return fmt.Errorf("invalid filename")
	}
	return nil
}

func validateAnimate(step map[string]any) error {
	if !optEnum(step, "preset", "fadeIn", "slideIn", "bounce", "pulse", "shake", "spin") {
		return fmt.Errorf("invalid animation")
	}
	return nil
}

func validateClass(step map[string]any) error {
	if !optEnum(step, "class", "add", "remove", "toggle") || !optStrings(step, "target", "name") {
		return fmt.Errorf("invalid class")
	}
	if name, ok := step["name"].(string); ok && !classNameRE.MatchString(name) {
		return fmt.Errorf("invalid class name")
	}
	return nil
}

func validateSet(step map[string]any) error {
	if name, ok := step["set"].(string); (ok && !nameRE.MatchString(name)) || !optString(step, "to") {
		return fmt.Errorf("invalid set")
	}
	return nil
}

func validateRun(step map[string]any) error {
	if name, ok := step["run"].(string); ok && !nameRE.MatchString(name) {
		return fmt.Errorf("invalid flow name")
	}
	return nil
}

func validateCall(step map[string]any) error {
	validKind := optEnum(step, "call", "api") && optEnum(step, "method", "GET", "POST", "PUT", "DELETE", "PATCH")
	if !validKind || !optStrings(step, "url", "body") {
		return fmt.Errorf("invalid call")
	}
	return validateHeaders(step)
}

func validateHeaders(step map[string]any) error {
	h, exists := step["headers"]
	if !exists {
		return nil
	}
	headers, ok := obj(h)
	if !ok {
		return fmt.Errorf("invalid headers")
	}
	for _, v := range headers {
		if _, ok := v.(string); !ok {
			return fmt.Errorf("invalid header value")
		}
	}
	return nil
}
