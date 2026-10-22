/*
Copyright (c) 2026.

MIT License - see LICENSE file for details.
*/

package credential

import (
	"context"
	"errors"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/orka-agents/orka/api/v1alpha1"
	"github.com/orka-agents/orka/internal/connectors"
	"github.com/orka-agents/orka/internal/store"
)

// serveHosts makes the fixture provider serve agent hosts from template, with
// the Connection's consent covering it, as a fresh link would.
func (h *harness) serveHosts(template string) {
	h.t.Helper()
	ctx := context.Background()
	provider := &corev1alpha1.ConnectorProvider{}
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "github"}, provider); err != nil {
		h.t.Fatal(err)
	}
	provider.Spec.AgentHost = &corev1alpha1.ConnectorAgentHost{TemplateRef: corev1alpha1.LocalObjectReference{Name: template}}
	if err := h.client.Update(ctx, provider); err != nil {
		h.t.Fatal(err)
	}
	connection := h.reload()
	connection.Status.Consent = connectors.ConsentFor(provider)
	if err := h.client.Status().Update(ctx, connection); err != nil {
		h.t.Fatal(err)
	}
}

func TestHostCredentialReturnsTheAccessTokenAndRefreshesBeforeExpiry(t *testing.T) {
	h := newHarness(t)
	h.serveHosts("copilotd")
	h.put(store.ConnectorCredential{AccessToken: "ghu_held", RefreshToken: "ghr_held", ExpiresAt: h.now.Add(4 * time.Hour)})

	held, err := h.source.HostCredential(context.Background(), testNamespace, h.connection.Name, "copilotd", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if held.AccessToken != "ghu_held" || h.refresher.calls.Load() != 0 {
		t.Fatalf("held = %+v, refreshes = %d", held, h.refresher.calls.Load())
	}
	if err := h.source.CheckHostLink(context.Background(), testNamespace, h.connection.Name, "copilotd"); err != nil {
		t.Fatal(err)
	}

	// A token that expires within the horizon is refreshed, and the new one
	// returned.
	h.put(store.ConnectorCredential{AccessToken: "ghu_held", RefreshToken: "ghr_held", ExpiresAt: h.now.Add(10 * time.Minute)})
	held, err = h.source.HostCredential(context.Background(), testNamespace, h.connection.Name, "copilotd", time.Minute)
	if err != nil || held.AccessToken != "ghu_held" {
		t.Fatalf("held = %+v, %v", held, err)
	}
	refreshed, err := h.source.HostCredential(context.Background(), testNamespace, h.connection.Name, "copilotd", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.AccessToken != "gho_new" || h.refresher.calls.Load() != 1 || refreshed.Version == held.Version {
		t.Fatalf("refreshed = %+v, refreshes = %d", refreshed, h.refresher.calls.Load())
	}
}

func TestHostCredentialIsUnavailableWhenTheLinkCannotFeedAHost(t *testing.T) {
	for name, setup := range map[string]func(h *harness){
		"provider serves no hosts": func(h *harness) {},
		"another template":         func(h *harness) { h.serveHosts("other") },
		"consent predates hosts": func(h *harness) {
			h.serveHosts("copilotd")
			connection := h.reload()
			connection.Status.Consent.AuthorityDigest = "stale"
			if err := h.client.Status().Update(context.Background(), connection); err != nil {
				h.t.Fatal(err)
			}
		},
		"connection gone": func(h *harness) {
			h.serveHosts("copilotd")
			if err := h.client.Delete(context.Background(), h.reload()); err != nil {
				h.t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.put(store.ConnectorCredential{AccessToken: "ghu_held", RefreshToken: "ghr_held", ExpiresAt: h.now.Add(4 * time.Hour)})
			setup(h)
			_, err := h.source.HostCredential(context.Background(), testNamespace, h.connection.Name, "copilotd", 15*time.Minute)
			if !errors.Is(err, ErrHostLinkUnavailable) {
				t.Fatalf("err = %v, want ErrHostLinkUnavailable", err)
			}
			if err := h.source.CheckHostLink(context.Background(), testNamespace, h.connection.Name, "copilotd"); !errors.Is(err, ErrHostLinkUnavailable) {
				t.Fatalf("check = %v, want ErrHostLinkUnavailable", err)
			}
		})
	}
}
