package main

import (
	"io/fs"

	webcapdata "github.com/goliatone/webcap/data"
)

func webcapAgentSkillSource() fs.FS {
	return webcapdata.WebcapAgent
}
