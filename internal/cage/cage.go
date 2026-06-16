package cage

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
)

const defaultGroup = "collabo"

var namePattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

type Options struct {
	Username string
	Group    string
	GOOS     string
}

type Script struct {
	Username string
	Group    string
	Filename string
	Body     string
}

func Generate(opts Options) (Script, error) {
	if !namePattern.MatchString(opts.Username) {
		return Script{}, fmt.Errorf("invalid username %q (use lowercase letters, digits, '-' or '_', starting with a letter or '_')", opts.Username)
	}
	group := opts.Group
	if group == "" {
		group = defaultGroup
	}
	if !namePattern.MatchString(group) {
		return Script{}, fmt.Errorf("invalid group %q (use lowercase letters, digits, '-' or '_', starting with a letter or '_')", group)
	}
	password, err := generatePassword()
	if err != nil {
		return Script{}, err
	}
	var body string
	switch opts.GOOS {
	case "darwin":
		body = darwinScript(opts.Username, password, group)
	case "linux":
		body = linuxScript(opts.Username, password, group)
	default:
		return Script{}, fmt.Errorf("unsupported platform %q (cage supports darwin and linux)", opts.GOOS)
	}
	return Script{
		Username: opts.Username,
		Group:    group,
		Filename: "cage-" + opts.Username + ".sh",
		Body:     body,
	}, nil
}

func generatePassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func header(username, password, group string) string {
	return fmt.Sprintf(`#!/bin/sh
# koko :cage setup for a dedicated low-privilege user.
# Review every line before running. Run with:  sudo sh %s
#
# A secure random password was generated below. Change it here before
# running if you prefer your own.
set -eu

NEW_USER="%s"
GROUP="%s"
PASSWORD="%s"
`, "cage-"+username+".sh", username, group, password)
}

func darwinScript(username, password, group string) string {
	return header(username, password, group) + `
PREFERRED_ID=69

# Prefer PREFERRED_ID; fall back to (highest existing + 1) if it is taken.
pick_id() {
	taken=$1
	if ! printf '%s\n' "$taken" | grep -qx "$PREFERRED_ID"; then
		echo "$PREFERRED_ID"
		return
	fi
	printf '%s\n' "$taken" | sort -n | tail -n1 | awk '{print $1 + 1}'
}

NEW_UID=$(pick_id "$(dscl . -list /Users UniqueID | awk '{print $2}')")
NEW_GID=$(pick_id "$(dscl . -list /Groups PrimaryGroupID | awk '{print $2}')")

dscl . -create "/Users/$NEW_USER"
dscl . -create "/Users/$NEW_USER" UserShell /bin/zsh
dscl . -create "/Users/$NEW_USER" RealName "$NEW_USER (caged lever)"
dscl . -create "/Users/$NEW_USER" UniqueID "$NEW_UID"
dscl . -create "/Users/$NEW_USER" PrimaryGroupID 20
dscl . -create "/Users/$NEW_USER" NFSHomeDirectory "/Users/$NEW_USER"
dscl . -create "/Users/$NEW_USER" IsHidden 1
dscl . -passwd "/Users/$NEW_USER" "$PASSWORD"
createhomedir -c -u "$NEW_USER"

dscl . -create "/Groups/$GROUP"
dscl . -create "/Groups/$GROUP" PrimaryGroupID "$NEW_GID"
dscl . -append "/Groups/$GROUP" GroupMembership "$(logname)"
dscl . -append "/Groups/$GROUP" GroupMembership "$NEW_USER"

mkdir -p "/Users/$NEW_USER/workshop"
chown -R "$NEW_USER:$GROUP" "/Users/$NEW_USER/workshop"
chmod -R 2770 "/Users/$NEW_USER/workshop"

echo "Done. Add 'umask 002' to your shell rc so shared files stay group-writable."
`
}

func linuxScript(username, password, group string) string {
	return header(username, password, group) + `
PREFERRED_ID=69

# Prefer PREFERRED_ID; fall back to (highest existing + 1) if it is taken.
pick_id() {
	taken=$1
	if ! printf '%s\n' "$taken" | grep -qx "$PREFERRED_ID"; then
		echo "$PREFERRED_ID"
		return
	fi
	printf '%s\n' "$taken" | sort -n | tail -n1 | awk '{print $1 + 1}'
}

NEW_UID=$(pick_id "$(getent passwd | awk -F: '{print $3}')")
NEW_GID=$(pick_id "$(getent group | awk -F: '{print $3}')")

groupadd -g "$NEW_GID" -f "$GROUP"
useradd -m -u "$NEW_UID" -s /bin/bash -c "caged lever" -G "$GROUP" "$NEW_USER"
printf '%s:%s\n' "$NEW_USER" "$PASSWORD" | chpasswd
usermod -aG "$GROUP" "$(logname)"

mkdir -p "/home/$NEW_USER/workshop"
chown -R "$NEW_USER:$GROUP" "/home/$NEW_USER/workshop"
chmod -R 2770 "/home/$NEW_USER/workshop"

echo "Done. Add 'umask 002' to your shell rc so shared files stay group-writable."
`
}
