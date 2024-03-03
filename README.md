statics
====

静的ファイルを配信する http.Handler

## Usage
```
$ go get github.com/thamaji/statics
```

```go
package main

import (
	"net/http"

	"github.com/thamaji/statics"
)

func main() {
	http.ListenAndServe(":8089", statics.FileServer("./dist"))
}
```
