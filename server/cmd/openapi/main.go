package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/api"
)

func main() {
	spec, err := api.New(http.NewServeMux(), nil, config.Config{}, nil).OpenAPI().MarshalJSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(spec))
}
