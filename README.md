
## Dependencies for golang

The below will only work for Viam, Inc. employees right now. The C++ code is independent however.

Make sure the following is in your shell configuration:
```
export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*
```

Also run 
```
git config --global url.ssh://git@github.com/.insteadOf https://github.com/
```

# Getting started

Currently only tested on an RPI:
1. `make`
2. Make sure you run the commands above
3. `go run -tags=pi cmd/server/main.go `