package main

import (
	"edaApp/patternTree"
	"fmt"
)

func main() {

	patterns := []string{
		"market.*.data",
		"market.*.data.**",
		"user.login.*",
		"service.*.status",
		"**.critical",
	}

	events := []string{
		"market.BTC.data",
		"market.ETH.data.volume",
		"user.login.success",
		"service.web.status",
		"app.service.",
		"app.critical",
	}

	tree := patternTree.NewPatternTree()
	for _, pattern := range patterns {
		tree.AddPattern(pattern)
	}

	for i := 0; i < len(events); i++ {
		event := events[i%len(events)]
		fmt.Println(fmt.Sprintf("%s: %b", event, tree.Matches(event)))
	}
}
