package tkutil

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	evalext "modernc.org/tk9.0/extensions/eval"
)

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
