package main

func emitTypewriter(text string, emit func(string)) {
	for _, r := range text {
		emit(string(r))
	}
}
