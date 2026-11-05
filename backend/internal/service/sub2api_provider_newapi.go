package service

import (
	"net/mail"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func validProviderProtocolAuth(providerType, mode string) bool {
	if mode == "" {
		mode = domain.Sub2APIProviderAuthModePassword
	}
	if mode == domain.Sub2APIProviderAuthModePassword {
		return providerType == "" || domain.IsValidProviderType(providerType)
	}
	if providerType == domain.ProviderTypeNewAPI {
		return mode == domain.Sub2APIProviderAuthModeAccessToken
	}
	return (providerType == "" || providerType == domain.ProviderTypeSub2API) && mode == domain.Sub2APIProviderAuthModeTokenPair
}

// Email remains the wire/storage name for compatibility. For New API this
// field contains a username (password mode) or numeric user ID (token mode).
func validateProviderIdentity(providerType, mode, identity string) error {
	if !validProviderProtocolAuth(providerType, mode) {
		return ErrInvalidProviderAuth
	}
	identity = strings.TrimSpace(identity)
	if identity == "" || len(identity) > 200 {
		return infraerrors.BadRequest("INVALID_PROVIDER_IDENTITY", "provider login identity is required (maximum 200 bytes)")
	}
	if providerType == domain.ProviderTypeNewAPI {
		if mode == domain.Sub2APIProviderAuthModeAccessToken {
			id, err := strconv.ParseInt(identity, 10, 64)
			if err != nil || id <= 0 {
				return infraerrors.BadRequest("INVALID_PROVIDER_IDENTITY", "New API management token requires a positive numeric user ID")
			}
		}
		return nil
	}
	addr, err := mail.ParseAddress(identity)
	if err != nil || addr.Address != identity {
		return infraerrors.BadRequest("INVALID_PROVIDER_IDENTITY", "Sub2API requires an email address")
	}
	return nil
}
