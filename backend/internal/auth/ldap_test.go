package auth

import (
	"context"
	"errors"
	"testing"

	"incidenthub/backend/internal/config"
)

type ldapRunCall struct {
	name string
	args []string
}

type fakeLDAPRunner struct {
	calls    []ldapRunCall
	outputs  map[string][]byte
	errors   map[string]error
	fallback []byte
}

func (f *fakeLDAPRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := ldapRunCall{name: name, args: append([]string{}, args...)}
	f.calls = append(f.calls, call)
	if err := f.errors[name]; err != nil {
		return f.outputs[name], err
	}
	if out, ok := f.outputs[name]; ok {
		return out, nil
	}
	return f.fallback, nil
}

func TestLDAPAuthenticatorAuthenticate_Disabled(t *testing.T) {
	authenticator := NewLDAPAuthenticator(config.LDAPConfig{Enabled: false})
	if err := authenticator.Authenticate(context.Background(), "user", "pass"); !errors.Is(err, ErrLDAPNotEnabled) {
		t.Fatalf("expected ErrLDAPNotEnabled, got %v", err)
	}
}

func TestLDAPAuthenticatorAuthenticate_Misconfigured(t *testing.T) {
	authenticator := NewLDAPAuthenticator(config.LDAPConfig{Enabled: true})
	if err := authenticator.Authenticate(context.Background(), "user", "pass"); !errors.Is(err, ErrLDAPMisconfigured) {
		t.Fatalf("expected ErrLDAPMisconfigured, got %v", err)
	}
}

func TestLDAPAuthenticatorAuthenticate_Success(t *testing.T) {
	runner := &fakeLDAPRunner{
		outputs: map[string][]byte{
			"ldapsearch": []byte("dn: cn=John Doe,ou=Users,dc=example,dc=org\n"),
			"ldapwhoami": []byte("dn:cn=John Doe,ou=Users,dc=example,dc=org\n"),
		},
		errors: map[string]error{},
	}
	authenticator := newLDAPAuthenticatorWithRunner(config.LDAPConfig{
		Enabled:      true,
		URL:          "ldap://ad.local:389",
		BaseDN:       "dc=example,dc=org",
		BindDN:       "cn=svc,dc=example,dc=org",
		BindPassword: "svc-pass",
		UserFilter:   "(uid=%s)",
	}, runner)

	if err := authenticator.Authenticate(context.Background(), "john.doe", "secret"); err != nil {
		t.Fatalf("expected successful authentication, got %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 ldap commands, got %d", len(runner.calls))
	}
	if runner.calls[0].name != "ldapsearch" {
		t.Fatalf("expected first command ldapsearch, got %s", runner.calls[0].name)
	}
	if runner.calls[1].name != "ldapwhoami" {
		t.Fatalf("expected second command ldapwhoami, got %s", runner.calls[1].name)
	}
}

func TestLDAPAuthenticatorAuthenticate_InvalidCredentials(t *testing.T) {
	runner := &fakeLDAPRunner{
		outputs: map[string][]byte{
			"ldapsearch": []byte("dn: cn=John Doe,ou=Users,dc=example,dc=org\n"),
		},
		errors: map[string]error{
			"ldapwhoami": errors.New("invalid credentials"),
		},
	}
	authenticator := newLDAPAuthenticatorWithRunner(config.LDAPConfig{
		Enabled:    true,
		URL:        "ldap://ad.local:389",
		BaseDN:     "dc=example,dc=org",
		UserFilter: "(uid=%s)",
	}, runner)

	if err := authenticator.Authenticate(context.Background(), "john.doe", "wrong"); !errors.Is(err, ErrLDAPInvalidCredentials) {
		t.Fatalf("expected ErrLDAPInvalidCredentials, got %v", err)
	}
}

func TestLDAPFilterEscape(t *testing.T) {
	got := ldapFilterEscape(`a*(b)\c` + "\x00")
	want := `a\2a\28b\29\5cc\00`
	if got != want {
		t.Fatalf("unexpected escaped filter: got %q want %q", got, want)
	}
}
