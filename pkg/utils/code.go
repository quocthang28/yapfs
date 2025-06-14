package utils

import (
	"crypto/rand"
	"math/big"
	"regexp"
)

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func GenerateCode(length int) (string, error) {
	result := make([]byte, length)
	max := big.NewInt(int64(len(charset)))

	for i := range result {
		num, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}

// IsValidCode validates that a code is exactly 8 alphanumeric characters
func IsValidCode(code string) bool {
	if len(code) != 8 {
		return false
	}
	
	alphanumeric := regexp.MustCompile(`^[A-Za-z0-9]+$`)
	return alphanumeric.MatchString(code)
}
