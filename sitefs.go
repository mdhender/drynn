// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package drynn

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed web/sitepublic
var sitePublic embed.FS

func SiteFS() fs.FS {
	sub, err := fs.Sub(sitePublic, "web/sitepublic")
	if err != nil {
		panic(fmt.Sprintf("sitefs: embedded sub-filesystem: %v", err))
	}
	return sub
}
