package tkutil

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	evalext "modernc.org/tk9.0/extensions/eval"
)

// Mirrors modernc.org/tk9.0 tclSafeString escaping rules.
var badChars = [...]bool{
	' ':  true,
	'"':  true,
	'$':  true,
	'&':  true,
	'(':  true,
	')':  true,
	'*':  true,
	';':  true,
	'<':  true,
	'>':  true,
	'?':  true,
	'[':  true,
	'\'': true,
	'\\': true,
	'\n': true,
	'\r': true,
	'\t': true,
	']':  true,
	'^':  true,
	'`':  true,
	'{':  true,
	'|':  true,
	'}':  true,
	'~':  true,
}

func Eval(format string, a ...any) (string, error) {
	eval := fmt.Sprintf(format, a...)
	r, err := evalext.Eval(eval)
	if err != nil {
		return "", fmt.Errorf("tk eval=%s; err=%w", eval, err)
	}
	return r, nil
}

func EvalOrEmpty(format string, a ...any) string {
	out, err := Eval(format, a...)
	if err != nil {
		slog.Debug("tk eval or empty", slog.Any("error", err))
		return ""
	}
	return out
}

func MustEval(format string, a ...any) string {
	r, err := Eval(format, a...)
	if err != nil {
		panic(err)
	}
	return r
}

func Atoi(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		if f, ferr := strconv.ParseFloat(raw, 64); ferr == nil {
			return int(f)
		}
		return 0
	}
	return v
}

// TclSafeString returns a Tcl-safe string; empty input returns "{}".
func TclSafeString(s string) string {
	if s == "" {
		return "{}"
	}

	const badString = "&;`'\"|*?~<>^()[]{}$\\\n\r\t "
	if strings.ContainsAny(s, badString) {
		var b strings.Builder
		for _, c := range s {
			switch {
			case int(c) < len(badChars) && badChars[c]:
				fmt.Fprintf(&b, "\\x%02x", c)
			default:
				b.WriteRune(c)
			}
		}
		s = b.String()
	}
	return s
}

// TclSafeStrings returns a space-separated list of Tcl-safe strings.
func TclSafeStrings(s ...string) string {
	if len(s) == 0 {
		return ""
	}
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = TclSafeString(v)
	}
	return strings.Join(out, " ")
}
