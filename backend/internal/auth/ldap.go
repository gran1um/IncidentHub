package auth

import (
	"context"
	"errors"
	"fmt"
	"incidenthub/backend/internal/config"
	"os/exec"
	"strings"
)

var ErrLDAPNotEnabled = errors.New("ldap authentication is disabled")
var ErrLDAPInvalidCredentials = errors.New("ldap invalid credentials")
var ErrLDAPMisconfigured = errors.New("ldap configuration is invalid")
var errLDAPUserDNNotFound = errors.New("ldap user dn not found")

type ldapCommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execLDAPCommandRunner struct{}

func (execLDAPCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type LDAPAuthenticator struct {
	cfg    config.LDAPConfig
	runner ldapCommandRunner
}

func NewLDAPAuthenticator(cfg config.LDAPConfig) *LDAPAuthenticator {
	return &LDAPAuthenticator{
		cfg:    cfg,
		runner: execLDAPCommandRunner{},
	}
}

func newLDAPAuthenticatorWithRunner(cfg config.LDAPConfig, runner ldapCommandRunner) *LDAPAuthenticator {
	if runner == nil {
		runner = execLDAPCommandRunner{}
	}
	return &LDAPAuthenticator{
		cfg:    cfg,
		runner: runner,
	}
}

func (a *LDAPAuthenticator) Authenticate(ctx context.Context, username string, password string) error {
	if !a.cfg.Enabled {
		return ErrLDAPNotEnabled
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return ErrLDAPInvalidCredentials
	}
	if strings.TrimSpace(a.cfg.URL) == "" || strings.TrimSpace(a.cfg.BaseDN) == "" || strings.TrimSpace(a.cfg.UserFilter) == "" {
		return ErrLDAPMisconfigured
	}

	searchArgs := []string{
		"-LLL",
		"-x",
		"-H", strings.TrimSpace(a.cfg.URL),
		"-b", strings.TrimSpace(a.cfg.BaseDN),
		fmt.Sprintf(strings.TrimSpace(a.cfg.UserFilter), ldapFilterEscape(username)),
		"dn",
	}
	if strings.TrimSpace(a.cfg.BindDN) != "" {
		searchArgs = append(searchArgs, "-D", strings.TrimSpace(a.cfg.BindDN), "-w", a.cfg.BindPassword)
	}

	searchOutput, err := a.runner.Run(ctx, "ldapsearch", searchArgs...)
	if err != nil {
		return fmt.Errorf("ldapsearch failed: %w", err)
	}
	userDN, err := extractLDAPDN(searchOutput)
	if err != nil {
		return ErrLDAPInvalidCredentials
	}

	whoamiArgs := []string{
		"-x",
		"-H", strings.TrimSpace(a.cfg.URL),
		"-D", userDN,
		"-w", password,
	}
	if _, err := a.runner.Run(ctx, "ldapwhoami", whoamiArgs...); err != nil {
		return ErrLDAPInvalidCredentials
	}
	return nil
}

func extractLDAPDN(output []byte) (string, error) {
	lines := strings.Split(string(output), "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(line), "dn:") {
			continue
		}
		dn := strings.TrimSpace(line[3:])
		if dn == "" {
			continue
		}
		return dn, nil
	}
	return "", errLDAPUserDNNotFound
}

func ldapFilterEscape(input string) string {
	replacer := strings.NewReplacer(
		"\\", "\\5c",
		"*", "\\2a",
		"(", "\\28",
		")", "\\29",
		"\x00", "\\00",
	)
	return replacer.Replace(strings.TrimSpace(input))
}
