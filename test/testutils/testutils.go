package testutils

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// 生成随机临时文件名
func RandomTempFilename(prefix, suffix string) string {
	rand.Seed(time.Now().UnixNano())
	randNum := rand.Intn(100000)
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d_%d%s", prefix, timestamp, randNum, suffix)
}

// 延时清理临时文件
func CleanTempFile(t *testing.T, tempFile string) {
	// 先尝试直接删除
	err := os.Remove(tempFile)
	if err != nil {
		// 进程属性设置
		procAttr := &os.ProcAttr{
			Files: []*os.File{nil, nil, nil}, // 标准输入、输出、错误均设置为nil
			Dir:   "",                        // 使用当前目录
		}

		var executable string
		var args []string

		switch runtime.GOOS {
		case "windows":
			// Windows系统
			executable, err = exec.LookPath("powershell.exe")
			if err != nil {
				t.Logf("Failed to find executable %s: %v", executable, err)
				return
			}
			t.Logf("Executable: %s", executable)
			// 使用Start-Sleep命令等待2秒后再删除
			args = []string{"-Command", fmt.Sprintf("Start-Sleep -Seconds 2; Remove-Item -Path '%s' -Force", tempFile)}
		case "darwin", "linux", "freebsd", "openbsd", "netbsd":
			// Unix系统
			executable = "/bin/sh"
			// 使用sleep命令等待2秒后再删除
			args = []string{"-c", fmt.Sprintf("sleep 2 && rm -f \"%s\"", tempFile)}
		default:
			t.Logf("Unsupported OS: %s", runtime.GOOS)
			return
		}

		// 启动进程
		_, err := os.StartProcess(executable, append([]string{executable}, args...), procAttr)
		if err != nil {
			t.Logf("Start process failed: %v", err)
			return
		}

		t.Logf("File locked, scheduled for deletion by separate process")
	}
}
