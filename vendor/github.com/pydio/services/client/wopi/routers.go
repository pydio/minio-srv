package wopi

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)
		handler = Auth(handler)

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return router
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Not accessible")
}

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/wopi/",
		Index,
	},

	Route{
		"Download",
		"GET",
		"/wopi/files/{uuid}/contents",
		Download,
	},

	Route{
		"GetNodeInfos",
		"GET",
		"/wopi/files/{uuid}",
		GetNodeInfos,
	},

	Route{
		"UploadStream",
		"POST",
		"/wopi/files/{uuid}/contents",
		UploadStream,
	},
}
