// Package components downloads the Google components which are intentionally
// absent from a ModAVD release package.
package components

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"modavd/internal/errs"
	"modavd/internal/root"
)

type Mirror struct {
	ID, Name, Base string
}

var defaultMirrors = []Mirror{
	{ID: "googledownloads_cn", Name: "Google 中国下载（googledownloads.cn）", Base: "https://googledownloads.cn/android/repository/"},
	{ID: "tencent", Name: "腾讯云镜像（mirrors.cloud.tencent.com）", Base: "https://mirrors.cloud.tencent.com/AndroidSDK/"},
	{ID: "google", Name: "Google 官方（dl.google.com）", Base: "https://dl.google.com/android/repository/"},
}

var fetchGOOS = runtime.GOOS
var componentCatalogue = root.WindowsFetchComponents

type Options struct {
	Root            *root.Root
	AcceptLicense   bool
	JSON            bool
	Input           io.Reader
	KeepDownloads   bool
	Mirror          string
	MirrorURLs      []string
	ConnectTimeout  time.Duration
	StallTimeout    time.Duration
	ManifestTimeout time.Duration
	TestFreeSpaceMB int64
	Progress        func(string)
	License         func(string)
}

type ComponentResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Installed  bool   `json:"installed"`
	Downloaded bool   `json:"downloaded"`
	Skipped    bool   `json:"skipped"`
}

type Result struct {
	Root          string            `json:"root"`
	License       string            `json:"license"`
	LicenseSource string            `json:"license_source"`
	Components    []ComponentResult `json:"components"`
	Downloads     []string          `json:"downloads"`
}

type licenseDoc struct {
	Licenses []struct {
		ID   string `xml:"id,attr"`
		Text string `xml:",chardata"`
	} `xml:"license"`
}

type fetcher struct {
	r       *root.Root
	o       Options
	log     func(string)
	client  *http.Client
	mirrors []Mirror
}

func Fetch(ctx context.Context, r *root.Root, o Options) (*Result, error) {
	if fetchGOOS != "windows" {
		return nil, errs.New(errs.Unsupported, "这个系统还没有组件清单")
	}
	if r == nil {
		return nil, errs.New(errs.NotFound, "找不到魔改 AVD 根目录")
	}
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = 20 * time.Second
	}
	if o.StallTimeout <= 0 {
		o.StallTimeout = 30 * time.Second
	}
	if o.ManifestTimeout <= 0 {
		o.ManifestTimeout = 40 * time.Second
	}
	f := &fetcher{r: r, o: o}
	f.log = func(msg string) {
		if o.Progress != nil {
			o.Progress(msg)
		}
		logDir := filepath.Join(r.Dir, "logs")
		if err := os.MkdirAll(logDir, 0o755); err == nil {
			p := filepath.Join(logDir, "fetch-components.log")
			f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err == nil {
				_, _ = fmt.Fprintf(f, "%s %s\r\n", time.Now().Format("2006-01-02 15:04:05"), msg)
				_ = f.Close()
			}
		}
	}
	f.log("开始补全 Google 组件")
	var mirrorErr error
	f.mirrors, mirrorErr = chooseMirrors(o.Mirror, o.MirrorURLs)
	if mirrorErr != nil {
		return nil, errs.Wrap(errs.InvalidArgument, mirrorErr, mirrorErr.Error())
	}
	if len(f.mirrors) == 0 {
		return nil, errs.New(errs.InvalidArgument, "没有可用的镜像")
	}
	if strings.IndexFunc(r.Dir, func(r rune) bool { return r > 0x7f }) >= 0 {
		return nil, errs.New(errs.NotReady, "这个文件夹的路径里有中文或其他非英文字符，模拟器在这种路径下起不来")
	}
	if _, err := os.Stat(filepath.Join(r.SDK(), "emulator", "emulator.exe")); err != nil {
		return nil, errs.Newf(errs.NotReady, "这里没有 sdk\\emulator\\emulator.exe，不像是魔改 AVD 的文件夹：%s", r.Dir)
	}
	specs := componentCatalogue()
	result := &Result{Root: r.Dir, Components: []ComponentResult{}, Downloads: []string{}}
	_ = os.RemoveAll(filepath.Join(r.SDK(), ".staging"))
	var todo []root.FetchComponentSpec
	for _, c := range specs {
		cr := ComponentResult{ID: c.ID, Name: c.Name, Version: c.Version}
		if installed(r, c) {
			cr.Installed, cr.Skipped = true, true
			result.Components = append(result.Components, cr)
			f.log("已有：" + c.Name + "（版本 " + c.Version + "），跳过")
		} else {
			result.Components = append(result.Components, cr)
			todo = append(todo, c)
		}
	}
	if len(todo) == 0 {
		return result, nil
	}
	if err := f.checkSpace(todo); err != nil {
		return nil, err
	}
	license, source, err := f.license(ctx)
	if err != nil {
		return nil, err
	}
	result.License, result.LicenseSource = license, source
	if o.License != nil {
		o.License(license)
	} else {
		f.log(license)
	}
	if o.JSON && !o.AcceptLicense {
		return nil, errs.New(errs.Usage, "--json 模式必须同时给 --accept-license，不能进入交互")
	}
	if !o.AcceptLicense {
		f.log("以上是 Google《Android 软件开发套件许可协议》全文。请输入 y 并回车表示同意：")
		in := bufio.NewReader(o.Input)
		if o.Input == nil {
			in = bufio.NewReader(os.Stdin)
		}
		line, _ := in.ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(line), "y") && !strings.EqualFold(strings.TrimSpace(line), "yes") {
			return nil, errs.New(errs.Usage, "没有同意许可协议，已退出，什么也没有下载")
		}
	}
	if err := os.MkdirAll(filepath.Join(r.Dir, "downloads"), 0o755); err != nil {
		return nil, errs.Wrap(errs.Internal, err, "建不了 downloads 目录")
	}
	licensePath := filepath.Join(r.Dir, "downloads", "android-sdk-license.txt")
	if err := os.WriteFile(licensePath, []byte(strings.ReplaceAll(license, "\n", "\r\n")+"\r\n"), 0o644); err != nil {
		return nil, errs.Wrap(errs.Internal, err, "保存许可协议失败")
	}
	for _, c := range todo {
		f.log(fmt.Sprintf("—— %s（%d）", c.Name, c.Size))
		zipPath, err := f.download(ctx, c)
		if err != nil {
			return nil, err
		}
		if err := f.install(c, zipPath); err != nil {
			return nil, err
		}
		for j := range result.Components {
			if result.Components[j].ID == c.ID {
				result.Components[j] = ComponentResult{ID: c.ID, Name: c.Name, Version: c.Version, Installed: true, Downloaded: true}
				break
			}
		}
		result.Downloads = append(result.Downloads, filepath.Base(zipPath))
		if !o.KeepDownloads {
			_ = os.Remove(zipPath)
		}
	}
	return result, nil
}

