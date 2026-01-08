package main

import "embed"

//go:embed static/*
var staticFiles embed.FS

//go:embed enhance_prompt.txt
var embeddedEnhancePrompt string
