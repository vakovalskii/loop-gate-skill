package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// trimLoop срезает зацикленный хвост текста для обрезка без стрима (23.10.26).
// Раньше клиент получал лучшую попытку целиком, с десятками копий одного куска.
// Теперь текст кончается там, где начинается второй круг цикла: N-граммы,
// встретившиеся не меньше minRepeat раз, - это и есть цикл, и самое раннее
// второе вхождение любой из них - начало повтора. Один круг остаётся, он часть
// ответа. Текст без такого повтора возвращается как есть.
func trimLoop(s string, n, minRepeat int) string {
	var starts []int
	var ws []string
	start := -1
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) {
			if start >= 0 {
				starts, ws = append(starts, start), append(ws, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
		i += size
	}
	if start >= 0 {
		starts, ws = append(starts, start), append(ws, s[start:])
	}
	if n < 2 || len(ws) < n {
		return s
	}
	type seen struct{ count, second int }
	grams := make(map[string]*seen, len(ws))
	for i := 0; i+n <= len(ws); i++ {
		k := strings.Join(ws[i:i+n], " ")
		g := grams[k]
		if g == nil {
			grams[k] = &seen{count: 1, second: -1}
			continue
		}
		g.count++
		if g.second < 0 {
			g.second = i
		}
	}
	cut := -1
	for _, g := range grams {
		if g.count >= minRepeat && (cut < 0 || g.second < cut) {
			cut = g.second
		}
	}
	if cut < 0 {
		return s
	}
	return strings.TrimRightFunc(s[:starts[cut]], unicode.IsSpace)
}
