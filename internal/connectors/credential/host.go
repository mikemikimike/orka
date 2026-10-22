/*
Copyright (c) 2026.

MIT License - see LICENSE file for details.
*/

package credential

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/orka-agents/orka/api/v1alpha1"
	"github.com/orka-agents/orka/internal/connectors"
	"github.com/orka-agents/orka/internal/store"
)

// ErrHostLinkUnavailable reports a Connection that can no longer feed an
// agent host: it is gone or unlinked, its consent no longer covers the
// provider, or the provider no longer serves hosts from the host's template.
var ErrHostLinkUnavailable = errors.New("the agent host's GitHub link is unavailable")

// HostCredential is the access token Orka hands a person's agent host.
type HostCredential struct {
	AccessToken string
	// ExpiresAt is zero when the provider reported no expiry.
	ExpiresAt time.Time
	// Version changes whenever custody holds new material.
	Version       int64
	GrantSequence int64
}

// HostCredential returns the access token for the agent host fed by the named
// Connection, refreshed first when it would expire within horizon. Only the
// access token leaves custody; the refresh token never does. It shares the
// tool path's single-flight refresh, so a host and a tool call never refresh
// the same grant twice, and a refresh made for either replaces the other's
// token.
func (s *Source) HostCredential(ctx context.Context, namespace, name, template string, horizon time.Duration) (HostCredential, error) {
	connection, ref, credential, err := s.hostLink(ctx, namespace, name, template)
	if err != nil {
		return HostCredential{}, err
	}
	if s.expiresWithin(credential, horizon) {
		refreshed, err := s.refreshSingleFlight(ctx, connection, ref, credential.GrantSequence, horizon)
		if err != nil {
			// A refresh that ended the link (a revoked or expired grant)
			// shows on the Connection; anything else is worth a retry.
			if _, linkErr := s.hostConnection(ctx, namespace, name); linkErr != nil {
				return HostCredential{}, linkErr
			}
			return HostCredential{}, err
		}
		credential = refreshed
	}
	return HostCredential{
		AccessToken:   credential.AccessToken,
		ExpiresAt:     credential.ExpiresAt,
		Version:       credential.Version,
		GrantSequence: credential.GrantSequence,
	}, nil
}

// CheckHostLink reports whether the named Connection can still feed an agent
// host from template, without refreshing its token: a suspended host keeps
// its token copy only while it can.
func (s *Source) CheckHostLink(ctx context.Context, namespace, name, template string) error {
	_, _, _, err := s.hostLink(ctx, namespace, name, template)
	return err
}

// hostLink loads a host's Connection and the credential it holds, and checks
// that the link may feed a host from template.
func (s *Source) hostLink(ctx context.Context, namespace, name, template string) (
	*corev1alpha1.Connection, store.ConnectorCredentialRef, store.ConnectorCredential, error) {
	var ref store.ConnectorCredentialRef
	if s == nil || s.Client == nil || s.Credentials == nil {
		return nil, ref, store.ConnectorCredential{}, errors.New("connection credential source is not configured")
	}
	connection, err := s.hostConnection(ctx, namespace, name)
	if err != nil {
		return nil, ref, store.ConnectorCredential{}, err
	}
	provider := &corev1alpha1.ConnectorProvider{}
	key := types.NamespacedName{Namespace: namespace, Name: connection.Spec.ProviderRef.Name}
	if err := s.reader().Get(ctx, key, provider); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ref, store.ConnectorCredential{}, hostLinkUnavailable("its connector provider is gone")
		}
		return nil, ref, store.ConnectorCredential{}, fmt.Errorf("load connector provider: %w", err)
	}
	if provider.Spec.AgentHost == nil || provider.Spec.AgentHost.TemplateRef.Name != template {
		return nil, ref, store.ConnectorCredential{}, hostLinkUnavailable("its connector provider no longer serves agent hosts from this template")
	}
	ref, err = connectors.CredentialRef(connection)
	if err != nil {
		return nil, ref, store.ConnectorCredential{}, err
	}
	credential, err := s.Credentials.GetConnectorCredential(ctx, ref)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ref, store.ConnectorCredential{}, hostLinkUnavailable("it holds no credential; the person must reconnect")
	}
	if err != nil {
		return nil, ref, store.ConnectorCredential{}, err
	}
	if _, err := s.validateProvider(ctx, connection, credential, nil); err != nil {
		return nil, ref, store.ConnectorCredential{}, hostLinkUnavailable(err.Error())
	}
	return connection, ref, credential, nil
}

// hostConnection reads a host's Connection fresh and requires it linked.
func (s *Source) hostConnection(ctx context.Context, namespace, name string) (*corev1alpha1.Connection, error) {
	connection := &corev1alpha1.Connection{}
	if err := s.reader().Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, connection); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, hostLinkUnavailable("the Connection is gone")
		}
		return nil, fmt.Errorf("load connection: %w", err)
	}
	if !connectors.ConnectionLinked(connection) {
		return nil, hostLinkUnavailable("the Connection is not linked")
	}
	return connection, nil
}

func hostLinkUnavailable(reason string) error {
	return fmt.Errorf("%w: %s", ErrHostLinkUnavailable, reason)
}