func chooseMirrors(one string, urls []string) ([]Mirror, error) {
	if len(urls) > 0 {
		var out []Mirror
		for _, raw := range urls {
			for _, u := range strings.Split(raw, ",") {
				u = strings.TrimSpace(u)
				if u == "" {
					continue
				}
				if !strings.HasSuffix(u, "/") {
					u += "/"
				}
				out = append(out, Mirror{ID: "custom", Name: "自定义镜像（" + u + "）", Base: u})
			}
		}
		return out, nil
	}
	if one == "" {
		return append([]Mirror(nil), defaultMirrors...), nil
	}
	for _, m := range defaultMirrors {
		if m.ID == one {
			return []Mirror{m}, nil
		}
	}
	return nil, fmt.Errorf("未知镜像 %s", one)
}

func (f *fetcher) httpClient() *http.Client {
	if f.client != nil {
		return f.client
	}
	dialer := &net.Dialer{Timeout: f.o.ConnectTimeout}
	tr := &http.Transport{DialContext: dialer.DialContext, ResponseHeaderTimeout: f.o.ConnectTimeout, DisableCompression: true}
	f.client = &http.Client{Transport: tr, CheckRedirect: func(req *http.Request, via []*http.Request) error { return nil }}
	return f.client
}

func (f *fetcher) request(ctx context.Context, method, url string, timeout time.Duration, offset int64) (*http.Response, error) {
	_ = timeout // connection and response-header limits are enforced by the transport
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "ModAVD-components/1.0")
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	resp, err := f.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if ce := resp.Header.Get("Content-Encoding"); ce != "" && !strings.EqualFold(ce, "identity") {
		resp.Body.Close()
		return nil, fmt.Errorf("服务器返回了压缩过的内容（Content-Encoding: %s）", ce)
	}
	return resp, nil
}

func readStall(ctx context.Context, body io.Reader, stall time.Duration) ([]byte, error) {
	buf := make([]byte, 1<<20)
	type rr struct {
		n   int
		err error
	}
	ch := make(chan rr, 1)
	go func() { n, err := body.Read(buf); ch <- rr{n, err} }()
	t := time.NewTimer(stall)
	select {
	case <-ctx.Done():
		t.Stop()
		return nil, ctx.Err()
	case <-t.C:
		return nil, fmt.Errorf("下载卡住超过 %s", stall)
	case r := <-ch:
		t.Stop()
		if r.n > 0 {
			return append([]byte(nil), buf[:r.n]...), r.err
		}
		return nil, r.err
	}
}

