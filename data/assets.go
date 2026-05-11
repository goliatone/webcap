package data

import "embed"

// WebcapAgent contains the bundled webcap-agent skill assets.
//
//go:embed webcap-agent
var WebcapAgent embed.FS
