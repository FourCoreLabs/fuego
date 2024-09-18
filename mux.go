package fuego

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

type GroupOption struct {
	HideTag bool
	Tag     openapi3.Tag
}

// Group allows grouping routes under a common path.
// Middlewares are scoped to the group.
// For example:
//
//	s := fuego.NewServer()
//	viewsRoutes := fuego.Group(s, "")
//	apiRoutes := fuego.Group(s, "/api")
//	// Registering a middlewares scoped to /api only
//	fuego.Use(apiRoutes, myMiddleware)
//	// Registering a route under /api/users
//	fuego.Get(apiRoutes, "/users", func(c fuego.ContextNoBody) (ans, error) {
//		return ans{Ans: "users"}, nil
//	})
//	s.Run()
func (group *RouterGroup) Group(path string, groupOption ...GroupOption) *RouterGroup {
	newGroup := group.newRouteGroup(path)

	if newGroup.groupTag != "" && !group.server.disableAutoGroupTags {
		groupOption = append(groupOption, GroupOption{Tag: openapi3.Tag{Name: newGroup.groupTag}})

		if !groupOption[0].HideTag {
			group.server.OpenApiSpec.Tags = append(group.server.OpenApiSpec.Tags, &groupOption[0].Tag)
		}
	}

	return newGroup
}

func Group(group *RouterGroup, path string, groupOption ...GroupOption) *RouterGroup {
	return group.Group(path, groupOption...)
}

type Route[ResponseBody any, RequestBody any] struct {
	Operation *openapi3.Operation // GENERATED OpenAPI operation, do not set manually in Register function. You can change it after the route is registered.
	All       bool                // define route to all HTTP methods. If true, ignore Method
	Method    string              // HTTP method (GET, POST, PUT, PATCH, DELETE)
	Path      string              // URL path. Will be prefixed by the base path of the server and the group path if any
	Handler   http.Handler        // handler executed for this route

	isNonStd   bool
	mainRouter *Server // ref to the main router, used to register the route in the OpenAPI spec
}

// Capture all methods (GET, POST, PUT, PATCH, DELETE) and register a controller.
func All[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...func(http.Handler) http.Handler) Route[T, B] {
	return Register(s, Route[T, B]{
		Path:     path,
		All:      true,
		isNonStd: true,
	}, HTTPHandler(s.server, controller), middlewares...)
}

func Get[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...func(http.Handler) http.Handler) Route[T, B] {
	return Register(s, Route[T, B]{
		Method:   http.MethodGet,
		Path:     path,
		isNonStd: true,
	}, HTTPHandler(s.server, controller), middlewares...)
}

func Post[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...func(http.Handler) http.Handler) Route[T, B] {
	return Register(s, Route[T, B]{
		Method:   http.MethodPost,
		Path:     path,
		isNonStd: true,
	}, HTTPHandler(s.server, controller), middlewares...)
}

func Delete[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...func(http.Handler) http.Handler) Route[T, B] {
	return Register(s, Route[T, B]{
		Method:   http.MethodDelete,
		Path:     path,
		isNonStd: true,
	}, HTTPHandler(s.server, controller), middlewares...)
}

func Put[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...func(http.Handler) http.Handler) Route[T, B] {
	return Register(s, Route[T, B]{
		Method:   http.MethodPut,
		Path:     path,
		isNonStd: true,
	}, HTTPHandler(s.server, controller), middlewares...)
}

func Patch[T, B any, Contexted ctx[B]](s *RouterGroup, path string, controller func(Contexted) (T, error), middlewares ...func(http.Handler) http.Handler) Route[T, B] {
	return Register(s, Route[T, B]{
		Method:   http.MethodPatch,
		Path:     path,
		isNonStd: true,
	}, HTTPHandler(s.server, controller), middlewares...)
}

