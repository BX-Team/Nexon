package core

import (
	"strings"
	"testing"

	"github.com/BX-Team/Nexon/internal/store"
)

func TestRenderSubscriptionPrefersTemplate(t *testing.T) {
	svc := testService(t)

	// No template yet → built-in clash generator (contains the NEXON group).
	body, _ := svc.RenderSubscription("clash", &store.User{}, nil)
	if strings.Contains(string(body), "CUSTOM-MARKER") {
		t.Fatal("unexpected custom output before a template is set")
	}

	if err := svc.SetTemplate("clash", "# CUSTOM-MARKER\nproxies:\n{{ .Proxies }}\n"); err != nil {
		t.Fatalf("set template: %v", err)
	}
	body, ctype := svc.RenderSubscription("clash", &store.User{}, nil)
	if !strings.Contains(string(body), "CUSTOM-MARKER") {
		t.Fatalf("expected custom template output, got:\n%s", body)
	}
	if !strings.Contains(ctype, "yaml") {
		t.Fatalf("content type = %q", ctype)
	}
}

func TestSetTemplateRejectsBadSyntax(t *testing.T) {
	svc := testService(t)
	if err := svc.SetTemplate("clash", "{{ .Proxies"); err == nil {
		t.Fatal("expected a template parse error to be rejected")
	}
	// A broken template must not have been stored.
	if _, ok := svc.GetTemplate("clash"); ok {
		t.Fatal("invalid template should not be persisted")
	}
}

func TestSetTemplateUnsupportedFormat(t *testing.T) {
	svc := testService(t)
	if err := svc.SetTemplate("base64", "x"); err == nil {
		t.Fatal("base64 should not accept a template")
	}
}

func TestSubFormatPinnedByClientApp(t *testing.T) {
	svc := testService(t)
	if err := svc.CreateClientApp(&store.ClientApp{
		Name: "mihomo", UAPattern: "[Mm]ihomo", Enabled: true, Sort: 5, Format: "clash",
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}
	if f := svc.SubFormat("mihomo/1.18.0"); f != "clash" {
		t.Fatalf("pinned format = %q, want clash", f)
	}
	if f := svc.SubFormat("curl/8.0"); f != "" {
		t.Fatalf("unmatched UA should yield empty format, got %q", f)
	}
}

func TestRenderPreviewDefaults(t *testing.T) {
	svc := testService(t)
	for _, f := range svc.TemplateFormats() {
		out, err := svc.RenderPreview(f, svc.StarterTemplate(f))
		if err != nil {
			t.Fatalf("starter %s does not render valid output: %v", f, err)
		}
		if out == "" {
			t.Fatalf("starter %s rendered empty", f)
		}
	}
}
