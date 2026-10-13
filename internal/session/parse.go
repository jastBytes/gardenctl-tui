package session

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
)

// ParseKubeconfigExport extracts the KUBECONFIG value from the bash output of
// `gardenctl kubectl-env bash`, e.g.
//
//	export KUBECONFIG='/tmp/garden/sessions/<id>/kubeconfig.yaml';
//
// gardenctl quotes the value POSIX-style: single quotes, with ' written as '"'"'.
func ParseKubeconfigExport(out string) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		rest, ok := strings.CutPrefix(line, "export KUBECONFIG=")
		if !ok {
			continue
		}
		rest = strings.TrimSuffix(strings.TrimSpace(rest), ";")
		v, err := unquotePOSIX(rest)
		if err != nil {
			return "", fmt.Errorf("parse KUBECONFIG from gardenctl kubectl-env: %w", err)
		}
		if v == "" {
			return "", errors.New("gardenctl kubectl-env returned an empty KUBECONFIG")
		}
		return v, nil
	}
	return "", fmt.Errorf("no 'export KUBECONFIG=' line in gardenctl kubectl-env output: %q", strings.TrimSpace(out))
}

// unquotePOSIX decodes a single shell word made of single-quoted, double-quoted
// and unquoted parts (enough for gardenctl's escaping; no variable expansion).
func unquotePOSIX(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch c := s[i]; c {
		case '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return "", errors.New("unterminated single quote")
			}
			b.WriteString(s[i+1 : i+1+end])
			i += end + 2
		case '"':
			j := i + 1
			for ; j < len(s) && s[j] != '"'; j++ {
				c := s[j]
				if c == '$' || c == '`' {
					return "", errors.New("shell expansion is not supported")
				}
				// In double quotes a backslash only escapes $ ` " \ (otherwise it is literal).
				if c == '\\' && j+1 < len(s) && strings.IndexByte("$`\"\\", s[j+1]) >= 0 {
					j++
					c = s[j]
				}
				b.WriteByte(c)
			}
			if j >= len(s) {
				return "", errors.New("unterminated double quote")
			}
			i = j + 1
		case '\\':
			if i+1 < len(s) {
				b.WriteByte(s[i+1])
			}
			i += 2
		case ' ', '\t':
			return "", errors.New("unexpected whitespace in value")
		case '$', '`':
			return "", errors.New("shell expansion is not supported")
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), nil
}
