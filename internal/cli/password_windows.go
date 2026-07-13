//go:build windows

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

func readHiddenToken(out io.Writer, confirm bool) (string, error) {
	fmt.Fprint(out, "Token：")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", fmt.Errorf("读取隐藏 Token 失败：%w", err)
	}
	value := strings.TrimSpace(string(first))
	if value == "" {
		return "", fmt.Errorf("不接受空 Token")
	}
	if !confirm {
		return value, nil
	}
	fmt.Fprint(out, "确认 Token：")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", fmt.Errorf("确认隐藏 Token 失败：%w", err)
	}
	if value != strings.TrimSpace(string(second)) {
		return "", fmt.Errorf("两次输入的 Token 不一致")
	}
	return value, nil
}
