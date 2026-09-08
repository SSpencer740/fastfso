package passkey

import (
	"context"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/fastfso/fastfso/backend/internal/env"
)

// webauthnUser adapts our identity model to the webauthn.User interface.
type webauthnUser struct {
	id          uuid.UUID
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u *webauthnUser) WebAuthnID() []byte                         { return u.id[:] }
func (u *webauthnUser) WebAuthnName() string                       { return u.name }
func (u *webauthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func newWebAuthnUser(id uuid.UUID, email, name string, passkeys []Passkey) *webauthnUser {
	creds := make([]webauthn.Credential, len(passkeys))
	for i, pk := range passkeys {
		transports := make([]protocol.AuthenticatorTransport, len(pk.Transports))
		for j, t := range pk.Transports {
			transports[j] = protocol.AuthenticatorTransport(t)
		}
		creds[i] = webauthn.Credential{
			ID:              pk.CredentialID,
			PublicKey:       pk.PublicKey,
			AttestationType: "",
			Transport:       transports,
			Flags: webauthn.CredentialFlags{
				BackupEligible: pk.BackupEligible,
				BackupState:    pk.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    pk.AAGUID,
				SignCount: uint32(pk.SignCount),
			},
		}
	}
	return &webauthnUser{id: id, name: email, displayName: name, credentials: creds}
}

func newWebAuthn() (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPDisplayName: "fastFSO",
		RPID:          env.WebAuthnRPID(),
		RPOrigins:     []string{env.WebAuthnRPOrigin()},
	})
}

// loadCredentials builds a webauthnUser with credentials from the store.
func loadCredentials(ctx context.Context, store *Store, identityID uuid.UUID, email, name string) (*webauthnUser, error) {
	passkeys, err := store.ListByIdentity(ctx, identityID)
	if err != nil {
		return nil, err
	}
	return newWebAuthnUser(identityID, email, name, passkeys), nil
}
