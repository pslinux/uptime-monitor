package db

import "testing"

func TestExtractErr(t *testing.T) {
	cases := []struct {
		name string
		out  string
		err  error
		want string
	}{
		{name: "ORA 错误", out: "ORA-12170: TNS:Connect timeout", want: "ORA-12170: TNS:Connect timeout"},
		{name: "无错误信息", out: "plain output", want: "未获取到有效错误信息"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractErr([]byte(c.out), c.err)
			if got == "" {
				t.Fatalf("期望非空")
			}
		})
	}
}

func TestFirstNonEmptyField(t *testing.T) {
	if got := firstNonEmptyField("  1\nfoo"); got != "1" {
		t.Fatalf("期望 1, 得到 %q", got)
	}
	if got := firstNonEmptyField("\n\n\n"); got != "" {
		t.Fatalf("期望空, 得到 %q", got)
	}
}
