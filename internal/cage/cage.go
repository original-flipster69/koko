package cage

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
)

var usernamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

type Script struct {
	Username string
	Filename string
	Body     string
}

func Generate(username, goos string) (Script, error) {
	if !usernamePattern.MatchString(username) {
		return Script{}, fmt.Errorf("invalid username %q (use lowercase letters, digits, '-' or '_', starting with a letter or '_')", username)
	}
	password, err := generatePassword()
	if err != nil {
		return Script{}, err
	}
	var body string
	switch goos {
	case "darwin":
		body = darwinScript(username, password)
	case "linux":
		body = linuxScript(username, password)
	default:
		return Script{}, fmt.Errorf("unsupported platform %q (cage supports darwin and linux)", goos)
	}
	return Script{
		Username: username,
		Filename: "cage-" + username + ".sh",
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

func header(username, password string) string {
	return fmt.Sprintf(`#!/bin/sh
# koko :cage setup for a dedicated low-privilege user.
# Review every line before running. Run with:  sudo sh %s
#
# A secure random password was generated below. Change it here before
# running if you prefer your own.
set -eu

NEW_USER="%s"
PASSWORD="%s"
`, "cage-"+username+".sh", username, password)
}

func darwinScript(username, password string) string {
	return header(username, password) + `
NEW_UID=$(( $(dscl . -list /Users UniqueID | awk '{print $2}' | sort -n | tail -n1) + 1 ))
NEW_GID=$(( $(dscl . -list /Groups PrimaryGroupID | awk '{print $2}' | sort -n | tail -n1) + 1 ))

dscl . -create "/Users/$NEW_USER"
dscl . -create "/Users/$NEW_USER" UserShell /bin/zsh
dscl . -create "/Users/$NEW_USER" RealName "$NEW_USER (caged agent)"
dscl . -create "/Users/$NEW_USER" UniqueID "$NEW_UID"
dscl . -create "/Users/$NEW_USER" PrimaryGroupID 20
dscl . -create "/Users/$NEW_USER" NFSHomeDirectory "/Users/$NEW_USER"
dscl . -create "/Users/$NEW_USER" IsHidden 1
dscl . -passwd "/Users/$NEW_USER" "$PASSWORD"
createhomedir -c -u "$NEW_USER"

dscl . -create /Groups/collabo
dscl . -create /Groups/collabo PrimaryGroupID "$NEW_GID"
dscl . -append /Groups/collabo GroupMembership "$(logname)"
dscl . -append /Groups/collabo GroupMembership "$NEW_USER"

mkdir -p "/Users/$NEW_USER/workshop"
chown -R "$NEW_USER:collabo" "/Users/$NEW_USER/workshop"
chmod -R 2770 "/Users/$NEW_USER/workshop"

echo "Done. Add 'umask 002' to your shell rc so shared files stay group-writable."
`
}

func linuxScript(username, password string) string {
	return header(username, password) + `
groupadd -f collabo
useradd -m -s /bin/bash -c "caged agent" -G collabo "$NEW_USER"
printf '%s:%s\n' "$NEW_USER" "$PASSWORD" | chpasswd
usermod -aG collabo "$(logname)"

mkdir -p "/home/$NEW_USER/workshop"
chown -R "$NEW_USER:collabo" "/home/$NEW_USER/workshop"
chmod -R 2770 "/home/$NEW_USER/workshop"

echo "Done. Add 'umask 002' to your shell rc so shared files stay group-writable."
`
}
