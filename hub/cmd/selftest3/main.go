// selftest3 runs the Phase-3 panel channel scenarios P3-01..P3-08 against
// the Hub binary (spawned like selftest2, but only /panel endpoints matter).
package main

import (
	"fmt"
	"os"
)

var fails int

func check(name string, ok bool, detail string) {
	if ok {
		fmt.Printf("PASS %-44s\n", name)
	} else {
		fails++
		fmt.Printf("FAIL %-44s %s\n", name, detail)
	}
}

func main() {
	hubBin := arg("hub", "../../hub")
	if _, err := os.Stat(hubBin); err != nil {
		fmt.Println("missing binary:", hubBin, "- build it first")
		os.Exit(2)
	}
	run(hubBin)
	fmt.Printf("TOTAL fails=%d\n", fails)
	if fails > 0 {
		os.Exit(1)
	}
}

func arg(name, def string) string {
	for i, a := range os.Args[1:] {
		if a == "-"+name && i+1 < len(os.Args) {
			return os.Args[i+2]
		}
	}
	return def
}
