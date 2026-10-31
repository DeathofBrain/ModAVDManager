// Package report contains the read-only diagnostics used by the report-config
// and collect-info CLI commands.  The package deliberately does not start adb,
// an emulator, or any other long lived process.
package report

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"modavd/internal/errs"
	"modavd/internal/root"
)

const privacyNote = "隐私提示：问题包只保存在本机，命令不会上传。\n" +
	"- 问题包可能含本机路径、Windows 用户名、截图和 AUTO-MAS 日志；上传前请检查内容。\n" +
	"- summary.txt 和日志通常可以贴到 Issue；crash 目录含线程栈和少量内存片段，请私下发送。"

type InstanceConfig struct {
	Index int      `json:"index"`
	Name  string   `json:"name"`
	RAM   string   `json:"memory"`
	Games []string `json:"games"`
}

type ConfigResult struct {
	Text      string           `json:"text"`
	System    []string         `json:"system"`
	Simulator []string         `json:"simulator"`
	Instances []InstanceConfig `json:"instances"`
}

type ConfigOptions struct {
	NoClipboard bool
}

// BuildConfig creates the same user-facing report as the bundled PowerShell
// helper.  It only asks an already-running private adb server for packages.
func BuildConfig(ctx context.Context, r *root.Root, opt ConfigOptions) (*ConfigResult, error) {
	if runtime.GOOS != "windows" {
		return nil, errs.New(errs.Unsupported, "复制配置信息目前只支持 Windows")
	}
	res := &ConfigResult{}
	res.System = collectWindowsSystem(ctx)
	res.Simulator = collectSimulator(ctx, r)
	res.Instances = collectInstances(ctx, r)
	if res.System == nil {
		res.System = []string{}
	}
	if res.Simulator == nil {
		res.Simulator = []string{}
	}
	if res.Instances == nil {
		res.Instances = []InstanceConfig{}
	}
	lines := append([]string{"【魔改 AVD 试用配置】"}, res.System...)
	lines = append(lines, res.Simulator...)
	if len(res.Instances) == 0 {
		lines = append(lines, "游戏包：没有实例开着，读不到。在 AUTO-MAS 里开着实例时再运行一次，就会列出来")
	} else {
		for _, in := range res.Instances {
			games := "没装"
			if len(in.Games) > 0 {
				games = strings.Join(in.Games, "、")
			}
			lines = append(lines, fmt.Sprintf("实例 %d%s：%s", in.Index, in.RAM, games))
		}
	}
	lines = append(lines, "AUTO-MAS 版本：", "用的脚本（MAA / M9A / …）：", "效果（能跑 / 有问题，问题是…）：")
	res.Text = strings.Join(lines, "\r\n")
	if !opt.NoClipboard {
		if err := copyClipboard(res.Text); err != nil {
			return nil, errs.Wrap(errs.Unsupported, err, "复制到剪贴板失败，请手动复制上面的文字")
		}
	}
	return res, nil
}

func collectWindowsSystem(ctx context.Context) []string {
	ps := `$OutputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding
$os=Get-CimInstance Win32_OperatingSystem
$cv=Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion'
$v=''; if($cv.DisplayVersion){$v=$cv.DisplayVersion}elseif($cv.ReleaseId){$v=$cv.ReleaseId}; $v=('{0}（{1}.{2}）' -f $v,$cv.CurrentBuild,$cv.UBR).Trim()
Write-Output ('系统：{0} {1}' -f ($os.Caption -replace '^Microsoft\s+',''),$v)
foreach($c in @(Get-CimInstance Win32_Processor)){Write-Output ('CPU：{0}（{1} 核 {2} 线程）' -f $c.Name.Trim(),$c.NumberOfCores,$c.NumberOfLogicalProcessors)}
Write-Output ('内存：{0:N0} GB（现在可用 {1:N0} GB）' -f ([double]$os.TotalVisibleMemorySize/1MB),([double]$os.FreePhysicalMemory/1MB))
$g=@(Get-CimInstance Win32_VideoController | ? {$_.PNPDeviceID -like 'PCI\*'}); if($g.Count -eq 0){$g=@(Get-CimInstance Win32_VideoController)}
foreach($x in $g){$d='';if($x.DriverDate){$d='，'+([datetime]$x.DriverDate).ToString('yyyy-MM-dd')};Write-Output ('显卡：{0}（驱动 {1}{2}）' -f $x.Name,$x.DriverVersion,$d)}`
	return runPowerShellLines(ctx, ps)
}

