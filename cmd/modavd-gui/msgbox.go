package main

import "modavd/internal/host"

func fatal(msg string) { _ = host.MessageBox(msg) }
