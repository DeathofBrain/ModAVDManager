package main

import "modavd/internal/win"

func fatal(msg string) { _ = win.MessageBox(msg) }
