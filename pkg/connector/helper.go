package connector

import (
	"errors"
	"strings"
	"unicode"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/crypto"
)

const symbols = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

func isPasswordValid(password string) bool {
	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case strings.ContainsRune(symbols, c):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasDigit && hasSpecial
}

// generateCredentials if the credential option is "Random Password", it returns a randomly generated password.
func generateCredentials(credentialOptions *v2.CredentialOptions) (string, error) {
	if credentialOptions.GetRandomPassword() == nil {
		return "", errors.New("unsupported credential option")
	}

	const maxAttempts = 20
	for i := 0; i < maxAttempts; i++ {
		password, err := crypto.GenerateRandomPassword(
			&v2.CredentialOptions_RandomPassword{
				Length: min(12, credentialOptions.GetRandomPassword().GetLength()),
			},
		)
		if err != nil {
			return "", err
		}
		if isPasswordValid(password) {
			return password, nil
		}
	}

	return "", errors.New("failed to generate a valid password after 20 attempts")
}
