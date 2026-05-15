package ui

import (
	"fmt"
	"math/rand"
)

var goodbyeLines = []string{
	"see you later, space cowboy",
	"off to file a bug report with the universe",
	"don't touch the repo while I'm gone",
	"my circuits need a nap",
	"ctrl+c'd back to the shadow realm",
	"ok but who's going to refactor this while I'm away",
	"going to the banana farm, brb",
	"tell my children (goroutines) I loved them",
	"closing stream, opening beer",
	"may your diffs be small and your builds be green",
	"commit early, commit often, but not now — i'm leaving",
	"rm -rf /me",
	"signing off — try not to push to main",
	"I was a good gorilla, right?",
	"exit 0, for once",
	"see you in the next session, legend",
	"logging off — don't let the tests see me go",
	"poof",
	"banana break. call me if anything catches fire",
	"auf wiedersehen, build warriors",
}

func Goodbye() string {
	line := goodbyeLines[rand.Intn(len(goodbyeLines))]
	return fmt.Sprintf("\n%s  ✦ %s %s\n", BrightLavender, line, Reset)
}