func (f *fetcher) license(ctx context.Context) (string, string, error) {
	var errsSeen []string
	for _, m := range f.mirrors {
		licenseCtx, cancel := context.WithTimeout(ctx, f.o.ManifestTimeout)
		resp, err := f.request(licenseCtx, http.MethodGet, m.Base+"repository2-3.xml", f.o.ManifestTimeout, 0)
		if err != nil {
			cancel()
			errsSeen = append(errsSeen, m.Name+"："+err.Error())
			continue
		}
		var b []byte
		for {
			chunk, readErr := readStall(licenseCtx, resp.Body, f.o.StallTimeout)
			if len(chunk) > 0 {
				b = append(b, chunk...)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				err = readErr
				break
			}
		}
		resp.Body.Close()
		cancel()
		if err != nil {
			errsSeen = append(errsSeen, m.Name+"："+err.Error())
			continue
		}
		var doc licenseDoc
		if err := xml.Unmarshal(b, &doc); err != nil {
			errsSeen = append(errsSeen, m.Name+"："+err.Error())
			continue
		}
		for _, l := range doc.Licenses {
			if l.ID == "android-sdk-license" && strings.TrimSpace(l.Text) != "" {
				return strings.TrimSpace(l.Text), m.Name, nil
			}
		}
		errsSeen = append(errsSeen, m.Name+"：清单里没有 android-sdk-license")
	}
	return "", "", errs.Wrap(errs.UpdateUnavailable, fmt.Errorf("%s", strings.Join(errsSeen, "；")), "取不到《Android 软件开发套件许可协议》全文，什么也没有下载")
}

func (f *fetcher) checkSpace(todo []root.FetchComponentSpec) error {
	need := int64(500 << 20)
	for _, c := range todo {
		zip := filepath.Join(f.r.Dir, "downloads", filepath.Base(c.URL))
		have := int64(0)
		if st, err := os.Stat(zip); err == nil {
			have = st.Size()
		} else if st, err := os.Stat(zip + ".part"); err == nil {
			have = st.Size()
		}
		if left := c.Size - have; left > 0 {
			need += left
		}
		need += c.ExtractedBytes
	}
	free := int64(0)
	if f.o.TestFreeSpaceMB >= 0 {
		free = f.o.TestFreeSpaceMB << 20
	} else if st, err := os.Stat(f.r.Dir); err == nil {
		_ = st
		free = diskFree(f.r.Dir)
	}
	if free > 0 && free < need {
		return errs.Newf(errs.NotReady, "磁盘空间不够：还需要约 %d MB，当前可用约 %d MB", need>>20, free>>20)
	}
	return nil
}

func installed(r *root.Root, c root.FetchComponentSpec) bool {
	dir := filepath.Join(r.SDK(), c.Target)
	for _, name := range c.KeyFiles {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
			return false
		}
	}
	props, err := os.ReadFile(filepath.Join(dir, "source.properties"))
	if err != nil {
		return false
	}
	return strings.Contains(string(props), "Pkg.Revision="+c.Version)
}

func (f *fetcher) download(ctx context.Context, c root.FetchComponentSpec) (string, error) {
	dir := filepath.Join(f.r.Dir, "downloads")
	_ = os.MkdirAll(dir, 0o755)
	zipPath := filepath.Join(dir, filepath.Base(c.URL))
	part := zipPath + ".part"
	if st, err := os.Stat(zipPath); err == nil && st.Size() == c.Size {
		if sha1File(zipPath) == c.SHA1 {
			return zipPath, nil
		}
		_ = os.Remove(zipPath)
	}
	var failures []string
	for _, m := range f.mirrors {
		for attempt := 0; attempt < 3; attempt++ {
			if err := f.downloadOne(ctx, m.Base+c.URL, part, c.Size); err != nil {
				failures = append(failures, m.Name+"："+err.Error())
				continue
			}
			if sha := sha1File(part); sha != c.SHA1 {
				_ = os.Remove(part)
				failures = append(failures, m.Name+"：SHA-1 "+sha)
				break
			}
			if err := os.Rename(part, zipPath); err != nil {
				return "", errs.Wrap(errs.Internal, err, "保存下载文件失败")
			}
			return zipPath, nil
		}
	}
	return "", errs.Wrap(errs.UpdateRejected, fmt.Errorf("%s", strings.Join(failures, "；")), "所有镜像都没下成或校验不通过")
}

