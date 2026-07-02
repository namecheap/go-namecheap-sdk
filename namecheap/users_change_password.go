package namecheap

import (
	"context"
	"encoding/xml"
)

// UsersChangePasswordResponse is the raw envelope for
// namecheap.users.changePassword.
type UsersChangePasswordResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersChangePasswordCommandResponse `xml:"CommandResponse"`
}

// UsersChangePasswordCommandResponse wraps the changePassword result.
type UsersChangePasswordCommandResponse struct {
	UserChangePasswordResult *UsersChangePasswordResult `xml:"UserChangePasswordResult"`
}

// UsersChangePasswordResult is the outcome of a password change. Field names
// follow the response table in docs/namecheap-api-v2.md (lines 1179-1182).
type UsersChangePasswordResult struct {
	// Success reports whether the password was changed.
	Success *bool `xml:"Success,attr"`
	// UserID is the unique integer representing the user.
	UserID *int `xml:"UserID,attr"`
}

// UsersChangePasswordArgs are the arguments for ChangePasswordWithContext. It
// supports both documented methods (docs/namecheap-api-v2.md lines 1163-1175):
// old-password (OldPassword + NewPassword) and reset-code (ResetCode +
// NewPassword). Provide exactly one of OldPassword or ResetCode.
//
// Secret handling. OldPassword, ResetCode and NewPassword are placed only in the
// outbound request parameters; they are never stored on the client, logged, or
// echoed in errors by this SDK. They are members of the observability secret-key
// set, so the request/response hooks and slog integration always redact them to
// "***" (see RequestInfo and redactParams). Do not add ad-hoc logging of these
// values.
type UsersChangePasswordArgs struct {
	// OldPassword is the user's current password (method 1). Mutually exclusive
	// with ResetCode.
	OldPassword string
	// ResetCode is the reset code from namecheap.users.resetPassword (method 2).
	// Mutually exclusive with OldPassword.
	ResetCode string
	// NewPassword is the new password. Required for both methods.
	NewPassword string
}

// ChangePasswordWithContext changes the password of the user account, using
// either the old password or a reset code (see UsersChangePasswordArgs).
//
// It is a non-idempotent account write but not charge-bearing; it uses the
// standard idempotent transport (a transient failure is safely retried, since a
// repeated password change to the same NewPassword is harmless).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/change-password/
func (us *UsersService) ChangePasswordWithContext(ctx context.Context, args *UsersChangePasswordArgs) (*UsersChangePasswordCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response UsersChangePasswordResponse
	_, err := us.client.DoXMLWithContext(ctx, args.params(), &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate enforces the required fields and the exactly-one-credential rule.
func (a *UsersChangePasswordArgs) validate() error {
	if a.NewPassword == "" {
		return &InvalidArgumentsError{Fields: []string{"NewPassword"}}
	}
	hasOld := a.OldPassword != ""
	hasReset := a.ResetCode != ""
	switch {
	case hasOld && hasReset:
		return &InvalidArgumentsError{
			Fields: []string{"OldPassword", "ResetCode"},
			Reason: "provide exactly one of OldPassword or ResetCode, not both",
		}
	case !hasOld && !hasReset:
		return &InvalidArgumentsError{
			Fields: []string{"OldPassword", "ResetCode"},
			Reason: "provide exactly one of OldPassword or ResetCode",
		}
	}
	return nil
}

// params flattens the validated args into the request map. Only the chosen
// credential is sent.
func (a *UsersChangePasswordArgs) params() map[string]string {
	params := map[string]string{
		"Command":     "namecheap.users.changePassword",
		"NewPassword": a.NewPassword,
	}
	if a.OldPassword != "" {
		params["OldPassword"] = a.OldPassword
	} else {
		params["ResetCode"] = a.ResetCode
	}
	return params
}
