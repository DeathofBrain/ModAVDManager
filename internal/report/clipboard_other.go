//go:build !windows

package report

import "errors"

func copyClipboard(string) error { return errors.New("当前平台没有剪贴板实现") }
func openExplorer(string) error  { return errors.New("当前平台没有资源管理器") }
