// SPDX-License-Identifier: Apache-2.0

/*
Package core contains some basic constants and variables
*/
package core

import (
	"fmt"
	"runtime"
	"strings"
)

// PucoraHeaderName is the name of the custom Pucora response header.
const PucoraHeaderName = "X-PUCORA"

// PucoraVersion is the version of the build.
var PucoraVersion = "undefined"

// GoVersion is the version of the go compiler used at build time
var GoVersion = strings.TrimPrefix(runtime.Version(), "go")

// GlibcVersion is the version of the glibc used by CGO at build time
var GlibcVersion = "undefined"

// PucoraHeaderValue is the value of the custom Pucora response header.
var PucoraHeaderValue = fmt.Sprintf("Version %s", PucoraVersion)

// PucoraUserAgent is the value of the user agent header sent to the backends.
var PucoraUserAgent = fmt.Sprintf("Pucora Version %s", PucoraVersion)