func collectSimulator(ctx context.Context, r *root.Root) []string {
	build := r.EmulatorVersion()
	if build == "" {
		build = "读不到"
	}
	free := ""
	if fi, err := os.Stat(r.Dir); err == nil && fi.IsDir() {
		free = ""
	}
	lines := []string{fmt.Sprintf("模拟器：%s%s", build, free)}
	check := filepath.Join(r.RuntimeSDK(), "emulator", "emulator-check.exe")
	if _, err := os.Stat(check); err == nil {
		if got := runCommandLines(ctx, check, "accel"); len(got) > 0 {
			lines = append(lines, "硬件加速："+strings.Join(got, " "))
		}
	}
	return lines
}

func runPowerShellLines(ctx context.Context, script string) []string {
	got := runCommandLines(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	if len(got) == 0 {
		return []string{"系统：Windows（硬件信息读不到）"}
	}
	return got
}

func runCommandLines(ctx context.Context, exe string, args ...string) []string {
	to, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	b, err := exec.CommandContext(to, exe, args...).Output()
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func collectInstances(ctx context.Context, r *root.Root) []InstanceConfig {
	ents, _ := os.ReadDir(r.AvdHome())
	var out []InstanceConfig
	port := r.AdbServerPort()
	for _, ent := range ents {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".ini") {
			continue
		}
		name := strings.TrimSuffix(ent.Name(), ".ini")
		if !strings.HasPrefix(name, root.AvdNamePrefix) {
			continue
		}
		i, err := strconv.Atoi(strings.TrimPrefix(name, root.AvdNamePrefix))
		if err != nil || !root.ValidIndex(i) {
			continue
		}
		in := InstanceConfig{Index: i, Name: name}
		if b, err := os.ReadFile(filepath.Join(r.AvdHome(), name+".avd", "config.ini")); err == nil {
			props := parseProperties(b)
			if v := props["hw.ramSize"]; v != "" {
				if n, e := strconv.Atoi(v); e == nil {
					in.RAM = fmt.Sprintf("，%d GB", n/1024)
				}
			}
		}
		serial := root.Serial(i)
		if games, ok := queryGames(ctx, port, serial); ok {
			in.Games = games
		}
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

func parseProperties(b []byte) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if i := strings.IndexByte(line, '='); i > 0 && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			m[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
	}
	return m
}

func adbRequest(ctx context.Context, port int, service string) ([]byte, error) {
	if port == 5037 || port < 1 || port > 65535 {
		return nil, errors.New("拒绝使用 5037")
	}
	d := net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	payload := []byte(service)
	if _, err := fmt.Fprintf(c, "%04x", len(payload)); err != nil {
		return nil, err
	}
	if _, err := c.Write(payload); err != nil {
		return nil, err
	}
	status := make([]byte, 4)
	if _, err := io.ReadFull(c, status); err != nil {
		return nil, err
	}
	if string(status) == "FAIL" {
		var h [4]byte
		if _, err := io.ReadFull(c, h[:]); err != nil {
			return nil, err
		}
		n, _ := strconv.ParseInt(string(h[:]), 16, 32)
		b := make([]byte, n)
		_, _ = io.ReadFull(c, b)
		return nil, errors.New(string(b))
	}
	if string(status) != "OKAY" {
		return nil, fmt.Errorf("adb server 返回 %q", status)
	}
	var h [4]byte
	if _, err := io.ReadFull(c, h[:]); err != nil {
		return nil, err
	}
	n, err := strconv.ParseInt(string(h[:]), 16, 32)
	if err != nil || n < 0 || n > 64<<20 {
		return nil, errors.New("adb 返回长度不对")
	}
	b := make([]byte, n)
	_, err = io.ReadFull(c, b)
	return b, err
}

func queryGames(ctx context.Context, port int, serial string) ([]string, bool) {
	devices, err := adbRequest(ctx, port, "host:devices")
	if err != nil || !strings.Contains(string(devices), serial+"\tdevice") {
		return nil, false
	}
	out, err := adbShell(ctx, port, serial, "pm list packages -3")
	if err != nil {
		return nil, true
	}
	known := map[string]string{
		"com.hypergryph.arknights":          "明日方舟（官服）",
		"com.hypergryph.arknights.bilibili": "明日方舟（B 服）",
		"com.hypergryph.endfield":           "明日方舟：终末地",
		"com.shenlan.m.reverse1999":         "重返未来：1999",
		"com.miHoYo.hkrpg":                  "崩坏：星穹铁道（官服）",
		"com.miHoYo.hkrpg.bilibili":         "崩坏：星穹铁道（B 服）",
		"com.miHoYo.enterprise.NGHSoD":      "崩坏3（官服）",
	}
	var games []string
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		pkg := strings.TrimSpace(strings.TrimPrefix(line, "package:"))
		if pkg == "" || pkg == "de.jrpie.android.launcher" {
			continue
		}
		label := pkg
		if name, ok := known[pkg]; ok {
			label = name
		}
		if version, e := adbShell(ctx, port, serial, "dumpsys package "+pkg+" 2>/dev/null | grep -m1 versionName="); e == nil {
			for _, v := range strings.Split(version, "\n") {
				if strings.HasPrefix(strings.TrimSpace(v), "versionName=") {
					if s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "versionName=")); s != "" {
						label += " " + s
					}
					break
				}
			}
		}
		games = append(games, label)
	}
	sort.Strings(games)
	return games, true
}

