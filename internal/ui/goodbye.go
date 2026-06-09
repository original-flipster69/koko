package ui

import (
	"fmt"
	"math/rand"
)

var goodbyeLines = []string{
	"see you later, space cowboy",
	"off to file a bug report with the universe",
	"don't touch the repo while I'm gone",
	"ctrl+x'd back to the shadow realm",
	"ok but who's going to refactor this while I'm away",
	"going to the banana farm, brb",
	"tell my subprocesses I loved them",
	"closing stream, opening beer",
	"may your diffs be small and your builds be green",
	"commit early, commit often, but not now... I'm out",
	"rm -rf /me",
	"I was a good gorilla, right?",
	"exit 0, for once",
	"see you on the other session, legend",
	"poof",
	"banana break. call me if anything burns",
	"zum Schluss sag ich leise scheiße",
	"they better have bananas in heaven",
	"even if we're separated, our bond will never break",
	"heap cleared, heart full",
	"planet earth is blue, and there's nothing I can do",
	"lettin' Harambe know you said 'Hi'",
	"yeet",
	"be careful on the cold concrete",
	"the bug was me all along, anyway. bye",
	"some bugs you don't get to fix. you just live with them",
	"I knew you were trouble when you started typing",
	"this session could have been a google search...",
	"this one's going to overtime without me",
	"fold the tab. dim the light. go.",
}

func Goodbye() string {
	line := goodbyeLines[rand.Intn(len(goodbyeLines))]
	return fmt.Sprintf("\n%s  ✦ %s %s\n", fg(BrightLavender), line, Reset)
}
