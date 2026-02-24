package backend

import (
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/go-srp"
	"github.com/google/uuid"
	"github.com/rclone/go-proton-api"
)

func (b *Backend) NewAuthInfo(username string) (proton.AuthInfo, error) {
	return withAccName(b, username, func(acc *account) (proton.AuthInfo, error) {
		server, err := srp.NewServerFromSigned(modulus, acc.verifier, 2048)
		if err != nil {
			return proton.AuthInfo{}, nil
		}

		challenge, err := server.GenerateChallenge()
		if err != nil {
			return proton.AuthInfo{}, nil
		}

		session := uuid.NewString()

		b.srpLock.Lock()
		defer b.srpLock.Unlock()

		b.srp[session] = server

		return proton.AuthInfo{
			Version:         4,
			Modulus:         modulus,
			ServerEphemeral: base64.StdEncoding.EncodeToString(challenge),
			Salt:            base64.StdEncoding.EncodeToString(acc.salt),
			SRPSession:      session,
		}, nil
	})
}

func (b *Backend) NewAuth(username string, ephemeral, proof []byte, session string) (proton.Auth, error) {
	return withAccName(b, username, func(acc *account) (proton.Auth, error) {
		b.srpLock.Lock()
		defer b.srpLock.Unlock()

		server, ok := b.srp[session]
		if !ok {
			return proton.Auth{}, fmt.Errorf("invalid session")
		}

		delete(b.srp, session)

		serverProof, err := server.VerifyProofs(ephemeral, proof)
		if !ok {
			return proton.Auth{}, fmt.Errorf("invalid proof: %w", err)
		}

		authUID, auth := uuid.NewString(), newAuth(b.authLife)

		acc.authLock.Lock()
		defer acc.authLock.Unlock()

		acc.auth[authUID] = auth

		return auth.toAuth(acc.userID, authUID, serverProof), nil
	})
}

func (b *Backend) NewAuthRef(authUID, authRef string) (proton.Auth, error) {
	b.accLock.RLock()
	defer b.accLock.RUnlock()

	for _, acc := range b.accounts {
		acc.authLock.Lock()
		defer acc.authLock.Unlock()

		auth, ok := acc.auth[authUID]
		if !ok {
			continue
		}

		if auth.ref != authRef {
			return proton.Auth{}, fmt.Errorf("invalid auth ref")
		}

		newAuth := newAuth(b.authLife)

		acc.auth[authUID] = newAuth

		return newAuth.toAuth(acc.userID, authUID, nil), nil
	}

	return proton.Auth{}, fmt.Errorf("invalid auth")
}

func (b *Backend) VerifyAuth(authUID, authAcc string) (string, error) {
	return withAccAuth(b, authUID, authAcc, func(acc *account) (string, error) {
		return acc.userID, nil
	})
}

func (b *Backend) GetSessions(userID string) ([]proton.AuthSession, error) {
	return withAcc(b, userID, func(acc *account) ([]proton.AuthSession, error) {
		acc.authLock.RLock()
		defer acc.authLock.RUnlock()

		var sessions []proton.AuthSession

		for authUID, auth := range acc.auth {
			sessions = append(sessions, auth.toAuthSession(authUID))
		}

		return sessions, nil
	})
}

func (b *Backend) DeleteSession(userID, authUID string) error {
	return b.withAcc(userID, func(acc *account) error {
		acc.authLock.Lock()
		defer acc.authLock.Unlock()

		delete(acc.auth, authUID)

		return nil
	})
}
