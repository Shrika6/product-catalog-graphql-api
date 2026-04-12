package service

import (
	"regexp"
	"strings"
)

var currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)

func normalizeName(name string) string {
	return strings.TrimSpace(name)
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}