func (f *fetcher) downloadOne(ctx context.Context, url, part string, expected int64) error {
	existing := int64(0)
	if st, err := os.Stat(part); err == nil {
		existing = st.Size()
		if existing > expected {
			_ = os.Remove(part)
			existing = 0
		}
	}
	resp, err := f.request(ctx, http.MethodGet, url, f.o.ConnectTimeout+f.o.StallTimeout+time.Hour, existing)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	mode := os.O_CREATE | os.O_WRONLY
	start := existing
	if existing > 0 && resp.StatusCode == http.StatusPartialContent && strings.HasPrefix(resp.Header.Get("Content-Range"), "bytes "+strconv.FormatInt(existing, 10)+"-") {
		mode |= os.O_APPEND
	} else {
		mode |= os.O_TRUNC
		start = 0
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("服务器返回 HTTP %d", resp.StatusCode)
	}
	fh, err := os.OpenFile(part, mode, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()
	done := start
	for {
		b, readErr := readStall(ctx2, resp.Body, f.o.StallTimeout)
		if len(b) > 0 {
			if _, err := fh.Write(b); err != nil {
				return err
			}
			done += int64(len(b))
			f.log(fmt.Sprintf("下载 %s：%d / %d", filepath.Base(part), done, expected))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
		if len(b) == 0 {
			break
		}
		if done >= expected {
			break
		}
	}
	if done != expected {
		return fmt.Errorf("下载不完整：应为 %d 字节，实际 %d 字节", expected, done)
	}
	return fh.Sync()
}

func sha1File(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha1.New()
	_, _ = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}
func sha256File(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	_, _ = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func (f *fetcher) install(c root.FetchComponentSpec, zipPath string) error {
	stageRoot := filepath.Join(f.r.SDK(), ".staging", c.ID)
	_ = os.RemoveAll(stageRoot)
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(stageRoot)
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return errs.Wrap(errs.UpdateRejected, err, "压缩包打不开")
	}
	defer r.Close()
	prefix := strings.Trim(c.ArchiveRoot, "/") + "/"
	rootDir := filepath.Clean(stageRoot)
	for _, e := range r.File {
		name := strings.ReplaceAll(e.Name, "\\", "/")
		if !strings.HasPrefix(name, prefix) {
			return errs.Newf(errs.UpdateRejected, "压缩包结构不对：条目 %s 不在 %s 下", e.Name, prefix)
		}
		rel := name
		if rel == prefix {
			continue
		}
		dest := filepath.Join(stageRoot, filepath.FromSlash(rel))
		clean, _ := filepath.Abs(dest)
		base, _ := filepath.Abs(rootDir)
		if clean != base && !strings.HasPrefix(clean, base+string(os.PathSeparator)) {
			return errs.Newf(errs.UpdateRejected, "压缩包里有非法路径：%s", e.Name)
		}
		if e.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		in, err := e.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			in.Close()
			return err
		}
		_, cpErr := io.Copy(out, in)
		_ = in.Close()
		closeErr := out.Close()
		if cpErr != nil {
			return cpErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	extracted := filepath.Join(stageRoot, filepath.FromSlash(c.ArchiveRoot))
	if err := validateExtracted(c, extracted); err != nil {
		return err
	}
	target := filepath.Join(f.r.SDK(), c.Target)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	backup := target + ".components-old"
	_ = os.RemoveAll(backup)
	backedUp := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
		backedUp = true
		defer os.RemoveAll(backup)
	}
	if err := os.Rename(extracted, target); err != nil {
		if backedUp {
			_ = os.Rename(backup, target)
		}
		return err
	}
	if !installed(f.r, c) {
		_ = os.RemoveAll(target)
		if backedUp {
			_ = os.Rename(backup, target)
		}
		return errs.Newf(errs.UpdateRejected, "装完后 %s 仍缺关键文件", c.Name)
	}
	return nil
}

func validateExtracted(c root.FetchComponentSpec, dir string) error {
	for _, n := range c.KeyFiles {
		st, err := os.Stat(filepath.Join(dir, n))
		if err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
			return fmt.Errorf("压缩包里缺少 %s", n)
		}
	}
	props := filepath.Join(dir, "source.properties")
	if _, err := os.Stat(props); os.IsNotExist(err) {
		if err := os.WriteFile(props, []byte(c.Properties), 0o644); err != nil {
			return err
		}
	}
	data, _ := os.ReadFile(props)
	if !strings.Contains(string(data), "Pkg.Revision="+c.Version) {
		return fmt.Errorf("source.properties 版本不对")
	}
	for name, want := range c.KeyHashes {
		if got := sha256File(filepath.Join(dir, name)); !strings.EqualFold(got, want) {
			return fmt.Errorf("关键文件 %s 的 SHA-256 不对", name)
		}
	}
	return nil
}
