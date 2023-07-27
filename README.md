[![Go Reference](https://pkg.go.dev/badge/github.com/KarpelesLab/retlsfetch.svg)](https://pkg.go.dev/github.com/KarpelesLab/retlsfetch)

# retlsfetch

Simple library to (re)fetch TLS secured pages in a way that the data can be re-certified afterward.

This works by saving the raw encrypted bytes, random generator data and other stuff into a file in order to be able to reproduce the exchange at any future time.

Because the TLS data is encrypted and secured against MITM attacks, it means in theory that saving the stream on disk can be used to certify the data in question indeed came from the server it claims to be coming from.


