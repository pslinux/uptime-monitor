package db

import (
	"strings"
)

// extractErr 从 sqlplus 输出与错误中提取首个有意义错误行。
func extractErr(out []byte, err error) string {
	var msgs []string
	if err != nil {
		msgs = append(msgs, err.Error())
	}
	lines := strings.Split(string(out), "\n")
	for _, ln := range lines {
		u := strings.ToUpper(ln)
		if strings.Contains(u, "ORA-") || strings.Contains(u, "ERROR") || strings.Contains(u, "SP2-") {
			t := strings.TrimSpace(ln)
			if t != "" {
				msgs = append(msgs, t)
			}
		}
	}
	if len(msgs) == 0 {
		return "未获取到有效错误信息"
	}
	return strings.Join(msgs, "; ")
}

// firstNonEmptyField 取输出首行首个非空字段。
func firstNonEmptyField(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}
