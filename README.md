# image-check [![builds.sr.ht status](https://builds.sr.ht/~delthas/image-check.svg)](https://builds.sr.ht/~delthas/image-check?)

A small tool to efficiently check whether an image file is **not truncated**.

*This tool **assumes that the file is not corrupted, only truncated**, i.e. that all bytes of the file are valid.*

Usage:
`image-check image.jpg`

Exits with 0 if the file is not truncated, otherwise exits with a code > 0 and an error message on stderr.

## Binaries

| OS | URL |
|---|---|
| Linux x64 | https://delthas.fr/image-check/linux/image-check |
| Mac OS X x64 | https://delthas.fr/image-check/mac/image-check |
| Windows x64 | https://delthas.fr/image-check/windows/image-check.exe |

You can also build it with: `go get github.com/delthas/image-check/cmd/image-check`

## Image file type support

| Type | Supported? |
| --- | --- |
| JPEG | ✔ |
| PNG | ✔ |
| GIF | ✔ |
| SWF | ✔ |
