package verifier

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	ErrUnexpectedResponse = "Unexpected response from deliverabler"

	// Standard Errors
	ErrTimeout           = "The connection to the mail server has timed out"
	ErrNoSuchHost        = "Mail server does not exist"
	ErrServerUnavailable = "Mail server is unavailable"
	ErrBlocked           = "Blocked by mail server"
	ErrSPF               = "SPF Error"

	// RCPT Errors
	ErrTryAgainLater           = "Try again later"
	ErrFullInbox               = "Recipient out of disk space"
	ErrTooManyRCPT             = "Too many recipients"
	ErrNoRelay                 = "Not an open relay"
	ErrMailboxBusy             = "Mailbox busy"
	ErrExceededMessagingLimits = "Messaging limits have been exceeded"
	ErrNotAllowed              = "Not Allowed"
	ErrNeedMAILBeforeRCPT      = "Need MAIL before RCPT"
	ErrRCPTHasMoved            = "Recipient has moved"
)

// LookupError is an error
type LookupError struct {
	Message string `json:"message" xml:"message"`
	Details string `json:"details" xml:"details"`
	Fatal   bool   `json:"fatal" xml:"fatal"`
}

// newLookupError creates a new LookupError reference and
// returns it
func newLookupError(message, details string, fatal bool) *LookupError {
	return &LookupError{message, details, fatal}
}

// Error satisfies the error interface
func (e *LookupError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s : %s", e.Message, e.Details)
}

// ParseSMTPError receives an MX Servers response message
// and generates the cooresponding MX error
func ParseSMTPError(err error) *LookupError {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	// Verify the length of the error before reading nil indexes
	if len(errStr) < 3 {
		return parseBasicErr(err)
	}

	// Strips out the status code string and converts to an integer for parsing
	status, convErr := strconv.Atoi(string([]rune(errStr)[0:3]))
	if convErr != nil {
		return parseBasicErr(err)
	}

	// If the status code is above 400 there was an error and we should return it
	if status > 400 {
		// Don't return an error if the error contains anything about the address
		// being undeliverable
		if insContains(errStr,
			"undeliverable",
			"does not exist",
			"may not exist",
			"user unknown",
			"user not found",
			"invalid address",
			"recipient invalid",
			"recipient rejected",
			"no mailbox") {
			return nil
		}

		switch status {
		case 421:
			return newLookupError(ErrTryAgainLater, errStr, false)
		case 450:
			return newLookupError(ErrMailboxBusy, errStr, false)
		case 451:
			return newLookupError(ErrExceededMessagingLimits, errStr, false)
		case 452:
			if insContains(errStr,
				"full",
				"space",
				"over quota",
				"insufficient",
			) {
				return newLookupError(ErrFullInbox, errStr, false)
			}
			return newLookupError(ErrTooManyRCPT, errStr, false)
		case 503:
			return newLookupError(ErrNeedMAILBeforeRCPT, errStr, false)
		case 550: // 550 is Mailbox Unavailable - usually undeliverable
			if insContains(errStr,
				"spamhaus",
				"proofpoint",
				"cloudmark",
				"banned",
				"blacklisted",
				"blocked",
				"block list",
				"denied") {
				return newLookupError(ErrBlocked, errStr, true)
			} else if insContains(errStr,
				"SPF Sender",
				"SPF Policy") {
				return newLookupError(ErrSPF, errStr, true)
			}
			return nil
		case 551:
			return newLookupError(ErrRCPTHasMoved, errStr, false)
		case 552:
			return newLookupError(ErrFullInbox, errStr, false)
		case 553:
			return newLookupError(ErrNoRelay, errStr, false)
		case 554:
			return newLookupError(ErrNotAllowed, errStr, false)
		default:
			return parseBasicErr(err)
		}
	}
	return nil
}

// parseBasicErr parses a basic MX record response and returns
// a more understandable LookupError
func parseBasicErr(err error) *LookupError {
	if err == nil {
		return nil
	}
	errStr := err.Error()

	// Return a more understandable error
	switch {
	case insContains(errStr,
		"spamhaus",
		"proofpoint",
		"cloudmark",
		"banned",
		"blocked",
		"denied"):
		return newLookupError(ErrBlocked, errStr, true)
	case insContains(errStr, "timeout"):
		return newLookupError(ErrTimeout, errStr, true)
	case insContains(errStr, "no such host"):
		return newLookupError(ErrNoSuchHost, errStr, false)
	case insContains(errStr, "unavailable"):
		return newLookupError(ErrServerUnavailable, errStr, false)
	default:
		return newLookupError(errStr, errStr, false)
	}
}

// insContains returns true if any of the substrings
// are found in the passed string. This method of checking
// contains is case insensitive
func insContains(str string, subStrs ...string) bool {
	for _, subStr := range subStrs {
		if strings.Contains(strings.ToLower(str),
			strings.ToLower(subStr)) {
			return true
		}
	}
	return false
}
