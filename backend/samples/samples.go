// Package samples 随二进制分发原创示例资料。
package samples

import "embed"

//go:embed */manifest.json */*.md
var Content embed.FS
