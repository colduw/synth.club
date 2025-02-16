package database

import (
	"errors"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

type (
	CHandle struct {
		gorm.Model
		Handle string `gorm:"column:handle;unique"`
		DID    string `gorm:"column:did;unique"`
		DHCode string `gorm:"column:dh_code"`
	}
)

// Makes sure the handle is:
// Does not start or end with a hypen
// 3-63 characters long
// lowercase a-z only
var (
	handleValidationRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
	dhValidationRegex     = regexp.MustCompile(`^dh=[0-9a-f]{40}$`)
	errHandleNil          = errors.New("handle is empty")
	errDIDNil             = errors.New("did is empty")
	errInvalidLength      = errors.New("invalid length")
	errValidationFailed   = errors.New("validation failed")
)

// BeforeSave hook to make sure that the handle/did is not empty,
// and that the handle is valid
func (ch *CHandle) BeforeSave(*gorm.DB) error {
	if ch.Handle == "" {
		return errHandleNil
	}

	if ch.DID == "" || !strings.HasPrefix(ch.DID, "did:") {
		return errDIDNil
	}

	if ch.DHCode == "" {
		ch.DHCode = "reserved"
	} else {
		if len(ch.DHCode) != 43 {
			return errInvalidLength
		}

		ch.DHCode = strings.ToLower(ch.DHCode)
		if !dhValidationRegex.MatchString(ch.DHCode) {
			return errValidationFailed
		}
	}

	if len(ch.Handle) < 3 || len(ch.Handle) > 63 {
		return errInvalidLength
	}

	ch.Handle = strings.ToLower(ch.Handle)
	if !handleValidationRegex.MatchString(ch.Handle) {
		return errValidationFailed
	}

	return nil
}