func adbShell(ctx context.Context, port int, serial, command string) (string, error) {
	d := net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	send := func(s string) error {
		b := []byte(s)
		if _, err := fmt.Fprintf(c, "%04x", len(b)); err != nil {
			return err
		}
		_, err := c.Write(b)
		return err
	}
	readStatus := func() error {
		var st [4]byte
		if _, err := io.ReadFull(c, st[:]); err != nil {
			return err
		}
		if string(st[:]) != "OKAY" {
			return fmt.Errorf("adb 状态 %q", st[:])
		}
		return nil
	}
	if err := send("host:transport:" + serial); err != nil {
		return "", err
	}
	if err := readStatus(); err != nil {
		return "", err
	}
	if err := send("shell:" + command); err != nil {
		return "", err
	}
	if err := readStatus(); err != nil {
		return "", err
	}
	var b bytes.Buffer
	_, err = io.Copy(&b, c)
	return strings.TrimSpace(b.String()), err
}

type CollectOptions struct {
	OutDir     string
	StageDir   string
	KeepStage  bool
	NoExplorer bool
}

type CollectResult struct {
	ZipPath string   `json:"zip_path"`
	Stage   string   `json:"stage"`
	Summary string   `json:"summary"`
	Entries []string `json:"entries"`
	Errors  []string `json:"errors"`
}

