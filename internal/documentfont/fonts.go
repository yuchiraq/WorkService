// Package documentfont loads local Times New Roman for PDF output. The embedded
// OFL fonts remain for portable renderer tests and old cached browser assets.
package documentfont

import _ "embed"

//go:embed LiberationSerif-Regular.ttf
var Regular []byte

//go:embed LiberationSerif-Bold.ttf
var Bold []byte

//go:embed LiberationSerif-Italic.ttf
var Italic []byte

func Lookup(name string) []byte {
	switch name {
	case "serif-regular.ttf":
		return Regular
	case "serif-bold.ttf":
		return Bold
	case "serif-italic.ttf":
		return Italic
	}
	return nil
}
