package hooks

import "testing"

func TestDebugNormalize(t *testing.T) {
	body := `function(ctx, payload) { payload.value = payload.value.toLowerCase(); return payload; }`
	t.Logf("body repr: %q", body)
	t.Logf("HasPrefix function: %v", len(body) > 0 && body[:8] == "function")
	norm := normalizeHookBody(body)
	t.Logf("normalized: %q", norm)

	// también testear con newline/tab
	body2 := "function(ctx, payload) {\n\t\t\tpayload.value = payload.value.toLowerCase();\n\t\t\treturn payload;\n\t\t}"
	t.Logf("body2 normalized: %q", normalizeHookBody(body2))
}