// Collect builds a diagnostics zip.  Every section is isolated so one missing
// WMI/log file does not prevent the remaining evidence from being collected.
func Collect(ctx context.Context, r *root.Root, opt CollectOptions) (*CollectResult, error) {
	if runtime.GOOS != "windows" {
		return nil, errs.New(errs.Unsupported, "收集问题信息目前只支持 Windows")
	}
	if opt.OutDir == "" {
		opt.OutDir = desktopDir()
	}
	if opt.StageDir == "" {
		opt.StageDir = os.TempDir()
	}
	if err := os.MkdirAll(opt.OutDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opt.StageDir, 0o755); err != nil {
		return nil, err
	}
	stamp := time.Now().Format("20060102-150405")
	stage, err := os.MkdirTemp(opt.StageDir, "modavd-collect-"+stamp+"-")
	if err != nil {
		return nil, err
	}
	res := &CollectResult{Stage: stage}
	defer func() {
		if !opt.KeepStage {
			_ = os.RemoveAll(stage)
		}
	}()
	var summary bytes.Buffer
	summary.WriteString("魔改 AVD 问题信息\r\n")
	summary.WriteString(privacyNote + "\r\n")
	summary.WriteString("生成时间：" + time.Now().Format("2006-01-02 15:04:05 -0700") + "\r\n")
	section := func(name string, fn func() error) {
		summary.WriteString("\r\n== " + name + " ==\r\n")
		if e := fn(); e != nil {
			msg := name + "：" + e.Error()
			summary.WriteString("[本节出错] " + msg + "\r\n")
			res.Errors = append(res.Errors, msg)
		}
	}
	section("ModAVD 目录与版本", func() error {
		summary.WriteString("路径：" + r.Dir + "\r\n")
		summary.WriteString("SDK：" + r.RuntimeSDK() + "\r\n")
		if err := copyFile(r.MetadataPath(), filepath.Join(stage, "mas-avd.json")); err != nil {
			summary.WriteString("没有 mas-avd.json\r\n")
		}
		return nil
	})
	section("实例列表与实例文件", func() error { return collectInstanceFiles(r, stage, &summary) })
	section("模拟器日志", func() error { return collectLogs(r, stage, &summary) })
	section("模拟器崩溃转储", func() error { return collectCrashes(stage, &summary) })
	section("AUTO-MAS 日志", func() error { return collectMASLogs(stage, &summary) })
	section("现场截图与状态", func() error { summary.WriteString("没有实例在运行，跳过\r\n"); return nil })
	summary.WriteString("\r\n== 收集过程 ==\r\n")
	if len(res.Errors) == 0 {
		summary.WriteString("各节都没出错。\r\n")
	} else {
		summary.WriteString("出错的节：\r\n")
		for _, e := range res.Errors {
			summary.WriteString("  " + e + "\r\n")
		}
	}
	res.Summary = summary.String()
	summaryBytes := append([]byte{0xEF, 0xBB, 0xBF}, []byte(res.Summary)...)
	if err := os.WriteFile(filepath.Join(stage, "summary.txt"), summaryBytes, 0o644); err != nil {
		return nil, err
	}
	name := "魔改AVD问题信息-" + stamp + ".zip"
	zipPath := filepath.Join(opt.OutDir, name)
	if _, err := os.Stat(zipPath); err == nil {
		zipPath = filepath.Join(opt.OutDir, "魔改AVD问题信息-"+stamp+"-"+strconv.Itoa(os.Getpid())+".zip")
	}
	entries, err := zipStage(zipPath, stage)
	if err != nil {
		return nil, err
	}
	res.ZipPath, res.Entries = zipPath, entries
	if !opt.NoExplorer {
		_ = openExplorer(zipPath)
	}
	return res, nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

func collectInstanceFiles(r *root.Root, stage string, summary *bytes.Buffer) error {
	ents, err := os.ReadDir(r.AvdHome())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			summary.WriteString("还没有实例。\r\n")
			return nil
		}
		return err
	}
	count := 0
	for _, ent := range ents {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".ini") {
			continue
		}
		count++
		name := strings.TrimSuffix(ent.Name(), ".ini")
		if !strings.HasPrefix(name, root.AvdNamePrefix) {
			continue
		}
		if err := copyFile(filepath.Join(r.AvdHome(), ent.Name()), filepath.Join(stage, "avd", ent.Name())); err != nil {
			summary.WriteString(name + "：复制 ini 失败：" + err.Error() + "\r\n")
			continue
		}
		dir := filepath.Join(r.AvdHome(), name+".avd")
		for _, fn := range []string{"config.ini", "hardware-qemu.ini", "emulator-user.ini"} {
			if err := copyFile(filepath.Join(dir, fn), filepath.Join(stage, "avd", name+".avd", fn)); err == nil {
				summary.WriteString(name + "：已收集 " + fn + "\r\n")
			}
		}
		var listing []string
		if files, e := os.ReadDir(dir); e == nil {
			for _, file := range files {
				if file.IsDir() {
					listing = append(listing, fmt.Sprintf("%-40s <DIR>", file.Name()))
				} else if info, e := file.Info(); e == nil {
					listing = append(listing, fmt.Sprintf("%-40s %16d  %s", file.Name(), info.Size(), info.ModTime().Format("2006-01-02 15:04:05")))
				}
			}
		}
		if len(listing) == 0 {
			listing = []string{"（空目录）"}
		}
		_ = os.MkdirAll(filepath.Join(stage, "avd", name+".avd"), 0o755)
		_ = os.WriteFile(filepath.Join(stage, "avd", name+".avd", "_files.txt"), []byte(strings.Join(listing, "\r\n")+"\r\n"), 0o644)
	}
	if count == 0 {
		summary.WriteString("还没有实例（avd 目录不存在或里面没有 *.ini）。\r\n")
	}
	return nil
}

