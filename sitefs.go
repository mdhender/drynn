// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package drynn

import (
	"embed"
	"io/fs"
)

//go:embed web/sitepublic
var sitePublic embed.FS

func SiteFS() fs.FS {
	sub, _ := fs.Sub(sitePublic, "web/sitepublic")
	return sub
}
