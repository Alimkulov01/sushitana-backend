package tgrouter

func assert1(guard bool, text string) {
	if !guard {
		panic(text)
	}
}