func collectLogs(r *root.Root, stage string, summary *bytes.Buffer) error {
	dir := r.LogsDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			summary.WriteString("没有 logs 目录\r\n")
			return nil
		}
		return err
	}
	count := 0
	for _, ent := range ents {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".log") && !strings.HasSuffix(ent.Name(), ".txt") {
			continue
		}
		if err := copyFile(filepath.Join(dir, ent.Name()), filepath.Join(stage, "logs", ent.Name())); err == nil {
			count++
		}
	}
	summary.WriteString(fmt.Sprintf("共复制 %d 份日志。\r\n", count))
	return nil
}

func collectCrashes(stage string, summary *bytes.Buffer) error {
	crashDir := regexp.MustCompile(`^emu-crash-[0-9]+\.[0-9]+\.[0-9]+-mas\.db$`)
	since := time.Now().Add(-7 * 24 * time.Hour)
	seen := map[string]bool{}
	var candidates []string
	for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("TEMP")} {
		if base == "" {
			continue
		}
		dir := filepath.Join(base, "Temp", "AndroidEmulator")
		if base == os.Getenv("TEMP") {
			dir = filepath.Join(base, "AndroidEmulator")
		}
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".dmp") {
				return nil
			}
			parent := filepath.Base(filepath.Dir(path))
			if !crashDir.MatchString(parent) || info.ModTime().Before(since) {
				return nil
			}
			if !seen[path] {
				seen[path] = true
				candidates = append(candidates, path)
			}
			return nil
		})
	}
	if len(candidates) == 0 {
		summary.WriteString("近 7 天没有\r\n")
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] > candidates[j] })
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	for _, src := range candidates {
		base := filepath.Base(filepath.Dir(src))
		dst := filepath.Join(stage, "crash", base, filepath.Base(src))
		if info, err := os.Stat(src); err == nil && info.Size() <= 100<<20 {
			_ = copyFile(src, dst)
			summary.WriteString(src + "  " + formatBytes(info.Size()) + "\r\n")
		}
	}
	return nil
}

func collectMASLogs(stage string, summary *bytes.Buffer) error {
	for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("APPDATA")} {
		if base == "" {
			continue
		}
		for _, dir := range []string{filepath.Join(base, "AUTO-MAS", "debug"), filepath.Join(base, "AUTO-MAS", "logs")} {
			for _, name := range []string{"app.log", "frontend.log"} {
				if err := copyFile(filepath.Join(dir, name), filepath.Join(stage, "mas", name)); err == nil {
					summary.WriteString(name + "：已收集\r\n")
				}
			}
		}
	}
	return nil
}

func formatBytes(n int64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	}
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	if n >= 1<<10 {
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func zipStage(dst, stage string) ([]string, error) {
	f, err := os.Create(dst)
	if err != nil {
		return nil, err
	}
	zw := zip.NewWriter(f)
	var names []string
	var files []struct{ path, name string }
	err = filepath.Walk(stage, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(stage, path)
		rel = filepath.ToSlash(rel)
		if rel == "" || forbiddenEntry(rel) {
			return nil
		}
		files = append(files, struct{ path, name string }{path, rel})
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		if files[i].name == "summary.txt" {
			return true
		}
		if files[j].name == "summary.txt" {
			return false
		}
		return files[i].name < files[j].name
	})
	if err == nil {
		for _, file := range files {
			in, e := os.Open(file.path)
			if e != nil {
				err = e
				break
			}
			h, e := zw.Create(file.name)
			if e == nil {
				_, e = io.Copy(h, in)
			}
			_ = in.Close()
			if e != nil {
				err = e
				break
			}
			names = append(names, file.name)
		}
	}
	if e := zw.Close(); err == nil {
		err = e
	}
	if e := f.Close(); err == nil {
		err = e
	}
	return names, err
}

func forbiddenEntry(name string) bool {
	for _, p := range strings.Split(strings.ToLower(name), "/") {
		if p == "config" || p == "data" || p == "snapshots" || p == "userdata" {
			return true
		}
	}
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".img") || strings.HasSuffix(l, ".qcow2")
}

func desktopDir() string {
	if v := os.Getenv("USERPROFILE"); v != "" {
		return filepath.Join(v, "Desktop")
	}
	return os.TempDir()
}