// Register registers a controller into the default mux and documents it in the OpenAPI spec.
func Register[T, B any](group *RouterGroup, route Route[T, B], controller http.Handler, middlewares ...func(http.Handler) http.Handler) Route[T, B] {
	route.Handler = controller

	allMiddlewares := append(group.middlewares, middlewares...)

	if route.All || route.Method == "" {
		group.rg.Any(route.Path, setPattern, gin.WrapH(withMiddlewares(route.Handler, allMiddlewares...)))
	} else {
		group.rg.Handle(route.Method, route.Path, setPattern, gin.WrapH(withMiddlewares(route.Handler, allMiddlewares...)))
	}

	if group.DisableOpenapi || route.Method == "" || route.All {
		return route
	}

	route.Path = group.rg.BasePath() + route.Path
	if route.isNonStd {
		route.Path = ginToStdPath(route.Path)
	}

	var err error
	route.Operation, err = RegisterOpenAPIOperation(group, route)
	if err != nil {
		slog.Warn("error documenting openapi operation", "error", err)
	}

	if route.Operation.OperationID == "" {
		route.Operation.OperationID = route.Method + "_" + route.Path
	}

	route.mainRouter = group.server
	return route
}

func UseStd(s *RouterGroup, middlewares ...func(http.Handler) http.Handler) {
	Use(s, middlewares...)
}

func Use(s *RouterGroup, middlewares ...func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middlewares...)
}

func (group *RouterGroup) Use(middlewares ...func(http.Handler) http.Handler) {
	group.middlewares = append(group.middlewares, middlewares...)
}

// Handle registers a standard HTTP handler into the default mux.
// Use this function if you want to use a standard HTTP handler instead of a Fuego controller.
func Handle(s *RouterGroup, path string, controller http.Handler, middlewares ...func(http.Handler) http.Handler) Route[any, any] {
	return Register(s, Route[any, any]{
		All:  true,
		Path: path,
	}, controller, middlewares...)
}

func AllStd(s *RouterGroup, path string, controller func(http.ResponseWriter, *http.Request), middlewares ...func(http.Handler) http.Handler) Route[any, any] {
	return Register(s, Route[any, any]{
		All:  true,
		Path: path,
	}, http.HandlerFunc(controller), middlewares...)
}

func GetStd(s *RouterGroup, path string, controller func(http.ResponseWriter, *http.Request), middlewares ...func(http.Handler) http.Handler) Route[any, any] {
	return Register(s, Route[any, any]{
		Method: http.MethodGet,
		Path:   path,
	}, http.HandlerFunc(controller), middlewares...)
}

func PostStd(s *RouterGroup, path string, controller func(http.ResponseWriter, *http.Request), middlewares ...func(http.Handler) http.Handler) Route[any, any] {
	return Register(s, Route[any, any]{
		Method: http.MethodPost,
		Path:   path,
	}, http.HandlerFunc(controller), middlewares...)
}

func DeleteStd(s *RouterGroup, path string, controller func(http.ResponseWriter, *http.Request), middlewares ...func(http.Handler) http.Handler) Route[any, any] {
	return Register(s, Route[any, any]{
		Method: http.MethodDelete,
		Path:   path,
	}, http.HandlerFunc(controller), middlewares...)
}

func PutStd(s *RouterGroup, path string, controller func(http.ResponseWriter, *http.Request), middlewares ...func(http.Handler) http.Handler) Route[any, any] {
	return Register(s, Route[any, any]{
		Method: http.MethodPut,
		Path:   path,
	}, http.HandlerFunc(controller), middlewares...)
}

func PatchStd(s *RouterGroup, path string, controller func(http.ResponseWriter, *http.Request), middlewares ...func(http.Handler) http.Handler) Route[any, any] {
	return Register(s, Route[any, any]{
		Method: http.MethodPatch,
		Path:   path,
	}, http.HandlerFunc(controller), middlewares...)
}

func withMiddlewares(controller http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		controller = middlewares[i](controller)
	}
	return controller
}

func ginToStdPath(path string) string {
	builder := strings.Builder{}
	builder.Grow(len(path))

	for len(path) > 0 {
		colIdx := strings.IndexRune(path, ':')
		if colIdx < 0 {
			builder.WriteString(path)
			break
		}

		builder.WriteString(path[:colIdx])
		path = path[colIdx+1:]
		end := strings.IndexRune(path, '/')
		if end < 0 {
			end = len(path)
		}

		builder.WriteRune('{')
		builder.WriteString(path[:end])
		builder.WriteRune('}')

		path = path[end:]
	}

	return builder.String()
}
