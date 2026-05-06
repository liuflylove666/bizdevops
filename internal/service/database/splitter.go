package database

import "strings"

// SplitStatements 按分号切分 SQL，感知单/双引号、反引号、行/块注释。
// 返回去除首尾空白后的非空语句列表。
func SplitStatements(sql string) []string {
	var stmts []string
	var buf strings.Builder

	runes := []rune(sql)
	n := len(runes)
	i := 0

	var quote rune // 0 / ' / " / `
	inLine := false
	inBlock := false

	flush := func() {
		s := strings.TrimSpace(buf.String())
		if s != "" {
			stmts = append(stmts, s)
		}
		buf.Reset()
	}

	for i < n {
		c := runes[i]

		if inLine {
			buf.WriteRune(c)
			if c == '\n' {
				inLine = false
			}
			i++
			continue
		}
		if inBlock {
			buf.WriteRune(c)
			if c == '*' && i+1 < n && runes[i+1] == '/' {
				buf.WriteRune('/')
				inBlock = false
				i += 2
				continue
			}
			i++
			continue
		}
		if quote != 0 {
			buf.WriteRune(c)
			if c == '\\' && i+1 < n {
				buf.WriteRune(runes[i+1])
				i += 2
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}

		// 未处于引号/注释
		if c == '-' && i+1 < n && runes[i+1] == '-' {
			inLine = true
			buf.WriteRune(c)
			i++
			continue
		}
		if c == '#' {
			inLine = true
			buf.WriteRune(c)
			i++
			continue
		}
		if c == '/' && i+1 < n && runes[i+1] == '*' {
			inBlock = true
			buf.WriteRune(c)
			buf.WriteRune('*')
			i += 2
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			quote = c
			buf.WriteRune(c)
			i++
			continue
		}
		if c == ';' {
			flush()
			i++
			continue
		}
		buf.WriteRune(c)
		i++
	}
	flush()
	return stmts
}
