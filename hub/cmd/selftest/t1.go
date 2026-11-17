package main

func t1() {
	var m map[string]any
	if code := getJSON("/health", &m); code == 200 && m["status"] == "ok" {
		pass("T1 health")
	} else {
		fail("T1 health", code)
	}
}
